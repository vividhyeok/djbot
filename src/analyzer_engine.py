import librosa
import numpy as np
import os
import json
from dataclasses import dataclass, asdict
from typing import List, Optional, Dict
from scipy import signal
from src.utils import logger, get_file_hash, CACHE_DIR, save_json, load_json

@dataclass
class AudioAnalysis:
    filepath: str
    duration: float
    bpm: float
    beat_frames: List[int]
    beat_times: List[float]
    downbeats: List[float] # estimated
    energy_curve: List[float] # 1 value per beat or frame
    vocal_curve: List[float] # if stems available
    highlights: List[Dict] # {start_time, end_time, score}

class AudioAnalyzer:
    def __init__(self):
        pass

    def analyze_track(self, filepath: str, stems_dir: Optional[str] = None) -> Dict:
        """
        Full analysis pipeline:
        1. Check cache.
        2. Load Audio.
        3. Extract Beats/BPM.
        4. Extract Energy.
        5. Detect Highlights.
        """
        file_hash = get_file_hash(filepath)
        cache_path = CACHE_DIR / f"{file_hash}_analysis.json"
        
        cached_data = load_json(cache_path)
        if cached_data:
            logger.info(f"Loaded analysis from cache: {filepath}")
            return cached_data

        logger.info(f"Analyzing track: {filepath}")
        
        # 1. Load Audio (Mono, 22050Hz for speed)
        y, sr = librosa.load(filepath, sr=22050, mono=True)
        duration = librosa.get_duration(y=y, sr=sr)

        # 2. Beat Tracking
        logger.info("Tracking beats...")
        tempo, beat_frames = librosa.beat.beat_track(y=y, sr=sr)
        beat_times = librosa.frames_to_time(beat_frames, sr=sr)
        
        # Scalar tempo
        if isinstance(tempo, np.ndarray):
            tempo = tempo[0]

        # 3. Energy (RMS)
        # We'll compute RMS in frames, then sync to beats
        hop_length = 512
        rms = librosa.feature.rms(y=y, frame_length=2048, hop_length=hop_length)[0]
        
        # Calculate global loudness (simple RMS dB)
        avg_rms = np.mean(rms)
        avg_loudness_db = 20 * np.log10(avg_rms + 1e-6)
        
        # Sync RMS to beats (average RMS between beat frames)
        # Simple beat-sync: sample RMS at beat times
        # Better: Average RMS within beat intervals
        beat_energy = librosa.util.sync(rms.reshape(1, -1), beat_frames, aggregate=np.mean)[0]
        
        # Normalize Energy
        if len(beat_energy) > 0:
            beat_energy = beat_energy / (np.max(beat_energy) + 1e-6)
        
        # 4. Phrase Grid (32 beats logic)
        # We assume 4/4 signature. 1 Bar = 4 beats. 8 Bars = 32 beats.
        # Ideally we find the "Downbeat" (Beat 1 of Bar 1).
        # Librosa beat_track gives approximate phase, but finding "THE 1" is hard without DL.
        # Heuristic: The beat with highest energy in a local window is often the Downbeat.
        # MVP: Just grid from the first beat.
        
        phrases = [] # List of start times (seconds) of 32-beat chunks
        beats = librosa.frames_to_time(beat_frames, sr=sr)
        
        # Grid Size: 32 beats
        grid_size = 32
        
        # Try to align grid to strongest beat in first 10 seconds?
        # Simple for now: Start at Beat 0.
        for i in range(0, len(beats), grid_size):
            if i < len(beats):
                phrases.append(beats[i])
                
        # 5. Key Detection (Chroma)
        # Use Chroma CQT for pitch class
        chroma = librosa.feature.chroma_cqt(y=y, sr=sr)
        # Sum over time to get global key profile
        chroma_sum = np.sum(chroma, axis=1)
        # Simple template matching for Major/Minor keys
        # Ref: http://rnhart.net/articles/key-finding/
        maj_profile = [6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88]
        min_profile = [6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17]
        
        def correlate(profile, template):
            return np.corrcoef(profile, template)[0, 1]
            
        key_corrs = []
        notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
        for i in range(12):
            # Rotate chroma to align with C, C#, etc
            rolled = np.roll(chroma_sum, -i)
            corr_maj = correlate(rolled, maj_profile)
            corr_min = correlate(rolled, min_profile)
            key_corrs.append( (corr_maj, f"{notes[i]} Major") )
            key_corrs.append( (corr_min, f"{notes[i]} Minor") )
            
        key_corrs.sort(reverse=True)
        detected_key = key_corrs[0][1] # Best match
        
        # 6. Structure Segments (Heuristic Labels)
        # We classify 32-beat phrases into Intro/Verse/Chorus/Bridge/Outro based on Energy & Position.
        # This is a Rough Heuristic.
        segments = []
        
        # Normalize phrase energy
        # Calculate avg energy for each phrase block
        phrase_energies = []
        if len(phrases) > 0:
            # We need beat indices to lookup energy
            # beats map to time. beat_frames map to frames.
            # beat_energy has energy per beat.
            for i in range(len(phrases)):
                # Start beat index
                b_start = i * grid_size
                b_end = min((i+1) * grid_size, len(beat_energy))
                if b_start < len(beat_energy):
                    avg = np.mean(beat_energy[b_start:b_end])
                    phrase_energies.append(avg)
                else:
                    phrase_energies.append(0)
                    
            # Classify
            # Sort energies to find thresholds
            sorted_e = sorted(phrase_energies)
            low_thresh = sorted_e[int(len(sorted_e)*0.3)]
            high_thresh = sorted_e[int(len(sorted_e)*0.7)]
            
            for i, p_time in enumerate(phrases):
                e = float(phrase_energies[i]) # Ensure float
                label = "Verse" # Default
                
                # Position logic
                rel_pos = p_time / duration
                
                if rel_pos < 0.15 and e < high_thresh:
                    label = "Intro"
                elif rel_pos > 0.85 and e < high_thresh:
                    label = "Outro"
                elif e >= high_thresh:
                    label = "Chorus"
                elif e <= low_thresh:
                    label = "Bridge" # Quiet middle part?
                else:
                    label = "Verse"
                    
                segments.append({
                    "time": float(p_time), # Ensure float
                    "label": label,
                    "energy": e
                })
                
            # 7. Vocal Analysis (Heuristic for MVP without Stems)
        # We estimate vocal activity by looking at energy in the 300Hz - 3000Hz range.
        # This is not perfect (includes snare/guitars) but better than nothing.
        
        # Bandpass filter for vocal range
        try:
            sos_vocal = signal.butter(4, [300, 3000], 'bp', fs=sr, output='sos')
            y_vocal = signal.sosfilt(sos_vocal, y)
            rms_vocal = librosa.feature.rms(y=y_vocal, frame_length=2048, hop_length=hop_length)[0]
            
            # Sync to beats
            vocal_curve = librosa.util.sync(rms_vocal.reshape(1, -1), beat_frames, aggregate=np.mean)[0]
            # Normalize
            if np.max(vocal_curve) > 0:
                vocal_curve = vocal_curve / np.max(vocal_curve)
        except Exception as e:
            logger.warning(f"Vocal heuristic failed: {e}")
            vocal_curve = np.zeros_like(beat_energy)
            
        # Add vocal info to segments
        for i, seg in enumerate(segments):
            # Find start/end beats for this segment
            # Segment 'time' is p_time. End is next p_time.
            start_time = seg['time']
            # Find corresponding beat index
            b_idx = (np.abs(beat_times - start_time)).argmin()
            # Length is grid_size (32 beats)
            e_idx = min(b_idx + grid_size, len(vocal_curve))
            
            if b_idx < len(vocal_curve):
                seg_vocal = np.mean(vocal_curve[b_idx:e_idx])
                seg['vocal_energy'] = float(seg_vocal) # Explicit float
            else:
                seg['vocal_energy'] = 0.0
        
        # 8. Highlight Detection (Heuristic)
        # Look for 16-bar (64 beat) segments with high energy
        highlights = self._detect_highlights(beat_times, beat_energy, vocal_curve, tempo)

        # Helper to clean numpy types for JSON
        def clean_types(obj):
            if isinstance(obj, (np.float32, np.float64, np.int32, np.int64)):
                return obj.item()
            elif isinstance(obj, list):
                return [clean_types(x) for x in obj]
            elif isinstance(obj, dict):
                return {k: clean_types(v) for k, v in obj.items()}
            return obj

        result = {
            "filepath": filepath,
            "hash": file_hash,
            "duration": float(duration),
            "bpm": float(tempo),
            "loudness_db": float(avg_loudness_db),
            "key": detected_key,
            "beat_times": beat_times.tolist(),
            "phrases": [float(p) for p in phrases], # Explicit float
            "segments": segments, 
            "energy": beat_energy.tolist(), # beat_energy is usually float32 array
            "highlights": highlights
        }
        
        cleaned_result = clean_types(result)
        
        save_json(cleaned_result, cache_path)
        return cleaned_result

    def _detect_highlights(self, beat_times, energy, vocals, bpm):
        """
        Find best 16-bar (approx 64 beats) segment.
        Score = avg_energy * 0.7 + avg_vocal * 0.3
        """
        if len(beat_times) < 64:
            return [{"start": 0, "end": beat_times[-1] if len(beat_times) > 0 else 0, "score": 0}]

        candidates = []
        window_size = 64 # ~16 bars in 4/4
        
        # Sliding window
        for i in range(0, len(energy) - window_size, 16): # Step by 4 bars (16 beats)
            segment_energy = energy[i : i+window_size]
            # segment_vocal = vocals[i : i+window_size]
            
            avg_e = np.mean(segment_energy)
            # avg_v = np.mean(segment_vocal)
            
            score = avg_e # + avg_v * 0.3 for now
            
            candidates.append({
                "start_beat_idx": i,
                "end_beat_idx": i+window_size,
                "start_time": beat_times[i],
                "end_time": beat_times[i+window_size-1],
                "score": float(score)
            })
        
        # Sort by score descending
        candidates.sort(key=lambda x: x['score'], reverse=True)
        return candidates[:3] # Return top 3 candidates

if __name__ == "__main__":
    # Test stub
    pass
