import random
from typing import List, Dict, Optional
from src.analyzer_engine import AudioAnalysis
from src.utils import logger
import numpy as np

class TransitionEngine:
    def __init__(self):
        pass

    def generate_candidates(self, track_a: Dict, track_b: Dict, stems_a: Optional[Dict], stems_b: Optional[Dict]) -> List[Dict]:
        """
        Generates 3 structural transition candidates for Track A -> Track B.
        Includes Beat Matching and EQ Logic.
        """
        candidates = []
        
        # Data Extraction
        dur_a = track_a['duration']
        dur_b = track_b['duration']
        bpm_a = track_a['bpm']
        bpm_b = track_b['bpm']
        
        # Beat Sync Calculation
        # We sync to the average BPM of the transition, or sync B to A.
        # Let's sync to the average to minimize distortion on both.
        # Unless diff is too large.
        bpm_diff = abs(bpm_a - bpm_b)
        can_sync = bpm_diff < 20 # Allow 20 BPM stretch (maybe too much for audio quality, but ok for mixing fun)
        
        if can_sync:
            target_bpm = (bpm_a + bpm_b) / 2
            speed_a = target_bpm / bpm_a
            speed_b = target_bpm / bpm_b
            sync_text = f" (Synced to {int(target_bpm)} BPM)"
        else:
            speed_a = 1.0
            speed_b = 1.0
            sync_text = ""
        
        # Highlights
        hl_a = track_a.get('highlights', [])
        hl_b = track_b.get('highlights', [])
        best_hl_a = hl_a[0] if hl_a else {'end_time': dur_a - 15, 'start_time': dur_a - 30}
        best_hl_b = hl_b[0] if hl_b else {'start_time': 15}
        
        # 1. Safe Mix (Outro -> Intro) with Beat Match
        candidates.append({
            "type": "crossfade",
            "name": "Safe Mix" + sync_text,
            "description": "Outro into Intro. Smooth & Beat-Matched.",
            "duration": 16, 
            "a_out_time": max(0, dur_a - 20),
            "b_in_time": 0,
            "speed_a": speed_a,
            "speed_b": speed_b,
            "filter_type": None
        })

        # 2. Energy/Bass Swap (Hook -> Hook)
        # For this, we simulate a Bass Swap:
        # We can't do dynamic automation easily in this structure without rendering segments.
        # Hack: We use a 'bass_swap' type which the renderer handles by applying filters
        # dynamically? No, too complex for renderer update right now.
        # Let's simple apply HighPass to outgoing A during the mix?
        # A (Highpass) + B (Incoming).
        
        candidates.append({
            "type": "bass_swap",
            "name": "Bass Swap / Energy Mix" + sync_text,
            "description": "High-energy mix. Filters bass from A while bringing in B.",
            "duration": 16,
            "a_out_time": best_hl_a['end_time'],
            "b_in_time": max(0, best_hl_b['start_time'] - 16),
            "speed_a": speed_a,
            "speed_b": speed_b,
            "filter_type": "bass_swap" # Special tag for renderer
        })

        # 3. Quick Cut / Drop
        # No stretch needed if we drop on the 1? 
        # Actually sync is still nice.
        candidates.append({
            "type": "cut",
            "name": "Hard Drop" + sync_text,
            "description": "Quick switch on the beat.",
            "duration": 4, 
            "a_out_time": max(0, dur_a - 15), 
            "b_in_time": 15, 
            "speed_a": speed_a,
            "speed_b": speed_b,
            "filter_type": None
        })
            
        return candidates

    def _find_out_point(self, track):
        # Default: Duration - 30s
        default = max(0, track['duration'] - 30)
        # Better: End of last highlight?
        if track.get('highlights'):
            # Use the end of the best highlight? Or start of highlight?
            # Usually we play the highlight, then mix out.
            # So let's take candidate highlight end.
            best = track['highlights'][0] # sorted by score
            if best['end_time'] < track['duration'] - 10:
                return best['end_time']
        return default

    def _find_in_point(self, track):
        # Default: Start + 15s (skip silence/intro)
        default = 15.0
        # Better: Start of best highlight?
        if track.get('highlights'):
            best = track['highlights'][0]
            # We want to mix IN before the highlight drops.
            # So maybe 16 bars (approx 30s) before highlight start?
            start = max(0, best['start_time'] - 15) 
            return start
        return default

    def generate_random_candidates(self, track_a: Dict, track_b: Dict, count=5, weights=None) -> List[Dict]:
        """
        Generates candidates using Stratified Sampling for Structure and Metric Cues for Timing.
        """
        candidates = []
        
        # Default Weights - FIXED TO 4 BARS ONLY
        if not weights:
            weights = {
                'types': {'crossfade': 0.5, 'bass_swap': 1.6, 'cut': 1.2, 'filter_fade': 1.0, 'mashup': 1.0},
                'bars': {4: 1.0}  # Only 4 bars
            }
        
        # Force 4 bars only
        weights['bars'] = {4: 1.0}
        
        segs_a = track_a.get('segments', [])
        segs_b = track_b.get('segments', [])
        beat_times_a = track_a.get('beat_times', [])
        beat_times_b = track_b.get('beat_times', [])
        
        # Fallbacks
        if not segs_a: segs_a = [{'time': track_a['duration']-30, 'label': 'Outro', 'energy': 0.5}]
        if not segs_b: segs_b = [{'time': 0, 'label': 'Intro', 'energy': 0.5}]

        # --- Helper: Grid Snapping ---
        def snap_to_grid(time_val, beat_times, grid_beats=16):
            """ 
            Strictly snaps to 4-bar boundaries (16 beats) for Hip-Hop.
            This ensures we don't break the natural 1 2 3 4 rhythm.
            """
            if not beat_times: return time_val
            closest_idx = (np.abs(np.array(beat_times) - time_val)).argmin()
            # Snap to nearest multiple of 16 (4 bars)
            snapped_idx = int(round(closest_idx / grid_beats) * grid_beats)
            
            # Boundary Safety: Strictly within [0, len - 1]
            max_idx = len(beat_times) - 1
            if snapped_idx > max_idx:
                # Floor to the highest possible 16-bar boundary
                snapped_idx = (max_idx // grid_beats) * grid_beats
            
            # Final fallback clip
            snapped_idx = max(0, min(snapped_idx, max_idx))
                
            return beat_times[snapped_idx]

        # --- Helper: Stratified Structure Selection ---
        def get_stratified_pair():
            """
            Instead of random(segs), pick random(LabelA)->random(LabelB).
            This ensures rare structures (like Bridge->Hook) get equal chance.
            Aggressively Retries if Outro->Intro is picked (User hates it).
            """
            labels_a = list(set(s['label'] for s in segs_a if s['label'] in ['Chorus', 'Verse', 'Bridge', 'Outro']))
            labels_b = list(set(s['label'] for s in segs_b if s['label'] in ['Intro', 'Verse', 'Chorus', 'Bridge']))
            
            if not labels_a: labels_a = ['Outro']
            if not labels_b: labels_b = ['Intro']
            
            best_pair = None
            
            # Try 10 times to find a non-Outro-Intro pair
            for _ in range(10):
                l_a = random.choice(labels_a)
                l_b = random.choice(labels_b)
                
                # Check for "Boredom" pair
                is_boring = (l_a == 'Outro' and l_b == 'Intro')
                
                # If boring, only keep it 5% of the time (rare fallback)
                if is_boring and random.random() > 0.05:
                    continue # Try again
                    
                # Found acceptable pair
                cand_a = [s for s in segs_a if s['label'] == l_a]
                cand_b = [s for s in segs_b if s['label'] == l_b]
                return random.choice(cand_a), random.choice(cand_b)
            
            # Fallback if we failed (e.g. only Outro/Intro exist)
            l_a = random.choice(labels_a)
            l_b = random.choice(labels_b)
            cand_a = [s for s in segs_a if s['label'] == l_a]
            cand_b = [s for s in segs_b if s['label'] == l_b]
            return random.choice(cand_a), random.choice(cand_b)

        # Harmonic Analysis
        key_a = track_a.get('key', 'C Major')
        key_b = track_b.get('key', 'C Major')
        
        def get_semitone_shift(k1, k2):
            notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
            root1 = k1.split(' ')[0]; root2 = k2.split(' ')[0]
            try:
                idx1 = notes.index(root1); idx2 = notes.index(root2)
                diff = idx1 - idx2
                if diff > 6: diff -= 12
                if diff < -6: diff += 12
                return diff
            except: return 0.0
            
        harmonic_shift = get_semitone_shift(key_a, key_b)

        def pick_weighted(w_dict):
            keys = list(w_dict.keys())
            vals = list(w_dict.values())
            return random.choices(keys, weights=vals, k=1)[0]

        for _ in range(count):
            # 1. Core Params
            t_type = pick_weighted(weights['types'])
            picked_bars = pick_weighted(weights['bars'])
            
            # Ensure picked_bars is an integer
            picked_bars = int(picked_bars)
            
            # 2. Structure (Stratified)
            exit_seg, entry_seg = get_stratified_pair()
            bpm_a = track_a['bpm']
            bpm_b = track_b['bpm']
            dur_a = track_a['duration']
            dur_b = track_b['duration']
            
            # 3. AUTOMATIC OR MANUAL HIGHLIGHT SELECTION
            # BUFFERED LOGIC: Transitions happen outside the designated segment.
            
            # TRACK A (Exit)
            if track_a.get('manual_out') is not None:
                # User wants manual_out to be the END of the clean body.
                # Window starts at manual_out. a_out_time = manual_out + overlap_duration
                # We reuse the picked_bars * (60.0 / bpm_a) for actual overlap calculation
                raw_out = track_a['manual_out'] + (picked_bars * (60.0 / bpm_a))
                a_out = snap_to_grid(raw_out, beat_times_a, grid_beats=4)
                if a_out > dur_a: a_out = beat_times_a[-1] if beat_times_a else dur_a
            else:
                try:
                    exit_label = exit_seg['label']
                    seg_start = exit_seg['time']
                    seg_duration = 32 * (60 / track_a['bpm'])
                    seg_end = seg_start + seg_duration
                    beat_times_a_arr = np.array(beat_times_a)
                    seg_beats_mask = (beat_times_a_arr >= seg_start) & (beat_times_a_arr < seg_end)
                    seg_beat_indices = np.where(seg_beats_mask)[0]
                    
                    if len(seg_beat_indices) > 4:
                        energy_curve = track_a.get('energy_curve', [])
                        if len(energy_curve) > 0 and len(seg_beat_indices) > 0:
                            seg_energies = [energy_curve[i] if i < len(energy_curve) else 0.5 for i in seg_beat_indices]
                            peak_idx = np.argmax(seg_energies)
                            peak_beat_idx = seg_beat_indices[peak_idx]
                            bars_after_peak = 6
                            beats_after_peak = bars_after_peak * 4
                            transition_beat_idx = min(peak_beat_idx + beats_after_peak, seg_beat_indices[-1])
                            raw_out = beat_times_a_arr[transition_beat_idx]
                        else:
                            exit_percentage = 0.85 if exit_label in ['Chorus', 'Hook'] else 0.70
                            raw_out = seg_start + (seg_duration * exit_percentage)
                    else:
                        exit_percentage = 0.85 if exit_label in ['Chorus', 'Hook'] else 0.70
                        raw_out = seg_start + (seg_duration * exit_percentage)
                    a_out = snap_to_grid(raw_out, beat_times_a, grid_beats=16)
                except Exception as e:
                    logger.warning(f"Automatic highlight selection failed for A: {e}")
                    a_out = exit_seg['time'] + 20

            # TRACK B (Entry)
            if track_b.get('manual_in') is not None:
                # User wants manual_in to be the START of the clean body.
                # Window ends at manual_in. b_in_time = manual_in - overlap_duration
                raw_in = track_b['manual_in'] - (picked_bars * (60.0 / bpm_b))
                b_in = snap_to_grid(raw_in, beat_times_b, grid_beats=4)
                if b_in < 0: b_in = 0.0
            else:
                try:
                    raw_in = entry_seg['time']
                    b_in = snap_to_grid(raw_in, beat_times_b, grid_beats=16)
                except Exception as e:
                    logger.warning(f"Automatic highlight selection failed for B: {e}")
                    b_in = entry_seg['time']
            
            # 4. Harmonic Adjustment (Intelligent Pitch Shifting)
            # Goal: Natural sound with minimal pitch adjustment
            # Strategy: Only shift if it significantly improves harmony
            
            if abs(harmonic_shift) <= 2:
                # Small shift (0-2 semitones): Always apply for perfect harmony
                apply_pitch = True
                final_pitch = harmonic_shift
            elif abs(harmonic_shift) <= 4:
                # Medium shift (3-4 semitones): Apply 50% of the time
                # This balances naturalness vs harmony
                apply_pitch = random.choice([True, False])
                final_pitch = harmonic_shift if apply_pitch else 0.0
            else:
                # Large shift (5+ semitones): Rarely apply (20% chance)
                # Prefer to keep original key for natural sound
                apply_pitch = random.random() < 0.2
                final_pitch = harmonic_shift if apply_pitch else 0.0
            
            # Safety: Cap at ±2 semitones to prevent chipmunk effect
            final_pitch = max(-2.0, min(2.0, final_pitch))
            
            # 5. Mashup Logic (New)
            # Mashup = Beat of A + Vocals of B
            # Heavily weighted if enabled
            
            f_type = None
            if t_type == 'bass_swap': 
                f_type = 'bass_swap'
            elif t_type == 'filter_fade': 
                f_type = 'filter_fade'
            elif t_type == 'mashup':
                f_type = 'mashup_split' # Special flag for renderer
                
            # 6. Speed
            bpm_a = track_a['bpm']
            bpm_b = track_b['bpm']
            target_bpm = (bpm_a + bpm_b) / 2
            speed_a = target_bpm / bpm_a
            speed_b = target_bpm / bpm_b
            
            # --- Feature Extraction ---
            features = []
            features.append(f"Type:{t_type}")
            features.append(f"Len:{picked_bars}Bar")
            
            struct_pair = f"Struct:{exit_seg['label']}->{entry_seg['label']}"
            features.append(struct_pair)
            
            # Vocal Analysis Features
            v_in = entry_seg.get('vocal_energy', 0.5)
            # v_out doesn't matter for mashup as much, we kill A's vocals anyway via EQ?
            # Actually, if A has vocals, they might clash with B's vocals even with LowPass.
            # Ideally A should be Instrumental.
            
            if t_type == 'mashup' and v_in > 0.5:
                 features.append("Mashup:VocalLayering")
            
            # ... (rest of features) ...
            
            # Construct
            beat_dur = 60 / target_bpm
            duration = picked_bars * 4 * beat_dur
            
            c = {
                "type": t_type,
                "name": f"{t_type} | {struct_pair}",
                "description": f"Features: {', '.join(features)}",
                "duration": duration,
                "a_out_time": a_out,
                "b_in_time": b_in,
                "speed_a": speed_a,
                "speed_b": speed_b,
                "pitch_step_b": final_pitch, 
                "filter_type": f_type,
                "meta": {
                    "features": features,
                    "type": t_type,
                    "bars": picked_bars
                }
            }
            # For Mashup, we want B to START immediately at A_Out? or Overlap?
            # Mashup implies playing TOGETHER.
            # So "Duration" is the length of the mashup overlap (usually long, e.g. 16/32 bars).
            # If picked_bars is small (4), mashup is short.
            # If t_type is mashup, force overlap?
            
            if t_type == 'mashup':
                 # Mashup needs long overlap
                 c['duration'] = max(duration, 15.0) # at least 15s
                 c['meta']['bars'] = max(picked_bars, 8)
                 
            candidates.append(c)
            
        return candidates

    def _can_sync(self, a, b):
        return abs(a['bpm'] - b['bpm']) < 15 # Allow sync if BPM within 15

    def select_best_candidate(self, candidates, weights=None, min_exit_time=None, harmonic_factor=1.0):
        """
        Picks the highest scoring candidate based on weights.
        Ensures chronological safety and maximizes accuracy.
        """
        if not candidates: return None
        
        scored_candidates = []
        for c in candidates:
            # 1. Base Type Weight
            type_weight = 1.0
            if weights:
                type_weight = weights.get('types', {}).get(c['type'], 1.0)
                bars = c.get('meta', {}).get('bars', 4)
                type_weight *= weights.get('bars', {}).get(bars, 1.0)
            
            # 2. Harmonic/Energy Modifier
            # (If speed_a/speed_b are close to 1.0, and pitch shift is small = higher score)
            acc_score = 1.0
            speed_dev = abs(c.get('speed_a', 1.0) - 1.0) + abs(c.get('speed_b', 1.0) - 1.0)
            acc_score -= (speed_dev * 2.0) # Penalize large stretches
            
            pitch_dev = abs(c.get('pitch_step_b', 0.0))
            acc_score -= (pitch_dev * 0.1) # Penalize large pitch shifts
            
            # 3. Chronological Safety Check
            a_out = c.get('a_out_time', 0)
            safety_penalty = 0
            if min_exit_time is not None and a_out < min_exit_time + 4: # 4s buffer
                safety_penalty = -500 
            
            # Total Score
            score = (type_weight * acc_score * harmonic_factor) + safety_penalty
            # Add VERY MINIMAL randomness to break ties but preserve 'Accuracy'
            score += random.uniform(0.0, 0.01) 
            
            scored_candidates.append((score, c))
        
        # Sort by score
        scored_candidates.sort(key=lambda x: x[0], reverse=True)
        return scored_candidates[0][1]
