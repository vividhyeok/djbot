from pydub import AudioSegment
import os
import subprocess
from pathlib import Path
from src.utils import logger

# Find ffmpeg executable
def _find_ffmpeg():
    """Find ffmpeg executable — try imageio_ffmpeg, then PATH."""
    try:
        import imageio_ffmpeg
        src = imageio_ffmpeg.get_ffmpeg_exe()
        d = os.path.dirname(src)
        exe = os.path.join(d, 'ffmpeg.exe')
        if os.path.exists(exe):
            return exe
        return src
    except:
        return 'ffmpeg'

FFMPEG_EXE = _find_ffmpeg()
AudioSegment.converter = FFMPEG_EXE

# WAV cache directory
WAV_CACHE = Path("cache/wav")
WAV_CACHE.mkdir(parents=True, exist_ok=True)


class MixRenderer:
    def __init__(self):
        pass

    def _to_wav(self, filepath: str) -> str:
        """
        Convert any audio file to WAV using ffmpeg.
        Returns path to WAV file (cached).
        """
        src = Path(filepath)
        wav_path = WAV_CACHE / f"{src.stem}.wav"
        
        if wav_path.exists() and wav_path.stat().st_size > 1000:
            return str(wav_path)
        
        cmd = [
            FFMPEG_EXE,
            '-y',                    # Overwrite
            '-i', str(src),          # Input
            '-ar', '44100',          # Sample rate
            '-ac', '2',              # Stereo
            '-sample_fmt', 's16',    # 16-bit
            str(wav_path)
        ]
        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
            if wav_path.exists() and wav_path.stat().st_size > 1000:
                logger.info(f"[wav] Converted: {src.name}")
                return str(wav_path)
            else:
                logger.error(f"[wav] Failed to convert {src.name}: {result.stderr[:200]}")
                return filepath  # Fallback to original
        except Exception as e:
            logger.error(f"[wav] Conversion error {src.name}: {e}")
            return filepath

    def _load_segment(self, filepath: str, start_sec: float, end_sec: float) -> AudioSegment:
        """
        Load a specific time range from an audio file as pydub AudioSegment.
        Uses pre-converted WAV for reliable timing.
        """
        wav_path = self._to_wav(filepath)
        
        try:
            audio = AudioSegment.from_file(wav_path)
        except Exception as e:
            logger.error(f"Failed to load {wav_path}: {e}")
            return AudioSegment.silent(duration=1000)
        
        start_ms = int(start_sec * 1000)
        end_ms = int(end_sec * 1000)
        
        # Clamp to actual file length
        end_ms = min(end_ms, len(audio))
        start_ms = min(start_ms, end_ms)
        
        segment = audio[start_ms:end_ms]
        
        # Normalize loudness to -14 LUFS (approximate with dBFS)
        target_db = -14.0
        if segment.dBFS != float('-inf') and len(segment) > 0:
            change = target_db - segment.dBFS
            # Limit adjustment to prevent clipping
            change = max(-10, min(10, change))
            segment = segment.apply_gain(change)
        
        return segment

    def _bars_to_ms(self, bpm: float, n_bars: int = 8) -> int:
        """Convert N bars to milliseconds at given BPM (4/4 time)."""
        beats_per_bar = 4
        beat_duration_ms = (60.0 / bpm) * 1000
        return int(n_bars * beats_per_bar * beat_duration_ms)

    def _snap_to_downbeat(self, time_sec: float, downbeats: list, direction: str = 'nearest') -> float:
        """Snap a time to the nearest downbeat."""
        if not downbeats:
            return time_sec
        
        if direction == 'before':
            candidates = [d for d in downbeats if d <= time_sec]
            return candidates[-1] if candidates else downbeats[0]
        elif direction == 'after':
            candidates = [d for d in downbeats if d >= time_sec]
            return candidates[0] if candidates else downbeats[-1]
        else:  # nearest
            closest = min(downbeats, key=lambda d: abs(d - time_sec))
            return closest

    def render_preview(self, filepath_a, filepath_b, transition_spec):
        """
        Render a short preview of a transition between two tracks.
        Simple crossfade preview.
        """
        t_out = transition_spec.get('a_out_time', 60)
        t_in = transition_spec.get('b_in_time', 0)
        duration = transition_spec.get('duration', 8)
        
        margin = 10  # seconds of context before/after
        
        # Load segments
        seg_a = self._load_segment(filepath_a, max(0, t_out - margin), t_out)
        seg_b = self._load_segment(filepath_b, t_in, t_in + margin)
        
        # Crossfade duration (clamped to segment lengths)
        xfade_ms = int(duration * 1000)
        xfade_ms = min(xfade_ms, len(seg_a) - 500, len(seg_b) - 500)
        xfade_ms = max(xfade_ms, 1000)  # At least 1 second
        
        # Simple crossfade
        preview = seg_a.append(seg_b, crossfade=xfade_ms)
        
        import uuid
        uid = uuid.uuid4().hex[:8]
        output_path = f"cache/preview_{uid}.mp3"
        preview.export(output_path, format="mp3", bitrate="192k")
        return output_path

    def render_final_mix(self, playlist: list, transitions: list, output_path: str):
        """
        Render the full mix using simple, reliable pydub crossfade.
        
        Approach:
        1. Pre-convert all tracks to WAV (precise timing)
        2. Extract each track's play window (highlight section)
        3. Crossfade consecutive tracks over 8 bars
        4. No speed/pitch manipulation — clean transitions
        """
        if not playlist or len(playlist) < 2:
            return
        
        logger.info(f"Starting automix render: {len(playlist)} tracks")
        
        # Step 1: Pre-convert all to WAV
        logger.info("Pre-converting tracks to WAV...")
        for t in playlist:
            self._to_wav(t['filepath'])
        
        # Step 2: Build the mix by crossfading track segments
        track_starts = []  # (time_ms, filename) for LRC
        
        # Load first track's play segment
        first = playlist[0]
        play_start = first.get('play_start', 0.0)
        play_end = first.get('play_end', float(first['duration']))
        
        current_mix = self._load_segment(first['filepath'], play_start, play_end)
        track_starts.append((0, first['filename']))
        logger.info(f"[1/{len(playlist)}] {first['filename'][:40]} ({play_end - play_start:.0f}s)")
        
        # Append each subsequent track with crossfade
        for i in range(len(transitions)):
            if i + 1 >= len(playlist):
                break
            
            next_track = playlist[i + 1]
            trans = transitions[i]
            
            n_play_start = next_track.get('play_start', 0.0)
            n_play_end = next_track.get('play_end', float(next_track['duration']))
            
            # Calculate crossfade duration
            # Use transition spec duration, or default to 8 bars
            bpm = float(next_track['bpm'])
            default_xfade_ms = self._bars_to_ms(bpm, n_bars=8)
            xfade_ms = int(trans.get('duration', 8) * 1000)
            
            # Sanity check: xfade can't be longer than either segment
            next_seg = self._load_segment(next_track['filepath'], n_play_start, n_play_end)
            
            xfade_ms = min(xfade_ms, len(current_mix) - 500)
            xfade_ms = min(xfade_ms, len(next_seg) - 500)
            xfade_ms = max(xfade_ms, 2000)  # At least 2 seconds
            
            # Record where next track starts in the mix
            # It starts at (current_mix length - crossfade)
            track_start_ms = len(current_mix) - xfade_ms
            track_starts.append((track_start_ms, next_track['filename']))
            
            # Apply crossfade
            current_mix = current_mix.append(next_seg, crossfade=xfade_ms)
            
            dur_sec = (n_play_end - n_play_start)
            logger.info(f"[{i+2}/{len(playlist)}] {next_track['filename'][:40]} ({dur_sec:.0f}s, xfade={xfade_ms/1000:.1f}s)")
        
        # Step 3: Final fade out (last 5 seconds)
        if len(current_mix) > 5000:
            current_mix = current_mix.fade_out(5000)
        
        # Step 4: Export
        total_min = len(current_mix) / 1000 / 60
        logger.info(f"Exporting final mix: {total_min:.1f} minutes to {output_path}")
        
        current_mix.export(
            output_path,
            format="mp3",
            bitrate="320k",
            parameters=["-q:a", "0"]
        )
        
        # Step 5: Generate LRC
        lrc_path = output_path.rsplit('.', 1)[0] + ".lrc"
        self._write_lrc(track_starts, lrc_path)
        logger.info(f"✅ Mix complete! {total_min:.1f} minutes, {len(playlist)} tracks")
        
        return output_path, lrc_path

    def _write_lrc(self, track_starts, path):
        """Generate LRC file for MP3 player navigation."""
        with open(path, 'w', encoding='utf-8') as f:
            f.write("[ar:DJ Bot Auto Mix]\n")
            f.write("[ti:Hip-Hop Club Mix]\n")
            f.write("[al:Auto Generated]\n")
            f.write("[by:DJ Bot]\n\n")
            
            for ms, filepath in track_starts:
                seconds = ms / 1000.0
                m = int(seconds // 60)
                s = seconds % 60
                name = os.path.splitext(os.path.basename(filepath))[0]
                f.write(f"[{m:02}:{s:05.2f}] {name}\n")
            
            logger.info(f"LRC: {len(track_starts)} tracks")
