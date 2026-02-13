import streamlit as st
import os
import sys
import time
from pathlib import Path

# Add src to path
sys.path.append(os.path.join(os.path.dirname(__file__), 'src'))

from src.analyzer_engine import AudioAnalyzer
from src.stem_separator import StemSeparator
from src.transition_engine import TransitionEngine
from src.mix_renderer import MixRenderer
from src.utils import ensure_dirs, logger, get_file_hash
import json
import zipfile

# Initialize
ensure_dirs()
analyzer = AudioAnalyzer()
separator = StemSeparator()
transition_engine = TransitionEngine()
renderer = MixRenderer()

# Load Preference Weights
def load_preference_weights():
    """Load learned weights from test_app.py training"""
    weights_file = "preference_weights.json"
    try:
        if os.path.exists(weights_file):
            import json
            with open(weights_file, 'r') as f:
                data = json.load(f)
                type_weights = data.get('types', {})
                bar_weights_raw = data.get('bars', {})
                
                # Convert string keys to integers for bar weights
                bar_weights = {}
                for k, v in bar_weights_raw.items():
                    try:
                        bar_weights[int(k)] = v
                    except (ValueError, TypeError):
                        bar_weights[k] = v
                
                return type_weights, bar_weights
    except Exception as e:
        st.warning(f"Could not load weights: {e}")
    
    # Default weights if file doesn't exist
    return {
        'crossfade': 0.5,
        'bass_swap': 1.6,
        'cut': 1.2,
        'filter_fade': 1.0,
        'mashup': 1.0
    }, {
        4: 1.2,
        8: 1.5
    }

st.set_page_config(layout="wide", page_title="AutoMix DJ Bot")

st.title("🎧 AutoMix Hip-Hop DJ Bot")
st.markdown("Automated Mixset Generator with Highlight Preservation")

# Session State
if 'playlist' not in st.session_state:
    st.session_state['playlist'] = [] # List of analyzed track dicts
if 'transitions' not in st.session_state:
    st.session_state['transitions'] = [] # List of chosen specs
if 'candidates' not in st.session_state:
    st.session_state['candidates'] = [] # List of List of candidates per transition
if 'type_weights' not in st.session_state:
    type_w, bar_w = load_preference_weights()
    st.session_state['type_weights'] = type_w
    st.session_state['bar_weights'] = bar_w
if 'final_mix_result' not in st.session_state:
    st.session_state['final_mix_result'] = None

# Helper to load tracks
def load_tracks(uploaded_files):
    import time as time_module
    tracks = []
    seen_hashes = set()
    
    # Save uploaded files to temp/cache so we can access by path
    temp_dir = Path("cache/uploads")
    temp_dir.mkdir(exist_ok=True, parents=True)
    
    total_files = len(uploaded_files)
    progress_bar = st.progress(0)
    status_text = st.empty()
    time_text = st.empty()
    
    start_time = time_module.time()
    
    for i, uploaded_file in enumerate(uploaded_files):
        file_start = time_module.time()
        
        status_text.text(f"📊 Analyzing {i+1}/{total_files}: {uploaded_file.name}")
        
        # Save file
        file_path = temp_dir / uploaded_file.name
        with open(file_path, "wb") as f:
            f.write(uploaded_file.getbuffer())
        
        # Analyze
        # Skip stem separation for now due to TorchCodec dependency issues
        try:
            file_hash = get_file_hash(str(file_path))
            if file_hash in seen_hashes:
                status_text.text(f"⏭️ Skipping duplicate: {uploaded_file.name}")
                continue
            seen_hashes.add(file_hash)

            stems = {}  # Empty stems dict - skip Demucs
            analysis = analyzer.analyze_track(str(file_path), stems)
            analysis['filename'] = uploaded_file.name
            analysis['filepath'] = str(file_path)
            analysis['stems'] = stems
            tracks.append(analysis)
            
        except Exception as e:
            st.error(f"Error analyzing {uploaded_file.name}: {e}")
        
        # Update progress
        progress = (i + 1) / total_files
        progress_bar.progress(progress)
        
        # Calculate time estimates
        elapsed = time_module.time() - start_time
        if i > 0:
            avg_time_per_file = elapsed / (i + 1)
            remaining_files = total_files - (i + 1)
            estimated_remaining = avg_time_per_file * remaining_files
            
            elapsed_str = f"{int(elapsed)}s"
            remaining_str = f"{int(estimated_remaining)}s"
            time_text.text(f"⏱️ Elapsed: {elapsed_str} | Remaining: ~{remaining_str}")
        
    total_time = time_module.time() - start_time
    status_text.text(f"✅ Analysis Complete! ({int(total_time)}s total)")
    time_text.empty()
    
    return tracks

# UI Layout
with st.sidebar:
    st.header("1. Upload Music")
    uploaded_files = st.file_uploader("Select MP3/WAV files", accept_multiple_files=True, type=['mp3', 'wav'])
    
    if uploaded_files and st.button("Analyze Tracks"):
        st.session_state['playlist'] = load_tracks(uploaded_files)
        st.session_state['candidates'] = [] # Reset candidates
        st.session_state['transitions'] = [None] * (len(st.session_state['playlist']) - 1)
        st.rerun()
    
    st.divider()
    
    # Weight Management
    with st.expander("⚙️ Preference Weights", expanded=False):
        st.caption("Learned from test_app.py training")
        
        st.write("**Transition Types:**")
        for t_type in ['crossfade', 'bass_swap', 'cut', 'filter_fade', 'mashup']:
            st.session_state['type_weights'][t_type] = st.slider(
                t_type.replace('_', ' ').title(),
                0.1, 10.0,
                float(st.session_state['type_weights'].get(t_type, 1.0)),
                0.1,
                key=f"weight_{t_type}"
            )
        
        st.write("**Bar Lengths:**")
        st.caption("4/8 bar transition preference (learned + manual tuning)")
        for bars in [4, 8]:
            st.session_state['bar_weights'][bars] = st.slider(
                f"{bars} Bars",
                0.1, 10.0,
                float(st.session_state['bar_weights'].get(bars, 1.0)),
                0.1,
                key=f"weight_{bars}bar"
            )
        
        col1, col2 = st.columns(2)
        if col1.button("💾 Save Weights"):
            import json
            data = {
                'types': st.session_state['type_weights'],
                'bars': st.session_state['bar_weights'],
                'features': {},
                'structure': {}
            }
            with open("preference_weights.json", 'w') as f:
                json.dump(data, f, indent=2)
            st.success("Saved!")
        
        if col2.button("🔄 Reload"):
            type_w, bar_w = load_preference_weights()
            st.session_state['type_weights'] = type_w
            st.session_state['bar_weights'] = bar_w
            st.rerun()

# Main Area
if st.session_state['playlist']:
    st.header("2. Arrange & Mix")
    
    col_list, col_mix = st.columns([1, 2])
    
    with col_list:
        st.subheader("Playlist Order")
        
        for i, track in enumerate(st.session_state['playlist']):
            # Show Volume info
            vol_info = f"{track.get('loudness_db', -99):.1f}dB"
            st.markdown(f"**{i+1}. {track['filename']}** ({int(track['bpm'])} BPM, {vol_info})")
            
            # Manual Highlight (Advanced)
            with st.expander(f"🎛️ Manual 구간 설정 (Track {i+1})"):
                dur = track['duration']
                
                # Default values if not set
                cur_in = track.get('manual_in', 0.0)
                cur_out = track.get('manual_out', dur)
                
                # Slider for In/Out points
                m_in = st.slider(f"Mix-In 시작 (초)", 0.0, dur, float(cur_in), step=1.0, key=f"in_{i}")
                m_out = st.slider(f"Mix-Out 종료 (초)", 0.0, dur, float(cur_out), step=1.0, key=f"out_{i}")
                
                if m_in != cur_in: 
                    track['manual_in'] = m_in
                    st.session_state['candidates'] = [] # Reset candidates if timing changes
                if m_out != cur_out: 
                    track['manual_out'] = m_out
                    st.session_state['candidates'] = []
                
                st.caption(f"💡 현재 설정: {m_in:.1f}s ~ {m_out:.1f}s (약 {int((m_out-m_in)/60)}분 {int((m_out-m_in)%60)}초)")

        st.divider()
        # Consolidated Intelligent Mix Button - MOVED HERE
        if st.button("⚡ ONE-CLICK SMART MIX (Plan & Optimize)", type="primary", use_container_width=True):
            if len(st.session_state['playlist']) < 2:
                st.error("Need 2+ tracks!")
            else:
                with st.status("🧠 Intelligent Mix Planning (5 Scenarios)...", expanded=True) as status:
                    # STEP 1: Deep Smart Sort (Internalized)
                    status.write("🎯 Step 1: Optimizing track order for harmonic & energy flow...")
                    playlist = st.session_state['playlist']
                    type_w, bar_w = st.session_state['type_weights'], st.session_state['bar_weights']
                    energy_pref = type_w.get('energy_build', 1.0)

                    notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
                    note_to_idx = {n: i for i, n in enumerate(notes)}

                    def normalize_energy(track):
                        raw = track.get('energy', 0.5)
                        if isinstance(raw, list):
                            return sum(raw) / len(raw) if raw else 0.5
                        return float(raw)

                    def get_key_distance(key1, key2):
                        try:
                            root1 = key1.split(' ')[0]
                            root2 = key2.split(' ')[0]
                            idx1 = note_to_idx[root1]
                            idx2 = note_to_idx[root2]
                            diff = abs(idx1 - idx2)
                            if diff > 6:
                                diff = 12 - diff
                            return diff
                        except Exception:
                            return 6
                    
                    sorted_playlist = [playlist[0]]
                    remaining = playlist[1:]
                    while remaining:
                        current = sorted_playlist[-1]
                        current_bpm = current['bpm']
                        current_key = current.get('key', 'C Major')
                        current_energy = normalize_energy(current)
                        
                        scores = []
                        for track in remaining:
                            score = 0
                            dist = get_key_distance(current_key, track.get('key', 'C Major'))
                            score += max(0, 60 - (dist * 10))
                            bpm_diff = abs(track['bpm'] - current_bpm)
                            score += max(0, 20 - bpm_diff)
                            track_energy = normalize_energy(track)
                            energy_diff = track_energy - current_energy
                            if energy_diff > 0: score += (energy_diff * 40 * energy_pref)
                            else: score += (energy_diff * 10)
                            scores.append((score, track))
                        
                        scores.sort(reverse=True, key=lambda x: x[0])
                        next_track = scores[0][1]
                        sorted_playlist.append(next_track)
                        remaining.remove(next_track)
                    
                    st.session_state['playlist'] = sorted_playlist
                    st.session_state['transitions'] = [None] * (len(sorted_playlist) - 1)
                    
                    # STEP 2: Multi-Plan Generation (5 Scenarios for Efficiency)
                    status.write("🎲 Step 2: Generating and scoring mix scenarios...")
                    total_pairs = len(st.session_state['playlist']) - 1
                    custom_weights = {'types': st.session_state['type_weights'], 'bars': st.session_state['bar_weights']}
                    
                    best_total_score = -9999
                    best_full_plan_cands = []
                    best_full_selections = []

                    num_scenarios = min(5, max(3, total_pairs + 1))  # adaptive for speed/quality balance
                    for plan_idx in range(num_scenarios):
                        status.write(f"Evaluating scenario {plan_idx+1}/{num_scenarios}...")
                        plan_score = 0
                        plan_cands = []
                        plan_selections = []
                        cur_entry_times = {i: 0.0 for i in range(len(st.session_state['playlist']))}

                        for i in range(total_pairs):
                            t_a = st.session_state['playlist'][i]
                            t_b = st.session_state['playlist'][i+1]
                            opts = transition_engine.generate_random_candidates(t_a, t_b, count=8, weights=custom_weights)
                            best = transition_engine.select_best_candidate(opts, weights=custom_weights, min_exit_time=cur_entry_times[i])
                            plan_score += custom_weights['types'].get(best['type'], 1.0)
                            plan_cands.append(opts)
                            plan_selections.append(best)
                            cur_entry_times[i+1] = best['b_in_time']

                        if plan_score > best_total_score:
                            best_total_score = plan_score
                            best_full_plan_cands = plan_cands
                            best_full_selections = plan_selections

                    # STEP 3: Render and Setup UI
                    status.write("🎨 Step 3: Preparing best choice previews...")
                    st.session_state['candidates'] = []
                    for i, best in enumerate(best_full_selections):
                        t_a = st.session_state['playlist'][i]
                        t_b = st.session_state['playlist'][i+1]
                        if not best.get('preview_path'):
                            best['preview_path'] = renderer.render_preview(t_a['filepath'], t_b['filepath'], best)
                        
                        other_opts = [o for o in best_full_plan_cands[i] if o != best]
                        reordered_opts = [best] + other_opts
                        st.session_state['candidates'].append(reordered_opts)
                        st.session_state[f"trans_{i}"] = 0
                    
                    status.update(label=f"✅ Smart Mix Optimized (Scenario Score: {best_total_score:.1f})", state="complete")
                    st.success("Analysis, Sort, and Planning complete! See results below.")
                    st.rerun()

    with col_mix:
        if st.session_state['candidates']:
            st.subheader("3. Select Transitions")
            st.info("Listen to options and select your favorite.")
            
            final_specs = []
            
            for i, opts in enumerate(st.session_state['candidates']):
                t_a = st.session_state['playlist'][i]
                t_b = st.session_state['playlist'][i+1]
                
                st.markdown(f"#### {i+1}. {t_a['filename']} ➡️ {t_b['filename']}")
                
                # Skip if no valid options
                if not opts:
                    st.warning("No valid transitions generated for this pair.")
                    continue
                
                # Create columns based on actual number of options (max 3)
                num_opts = min(len(opts), 3)
                cols = st.columns(num_opts)
                selected_idx = 0
                
                # Display Players
                for idx, opt in enumerate(opts[:num_opts]):  # Only show first 3
                    with cols[idx]:
                        st.markdown(f"**{opt['name']}**")
                        if opt.get('preview_path') and os.path.exists(opt['preview_path']):
                            st.audio(opt['preview_path'], format="audio/mp3")
                        else:
                            st.error(f"Error: {opt.get('error', 'Unknown')}")
                        st.caption(opt['description'])

                # Selection Safety: Reset if index is now out of bounds
                key = f"trans_{i}"
                if key in st.session_state and st.session_state[key] >= num_opts:
                    st.session_state[key] = 0

                selected_idx = st.radio(
                    f"Select for Pair {i+1}", 
                    options=list(range(num_opts)),
                    format_func=lambda x: str(opts[x]['name']),
                    key=key,
                    horizontal=True,
                    label_visibility="collapsed"
                )
                
                final_specs.append(opts[selected_idx])
            
            st.markdown("---")
            st.subheader("🎵 Generate Final Mix")
            
            # Show mix preview
            total_duration = sum([spec.get('duration', 10) for spec in final_specs])
            st.info(f"**Mix Preview:** {len(st.session_state['playlist'])} tracks • ~{total_duration/60:.1f} min • {len(final_specs)} transitions")
            
            if st.button("🎧 Generate Final Club Mix (MP3 + LRC)", key="gen_mix"):
                with st.spinner("🎛️ Rendering final mix... This may take a few minutes."):
                    out_dir = Path("output")
                    out_dir.mkdir(exist_ok=True)
                    
                    # Generate filename with timestamp
                    import datetime
                    timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
                    out_path = str(out_dir / f"club_mix_{timestamp}.mp3")
                    
                    try:
                        result = renderer.render_final_mix(
                            st.session_state['playlist'], 
                            final_specs, 
                            out_path
                        )
                        
                        # Fix: Get paths correctly from result
                        if isinstance(result, tuple):
                            mp3_gen, lrc_gen = result
                        else:
                            mp3_gen = result
                            lrc_gen = result.replace(".mp3", ".lrc")

                        st.session_state['final_mix_result'] = {
                            'mp3': mp3_gen,
                            'lrc': lrc_gen,
                            'timestamp': timestamp
                        }
                        
                        # Create ZIP package
                        zip_path = mp3_gen.replace(".mp3", ".zip")
                        with zipfile.ZipFile(zip_path, 'w') as zipf:
                            zipf.write(mp3_gen, os.path.basename(mp3_gen))
                            if os.path.exists(lrc_gen):
                                zipf.write(lrc_gen, os.path.basename(lrc_gen))
                        st.session_state['final_mix_result']['zip'] = zip_path
                        
                        st.success("✅ Mix Generated Successfully!")
                        st.rerun()
                        
                    except Exception as e:
                        st.error(f"Render Error: {e}")
                        logger.error(f"Render Error: {e}")

            # Persistent Download UI
            if st.session_state['final_mix_result']:
                res = st.session_state['final_mix_result']
                st.markdown("---")
                st.subheader("🎉 Your Mix is Ready!")
                
                col_dl, col_info = st.columns([1, 1])
                
                with col_dl:
                    if 'zip' in res and os.path.exists(res['zip']):
                        with open(res['zip'], "rb") as f:
                            st.download_button(
                                "📥 Download FULL PACK (MP3 + LRC)",
                                f,
                                file_name=os.path.basename(res['zip']),
                                mime="application/zip",
                                type="primary",
                                use_container_width=True
                            )
                    
                    sub_col1, sub_col2 = st.columns(2)
                    with sub_col1:
                        if os.path.exists(res['mp3']):
                            with open(res['mp3'], "rb") as f:
                                st.download_button("🎵 MP3 Only", f, file_name=os.path.basename(res['mp3']), mime="audio/mpeg", use_container_width=True)
                    with sub_col2:
                        if os.path.exists(res['lrc']):
                            with open(res['lrc'], "rb") as f:
                                st.download_button("📄 LRC Only", f, file_name=os.path.basename(res['lrc']), mime="text/plain", use_container_width=True)

                with col_info:
                    st.info(f"💾 **Package Created**: `{res['timestamp']}`\n\n💡 **Tip**: Use the **Full Pack** for the best experience. Load both the MP3 and LRC into your player to see track titles and use 'Lyric Jump' navigation!")
