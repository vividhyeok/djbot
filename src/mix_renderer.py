from pydub import AudioSegment
from pydub.playback import play
import os
from src.utils import logger

# Fix ffmpeg path for pydub if needed
import imageio_ffmpeg
AudioSegment.converter = imageio_ffmpeg.get_ffmpeg_exe() 
# We don't rely on pydub loading anymore, but we might need converter for export.

import librosa
import numpy as np
import soundfile as sf
import io
from scipy import signal

class MixRenderer:
    def __init__(self):
        pass

    def _process_numpy_audio(self, y, sr, speed_change=1.0, filter_type=None, pitch_steps=0.0):
        """
        Apply DSP effects on numpy array:
        1. Time Stretch
        2. Pitch Shift (Harmonic Mixing)
        3. EQ Filter
        4. Safety (Highpass + Limiter)
        """
        # 1. Time Stretch (Keep Pitch)
        if abs(speed_change - 1.0) > 0.01:
            try:
                if y.ndim > 1:
                    new_channels = []
                    for i in range(y.shape[0]):
                         new_channels.append(librosa.effects.time_stretch(y[i], rate=speed_change))
                    y = np.array(new_channels)
                else:
                    y = librosa.effects.time_stretch(y, rate=speed_change)
            except Exception as e:
                logger.warning(f"Time stretch failed: {e}")

        # 2. Pitch Shift (New)
        if abs(pitch_steps) > 0.1:
            try:
                # Limit to +/- 2 semitones to prevent chipmunk effect
                steps = max(-2.0, min(2.0, pitch_steps))
                if y.ndim > 1:
                    new_channels = []
                    for i in range(y.shape[0]):
                         new_channels.append(librosa.effects.pitch_shift(y[i], sr=sr, n_steps=steps))
                    y = np.array(new_channels)
                else:
                    y = librosa.effects.pitch_shift(y, sr=sr, n_steps=steps)
            except Exception as e:
                logger.warning(f"Pitch shift failed: {e}")

        # 3. EQ Filter (Butterworth)
        if filter_type:
            try:
                if filter_type == 'lowpass':
                    cutoff = 300 
                    sos = signal.butter(4, cutoff, 'lp', fs=sr, output='sos')
                elif filter_type == 'highpass':
                    cutoff = 300 
                    sos = signal.butter(4, cutoff, 'hp', fs=sr, output='sos')
                # Bass Swap Helper: Steeper Highpass for cleaner swap
                elif filter_type == 'bass_swap': 
                    cutoff = 250
                    sos = signal.butter(8, cutoff, 'hp', fs=sr, output='sos')
                # Filter Fade: Simply apply Lowpass to simulate being "filtered out" 
                # (For preview, we can't easily automate cut-off sweep in this architecture without frame-by-frame)
                # So we simulate the "result": A is low-passed, B is clear.
                elif filter_type == 'filter_fade':
                    cutoff = 400
                    sos = signal.butter(4, cutoff, 'lp', fs=sr, output='sos')
                else:
                    sos = None
                
                if sos is not None:
                    y = signal.sosfilt(sos, y, axis=-1)
            except Exception as e:
                logger.warning(f"Filter failed: {e}")

        # 4. Safety: Highpass 30Hz (Remove Sub-sonic rumble)
        try:
             sos_sub = signal.butter(4, 30, 'hp', fs=sr, output='sos')
             y = signal.sosfilt(sos_sub, y, axis=-1)
        except: pass

        # 5. Safety: Limiter / Clipper
        # Hard clip at -1.0 dB to prevent distortion in speakers
        threshold = 0.9 # approx -1dB
        if np.max(np.abs(y)) > threshold:
            # Soft-clip or Hard-clip? Hard clip for speed, but Soft is better.
            # Simple tanh compression
            y = np.tanh(y) 
            # Normalize peak if still too loud? 
            # tanh keeps it < 1.0 naturally.
            
        return y

    def _load_as_audiosegment(self, filepath, target_db=-14.0, speed_change=1.0, filter_type=None, pitch_steps=0.0, start_offset=0, duration=None, trim=True):
        """
        Loads audio, applies DSP (Stretch/Pitch/EQ), Normalizes, returns AudioSegment.
        trim=True: Trims leading/trailing silence (good for standalone/previews).
        trim=False: Keeps exact timing (essential for stitching segments).
        """
        try:
            # 1. Load with Librosa (Fast)
            y, sr = librosa.load(filepath, sr=44100, mono=False)
            
            # --- Silence Trimming (New: Remove dead air for tight mixing) ---
            # Trim leading/trailing silence below 30dB (or custom thresh)
            # Only trim if we are loading the FULL file context (not relevant for crop-loads, but here we load full)
            # Actually user wants to trim "Intro empty parts" or "Outro empty parts".
            # Safe approach: Trim start and end of the loaded buffer.
            
            # If we trimmed, duration changed.
            # But our start_time/end_time in args refer to the ORIGINAL file time.
            # This is complex. If we trim, all timestamps shift.
            # STRATEGY:
            # Instead of trimming the *source* blindly (which breaks timestamps),
            # check if start_time maps to silence? 
            # Easier MVP: Just check the *segment* we are about to play.
            # If this function returns a segment, trim that segment? 
            # "Intro" usually starts at 0. If 0-5s is silence, we should skip it.
            # "Outro" usually ends at duration. If last 5s is silence, stop early.
            
            # Current implementation of `_load_as_audiosegment` takes start_offset/duration.
            # Let's crop first, THEN trim the result?
            # Yes. If I ask for the Intro (0-10s) and it's silence, I want the result to be trimmed?
            # NO. The user wants the mix to START where sound starts.
            # `transition_engine` picked `b_in_time` (e.g. 0.0s).
            # We need to DETECT if 0.0s is silence, and if so, shift `b_in_time` forward.
            
            # Correct fix is in `TransitionEngine` or `Analyzer` to identify "Actual Audio Start".
            # But I can do a hack here:
            # If I am loading an "Intro" (start_offset approx 0), trim leading silence.
            # If I am loading an "Outro", trim trailing silence.
            
            # Actually, let's keep it simple: Trim the loaded *raw* audio before processing? 
            # No, that breaks sync.
            
            # Let's use `librosa.effects.trim` on the loaded audio `y` IF it covers the whole file 
            # AND update our "play cursor". 
            # But this function is just a loader.
            
            # BETTER FIX: Let's simply trim the OUTPUT of this function?
            # If I return a snippet, trim silence from it?
            # Yes. If the snippet is "Intro", trimming leading silence makes it start punchy.
            
            # However, `y` is the full file here.
            # Let's just crop the requested region first.
            
            y_segment = y
            if start_offset is not None or duration is not None:
                s = int((start_offset or 0) * sr)
                e = int(((start_offset or 0) + (duration or (len(y)/sr - (start_offset or 0)))) * sr)
                # Ensure bounds are within the loaded audio
                s = max(0, s)
                e = min(len(y) if y.ndim == 1 else y.shape[-1], e)
                y_segment = y[:, s:e] if y.ndim > 1 else y[s:e]
            
            # Now Trim THIS segment?
            # User Feedback: "If >1s silence at Outro, cut it. 5s is too long."
            # We want aggressive trimming of leading/trailing silence to avoid gaps.
            
            # Use librosa.trim
            # top_db=40 (default is 60, but 40 is safer for "perceived silence" in loud hip hop)
            # frame_length/hop_length default is fine.
            # We just want to trim start/end.
            
            # However, `y` is the full requested segment.
            # If it contains "Song -> Silence", we want "Song".
            
            if trim:
                y_trimmed, index = librosa.effects.trim(y_segment, top_db=40)
                y_segment = y_trimmed

            # 2. Process (DSP)
            y_processed = self._process_numpy_audio(y_segment, sr, speed_change, filter_type, pitch_steps)
            
            # 3. Convert to Segment
            if y_processed.ndim == 1:
                y_processed = np.vstack([y_processed, y_processed])
                
            y_int16 = (y_processed * 32767).astype(np.int16)
            
            seg = AudioSegment(
                y_int16.T.tobytes(), 
                frame_rate=sr,
                sample_width=2, 
                channels=2
            )
            
            # Volume Normalization (Robust Peak)
            # User complained about volume diffs.
            # RMS normalization is better but pydub.effects.normalize does Peak.
            # Peak is safer for distortion. Let's maximize peak to -1.0dB.
            # This ensures "Quiet" tracks get boosted, "Loud" tracks get tamed (if they clipped).
            
            from pydub.effects import normalize
            # Normalize to 0dB then drop to target
            seg = normalize(seg, headroom=1.0) # Peaks at -1.0dB
            
            return seg
            
        except Exception as e:
            logger.error(f"Failed to load {filepath}: {e}")
            # Fallback (can't stretch easily with pydub fallback, ignore params)
            seg = AudioSegment.from_file(filepath)
            if target_db:
                return seg.apply_gain(target_db - seg.dBFS)
            return seg

    def render_preview(self, track_a_path, track_b_path, transition_spec) -> str:
        """
        Renders a short mp3 preview (e.g. 30s) of the transition.
        """
        margin = 10
        overlap = transition_spec.get('duration', 10)
        t_out = transition_spec['a_out_time']
        t_in = transition_spec['b_in_time']
        
        speed_a = transition_spec.get('speed_a', 1.0)
        speed_b = transition_spec.get('speed_b', 1.0)
        filter_type = transition_spec.get('filter_type', None)
        pitch_a = transition_spec.get('pitch_step_a', 0.0)
        pitch_b = transition_spec.get('pitch_step_b', 0.0)
        
        logger.info(f"Rendering preview: {transition_spec['name']} (Spd: {speed_a:.2f}) (Pitch: {pitch_b:.1f})")
        
        # Load A
        t_start_a = max(0, t_out - margin)
        t_end_a = t_out + overlap
        dur_a_orig = t_end_a - t_start_a
        
        sound_a = self._load_as_audiosegment(track_a_path, start_offset=t_start_a, duration=dur_a_orig, speed_change=speed_a, pitch_steps=pitch_a)
        
        # Slice B:
        t_start_b = t_in
        t_end_b = t_in + margin + overlap
        dur_b_orig = t_end_b - t_start_b
        
        sound_b = self._load_as_audiosegment(track_b_path, start_offset=t_start_b, duration=dur_b_orig, speed_change=speed_b, pitch_steps=pitch_b)
        
        # Now sound_a and sound_b are stretched.
        # Their durations have changed.
        # dur_new = dur_orig / speed.
        
        # We need to align them.
        # Overlap region:
        # In original time, overlap was `overlap`.
        # In new time, overlap is `overlap / speed`.
        # Assume speeds are similar (master tempo). Use average speed for alignment calc?
        # Or just align the END of A with the logic.
        
        # Logic:
        # We want the "Mix Point" to align.
        # Mix Point in A was `t_out`. In slice, that is `margin` seconds from start.
        # In stretched slice, it is `margin / speed_a`.
        
        mix_point_a = margin / speed_a
        
        # B starts at t_in. In slice, that is 0.
        # We overlay B such that it starts at ...?
        # Crossfade: B starts `overlap` BEFORE A ends?
        # A ends at `end of clip`.
        # Wait, earlier we established:
        # Overlap means they play together for X seconds.
        # So B starts at (A_Mix_Point - Overlap/2)? No.
        # Standard: A plays. At (A_End - Overlap), B starts.
        
        # Let's deduce where B starts relative to A's clip start.
        # A's Clip: [Pre-Mix (margin)] [Mix (overlap)]
        # B's Clip: [Mix (overlap)] [Post-Mix (margin)]
        
        # We want to align the [Mix] regions.
        # So B starts at `margin` (scaled) into A?
        # Yes.
        
        # If speed_a != speed_b, the overlap regions might drift.
        # That's what beatmatching fixes (speeds should match).
        
        # 3. Apply Filters for Bass Swap / Mashup (Transition Zone Only)
        
        # Calculate Mix Position
        # mix_point_a in audio time (not stretched) is `margin`.
        # in stretched time, it is margin / speed_a
        mix_point_a = margin / speed_a
        pos_b = mix_point_a * 1000 # ms
        
        # User Request: "Only filter during the transition part".
        # We need to slice the audio into:
        # [A_Pre_Mix] + [Mix_Zone (Filtered)] + [B_Post_Mix]
        
        mix_start_ms = int(pos_b)
        mix_end_ms = mix_start_ms + int(overlap * 1000 / speed_a)
        
        # Slicing A
        a_pre = sound_a[:mix_start_ms]
        a_mix = sound_a[mix_start_ms:mix_end_ms]
        # a_post? Usually A ends after mix.
        
        # Slicing B
        # B starts at 0 relative to mix_point (technically).
        # But wait, sound_b is the WHOLE chunk loaded? 
        # No, sound_b is the chunk starting at b_in.
        # So sound_b's [0 : overlap] is the mix part.
        b_mix = sound_b[:(mix_end_ms - mix_start_ms)]
        b_post = sound_b[(mix_end_ms - mix_start_ms):]
        
        # Apply Filters to Mix Zone ONLY
        if filter_type == 'bass_swap':
            a_mix = a_mix.high_pass_filter(300)
            # b_mix usually full bass
            
        elif filter_type == 'mashup_split':
            # A (Beat) = Full (or slight dip)
            # B (Vocals) = High Pass
            b_mix = b_mix.high_pass_filter(300)
            
            # Volume tweak for mashup blend
            a_mix = a_mix - 1.0 # dip beat slightly
            b_mix = b_mix + 1.0 # boost vocals
            
        # 4. Mix the Zone
        # Handle Crossfades/Overlays
        fade_ms = int((overlap / speed_a) * 1000)
        
        if transition_spec['type'] in ['crossfade', 'bass_swap']:
            # Standard Crossfade
            if len(a_mix) > fade_ms: a_mix = a_mix.fade_out(fade_ms)
            if len(b_mix) > fade_ms: b_mix = b_mix.fade_in(fade_ms)
            
            # Overlay
            mixed_zone = a_mix.overlay(b_mix)
            
        elif transition_spec['type'] == 'mashup':
            # Mashup = Full Overlay (No Fade usually, or fast fade)
            # We want them to run parallel.
            mixed_zone = a_mix.overlay(b_mix)
            
        else:
            # Cut (No overlap mixing, just switch)
             mixed_zone = a_mix + b_mix # Append? No, cut means switch.
             # If cut, logic is different.
             return self._render_cut(sound_a, sound_b, pos_b)
             
        # 5. Stitch
        # Result = A_Pre + Mixed_Zone + B_Post
        # Note: 'a_mix' and 'b_mix' lengths might differ slightly due to stretching rounding?
        # overlay matches length of first arg usually.
        
        final_mix = a_pre + mixed_zone + b_post
        
        # Use a more unique filename to prevent namespace collisions
        import uuid
        unique_id = uuid.uuid4().hex[:8]
        output_path = f"cache/preview_{transition_spec['type']}_{int(t_out)}_{unique_id}.mp3"
        final_mix.export(output_path, format="mp3")
        return output_path

    def _render_cut(self, sound_a, sound_b, cut_point_ms):
        """ Splicing with unique filename """
        p1 = sound_a[:int(cut_point_ms)]
        p2 = sound_b
        mixed = p1 + p2
        
        import uuid
        unique_id = uuid.uuid4().hex[:8]
        output_path = f"cache/preview_cut_{unique_id}.mp3"
        mixed.export(output_path, format="mp3")
        return output_path

    def render_final_mix(self, playlist: list, transitions: list, output_path: str):
        """
        Renders a full mixset with 'Speed Restoration':
        - Tracks play at original speed during the body.
        - Speed changes only during the transition overlap.
        - BPM burden shared between A and B (Balanced Sync).
        """
        if not playlist:
            return
            
        logger.info("Starting high-precision segment-based mix render...")
        
        current_mix = AudioSegment.silent(duration=0)
        track_starts = [] # (time_ms, label)
        
        # We track where we are in each track's original audio
        # track_cursors[i] = current offset in seconds for playlist[i]
        track_cursors = [0.0] * len(playlist)
        
        for i in range(len(playlist)):
            # 1. Identify context
            is_first = (i == 0)
            is_last = (i == len(playlist) - 1)
            
            t_current = playlist[i]
            # Transition with PREVIOUS track (if any)
            trans_prev = transitions[i-1] if i > 0 else None
            # Transition with NEXT track (if any)
            trans_next = transitions[i] if i < len(transitions) else None
            
            # --- PHASE A: Mix-In from previous track ---
            # This was already handled in the PREVIOUS iteration's "Transition Overlap" step.
            # Except for the initial silence trimming of the first track.
            if is_first:
                # Determine how much of the first track to play before its first transition
                # Start at 0, but detect silence if it's the very start
                first_body_dur = (trans_next['a_out_time'] - trans_next['duration'])
                # Load first track's intro
                sound_intro = self._load_as_audiosegment(t_current['filepath'], start_offset=0, duration=first_body_dur, trim=True)
                
                track_starts.append((0, t_current['filename']))
                current_mix += sound_intro
                track_cursors[i] = first_body_dur
            
            # --- PHASE B: Transition Overlap (Current Track A + Next Track B) ---
            if trans_next:
                T_A = t_current
                T_B = playlist[i+1]
                
                # Parameters
                overlap = trans_next['duration']
                t_out_a = trans_next['a_out_time']
                t_in_b = trans_next['b_in_time']
                
                speed_a = trans_next.get('speed_a', 1.0)
                speed_b = trans_next.get('speed_b', 1.0)
                pitch_b = trans_next.get('pitch_step_b', 0.0)
                f_type = trans_next.get('filter_type', None)
                
                # Load Overlap Segment from A (Stretched)
                # Note: No trimming here to prevent desync
                seg_a = self._load_as_audiosegment(T_A['filepath'], start_offset=t_out_a - overlap, duration=overlap, speed_change=speed_a, trim=False)
                
                # Load Overlap Segment from B (Stretched + Pitch Shift)
                seg_b = self._load_as_audiosegment(T_B['filepath'], start_offset=t_in_b, duration=overlap, speed_change=speed_b, pitch_steps=pitch_b, trim=False)
                
                # LRC Point: Track B officially "starts" at the beginning of the overlap
                track_starts.append((len(current_mix), T_B['filename']))
                
                # Apply Transition FX (similar to render_preview)
                # Slicing for filtering (if needed)
                dur_ms = len(seg_a)
                fade_ms = int(dur_ms / 2) # Crossfade over half the overlap is usually safe
                
                a_mix = seg_a
                b_mix = seg_b
                
                if f_type == 'bass_swap':
                    # A (Out) loses bass, B (In) keeps it
                    a_mix = a_mix.high_pass_filter(300)
                elif f_type == 'mashup_split':
                    # A (Beat) full, B (Vocals) hpf
                    b_mix = b_mix.high_pass_filter(300)
                
                if trans_next['type'] in ['crossfade', 'bass_swap']:
                    a_mix = a_mix.fade_out(dur_ms)
                    b_mix = b_mix.fade_in(dur_ms)
                    mixed = a_mix.overlay(b_mix)
                elif trans_next['type'] == 'mashup':
                    mixed = a_mix.overlay(b_mix)
                elif trans_next['type'] == 'cut':
                    # Hard cut at the out point
                    mixed = b_mix
                else:
                    mixed = a_mix.append(b_mix, crossfade=int(dur_ms/2))
                
                current_mix += mixed
                
                # Update cursors
                track_cursors[i] = t_out_a # A is done
                track_cursors[i+1] = t_in_b + overlap # B has played its transition part
                
                # --- PHASE C: Body of Track B (Original Speed) ---
                # Before the NEXT transition for B starts
                next_trans_for_b = transitions[i+1] if i+1 < len(transitions) else None
                
                if next_trans_for_b:
                    b_body_start = track_cursors[i+1]
                    b_body_end = next_trans_for_b['a_out_time'] - next_trans_for_b['duration']
                    b_body_dur = b_body_end - b_body_start
                    
                    if b_body_dur > 0:
                        sound_body_b = self._load_as_audiosegment(T_B['filepath'], start_offset=b_body_start, duration=b_body_dur, speed_change=1.0, trim=False)
                        current_mix += sound_body_b
                        track_cursors[i+1] = b_body_end
                else:
                    # No more transitions, play remainder of last track
                    b_remainder = T_B['duration'] - track_cursors[i+1]
                    if b_remainder > 0:
                        sound_remainder = self._load_as_audiosegment(T_B['filepath'], start_offset=track_cursors[i+1], duration=b_remainder, speed_change=1.0, trim=True)
                        current_mix += sound_remainder
                        track_cursors[i+1] = T_B['duration']

        # 3. Export Mix (High Quality - 320kbps)
        logger.info(f"Exporting final mix to {output_path} (320kbps)")
        current_mix.export(
            output_path, 
            format="mp3",
            bitrate="320k",
            parameters=["-q:a", "0"]
        )
        
        # 4. Generate LRC for MP3 player navigation
        lrc_path = output_path.rsplit('.', 1)[0] + ".lrc"
        self._write_lrc(track_starts, lrc_path)
        logger.info(f"LRC saved to {lrc_path}")
        logger.info(f"✅ Mix complete! Total duration: {len(current_mix)/1000/60:.1f} minutes")
        
        return output_path, lrc_path
        
    def _write_lrc(self, track_starts, path):
        """
        Generate LRC file with track names for MP3 player navigation.
        track_starts: list of (ms, filepath)
        """
        with open(path, 'w', encoding='utf-8') as f:
            # Add LRC metadata headers
            f.write("[ar:DJ Bot Auto Mix]\n")
            f.write("[ti:Hip-Hop Club Mix]\n")
            f.write("[al:Auto Generated]\n")
            f.write("[by:DJ Bot]\n")
            f.write("\n")
            
            # Write track entries
            for ms, filepath in track_starts:
                # Format [mm:ss.xx]
                seconds = ms / 1000.0
                m = int(seconds // 60)
                s = seconds % 60
                
                # Get track name from filename (original MP3 filename)
                name = os.path.basename(filepath)
                name = os.path.splitext(name)[0]
                
                # LRC format: [MM:SS.xx] Track Name
                line = f"[{m:02}:{s:05.2f}] {name}\n"
                f.write(line)
            
            logger.info(f"LRC file created with {len(track_starts)} tracks")

