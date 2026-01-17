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
from src.utils import ensure_dirs, logger
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

st.set_page_config(layout="wide", page_title="AutoMix DJ Bot (Auto Mode)")

st.title("🎧 AutoMix Hip-Hop DJ Bot (Auto)")
st.markdown("Fully Automated Mixset Generator")

# Session State
if 'playlist' not in st.session_state:
    st.session_state['playlist'] = [] # List of analyzed track dicts
if 'transitions' not in st.session_state:
    st.session_state['transitions'] = [] # List of chosen specs
if 'candidates' not in st.session_state:
    st.session_state['candidates'] = [] # List of List of candidates per transition

# FIXED: Always initialize weights correctly
if 'type_weights' not in st.session_state or 'bar_weights' not in st.session_state:
    type_w, bar_w = load_preference_weights()
    st.session_state['type_weights'] = type_w
    st.session_state['bar_weights'] = bar_w

if 'final_mix_result' not in st.session_state:
    st.session_state['final_mix_result'] = None

# Helper to load tracks
def load_tracks(uploaded_files):
    import time as time_module
    tracks = []
    temp_dir = Path("cache/uploads")
    temp_dir.mkdir(exist_ok=True, parents=True)
    total_files = len(uploaded_files)
    progress_bar = st.progress(0)
    status_text = st.empty()
    time_text = st.empty()
    start_time = time_module.time()
    
    for i, uploaded_file in enumerate(uploaded_files):
        status_text.text(f"📊 Analyzing {i+1}/{total_files}: {uploaded_file.name}")
        file_path = temp_dir / uploaded_file.name
        with open(file_path, "wb") as f:
            f.write(uploaded_file.getbuffer())
        
        try:
            analysis = analyzer.analyze_track(str(file_path), {})
            analysis['filename'] = uploaded_file.name
            analysis['filepath'] = str(file_path)
            analysis['stems'] = {}
            tracks.append(analysis)
        except Exception as e:
            st.error(f"Error analyzing {uploaded_file.name}: {e}")
        
        progress = (i + 1) / total_files
        progress_bar.progress(progress)
        elapsed = time_module.time() - start_time
        if i > 0:
            avg_time_per_file = elapsed / (i + 1)
            estimated_remaining = avg_time_per_file * (total_files - (i + 1))
            time_text.text(f"⏱️ Elapsed: {int(elapsed)}s | Remaining: ~{int(estimated_remaining)}s")
        
    status_text.text(f"✅ Analysis Complete! ({int(time_module.time() - start_time)}s)")
    time_text.empty()
    return tracks

# UI Layout
with st.sidebar:
    st.header("1. Upload Music")
    uploaded_files = st.file_uploader("Select MP3/WAV files", accept_multiple_files=True, type=['mp3', 'wav'])
    
    if uploaded_files and st.button("Analyze & Plan Mix"):
        st.session_state['playlist'] = load_tracks(uploaded_files)
        st.session_state['candidates'] = []
        st.rerun()

# Main Area
if st.session_state['playlist']:
    st.header("2. Playlist & Manual Highlights")
    st.info("💡 각 곡의 시작(In)과 끝(Out) 지점을 설정하세요. AI가 이 지점을 기준으로 최적의 믹스를 구성합니다.")
    
    # --- DEV TOOL: Load Preset ---
    with st.expander("🛠️ Developer Tools"):
        st.caption("JSON 형식을 붙여넣어 한번에 설정을 완료하세요.")
        json_input = st.text_area("Paste Mapping JSON here:", height=150)
        
        if st.button("📥 Apply JSON Mapping"):
            try:
                data = json.loads(json_input)
                found = 0
                for item in data:
                    title = item.get('title', '')
                    m_in = item.get('mix_in_sec', 0.0)
                    m_out = item.get('mix_out_sec', 0.0)
                    
                    for track in st.session_state['playlist']:
                        # Match by title fuzzy
                        if title.lower() in track['filename'].lower() or track['filename'].lower() in title.lower():
                            track['manual_in'] = float(m_in)
                            track['manual_out'] = float(m_out)
                            found += 1
                            break
                st.success(f"✅ {found}곡의 설정이 반영되었습니다!")
                st.session_state['candidates'] = []
                st.rerun()
            except Exception as e:
                st.error(f"JSON Parsing Error: {e}")

        st.divider()
        if st.button("📥 Load 20-Track Demo Mapping (Fix Title Matching)"):
            preset_data = [
                {"title": "Coogie, lobonabeat! - coogieandme (Feat. lobonabeat!) (쿠기랑 나 (Feat. lobonabeat!))", "in": 0.0, "out": 43.0},
                {"title": "DBO, Dok2, Okasian - POP IT UP (feat. Dok2 & Okasian)", "in": 7.0, "out": 74.0},
                {"title": "DBO, E SENS - I Know What I'm Doing (feat. E SENS)", "in": 68.0, "out": 105.0},
                {"title": "DBO, Nochang - 홍길동 (Hong Gil-Dong) (feat. Nochang)", "in": 83.0, "out": 133.04},
                {"title": "Effie - MAKGEOLLI BANGER", "in": 0.0, "out": 68.48},
                {"title": "Eric Reprid - KPOP", "in": 3.0, "out": 42.0},
                {"title": "KC, NOWIMYOUNG - KOLOK COLOK", "in": 0.0, "out": 30.0},
                {"title": "KC, Sik-K, HAON, NOWIMYOUNG - KARMA COLLECTOR", "in": 0.0, "out": 46.0},
                {"title": "Kid Milli, lobonabeat! - Coupe ! (Feat. lobonabeat!) (Coupe ! (Feat. lobonabeat!))", "in": 29.0, "out": 172.0},
                {"title": "Leellamarz, Young B - Let off steam", "in": 0.0, "out": 83.0},
                {"title": "lobonabeat! - Magician", "in": 0.0, "out": 68.0},
                {"title": "ShyboiiTobii - Vivien Westwood", "in": 0.0, "out": 105.0},
                {"title": "ShyboiiTobii (샤이보이토비) - Born ii Ball (feat. LIL GIMCHI & Royal 44)", "in": 62.0, "out": 139.0},
                {"title": "ShyboiiTobii (샤이보이토비) - My Team", "in": 29.0, "out": 103.0},
                {"title": "Sik-K, Lil Moshpit, GroovyRoom, 2SL, Okasian, HOLYDAY - LALALA (Snitch Club)", "in": 0.0, "out": 114.0},
                {"title": "Sik-K, Lil Moshpit, GroovyRoom, Silica Gel, rager_o2 - K-FLIP", "in": 67.0, "out": 134.0},
                {"title": "YANGHONGWON - 4 Seasons(1.12) (사계)", "in": 77.0, "out": 135.0},
                {"title": "YANGHONGWON, Kid Milli, NO：EL, Ksmartboi, Swings - Ballin (Feat. Kid Milli, NO：EL, Ksmartboi, Swings)", "in": 89.0, "out": 153.0},
                {"title": "Yung Blesh, Royal 44, danmish, fomade8464, Festy Wxs - M.D.M.A", "in": 0.0, "out": 71.0},
                {"title": "ZICO, Crush - Yin and Yang", "in": 65.0, "out": 129.38}
            ]
            found = 0
            for item in preset_data:
                for track in st.session_state['playlist']:
                    if item['title'].lower() in track['filename'].lower() or track['filename'].lower() in item['title'].lower():
                        track['manual_in'] = float(item['in'])
                        track['manual_out'] = float(item['out'])
                        found += 1
                        break
            st.success(f"✅ {found}곡 설정 로드 완료!")
            st.session_state['candidates'] = []
            st.rerun()

    st.divider()

    # --- ACTION BUTTON (MOVED UP FOR STABILITY) ---
    if not st.session_state['candidates']:
        if st.button("⚡ ONE-CLICK SMART MIX (Plan & Optimize)", type="primary", use_container_width=True):
            with st.status("🧠 Generating optimal mix plan based on your highlights...", expanded=True) as status:
                playlist = st.session_state['playlist']
                custom_weights = {'types': st.session_state['type_weights'], 'bars': st.session_state['bar_weights']}
                energy_pref = st.session_state['type_weights'].get('energy_build', 1.0)

                # STEP 1: Deep Smart Sort (Optimized Track Ordering)
                status.write("🎯 Step 1: Optimizing track order for harmonic & energy flow...")
                
                def get_key_distance(key1, key2):
                    notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
                    try:
                        root1 = key1.split(' ')[0]; root2 = key2.split(' ')[0]
                        idx1 = notes.index(root1); idx2 = notes.index(root2)
                        diff = abs(idx1 - idx2)
                        if diff > 6: diff = 12 - diff
                        return diff
                    except: return 6
                
                sorted_playlist = [playlist[0]]
                remaining = playlist[1:]
                while remaining:
                    current = sorted_playlist[-1]
                    current_bpm = current['bpm']
                    current_key = current.get('key', 'C Major')
                    current_energy_raw = current.get('energy', 0.5)
                    if isinstance(current_energy_raw, list):
                        current_energy = sum(current_energy_raw) / len(current_energy_raw) if current_energy_raw else 0.5
                    else: current_energy = current_energy_raw
                    
                    scores = []
                    for track in remaining:
                        score = 0
                        dist = get_key_distance(current_key, track.get('key', 'C Major'))
                        score += max(0, 60 - (dist * 10))
                        bpm_diff = abs(track['bpm'] - current_bpm)
                        score += max(0, 20 - bpm_diff)
                        track_energy_raw = track.get('energy', 0.5)
                        if isinstance(track_energy_raw, list):
                            track_energy = sum(track_energy_raw) / len(track_energy_raw) if track_energy_raw else 0.5
                        else: track_energy = track_energy_raw
                        energy_diff = track_energy - current_energy
                        if energy_diff > 0: score += (energy_diff * 40 * energy_pref)
                        else: score += (energy_diff * 10)
                        scores.append((score, track))
                    
                    scores.sort(reverse=True, key=lambda x: x[0])
                    next_track = scores[0][1]
                    sorted_playlist.append(next_track)
                    remaining.remove(next_track)
                
                st.session_state['playlist'] = sorted_playlist
                playlist = sorted_playlist # Update local reference for plotting
                
                # STEP 2: Transition Planning
                status.write("🎲 Step 2: Planning optimal transitions between sorted tracks...")
                total_pairs = len(playlist) - 1
                plan_cands = []
                plan_selections = []
                cur_entry_times = {i: 0.0 for i in range(len(playlist))}

                for i in range(total_pairs):
                    t_a = playlist[i]
                    t_b = playlist[i+1]
                    opts = transition_engine.generate_random_candidates(t_a, t_b, count=5, weights=custom_weights)
                    best = transition_engine.select_best_candidate(opts, weights=custom_weights, min_exit_time=cur_entry_times[i])
                    plan_cands.append(opts)
                    plan_selections.append(best)
                    cur_entry_times[i+1] = best['b_in_time']

                st.session_state['candidates'] = []
                for i, best in enumerate(plan_selections):
                    t_a = playlist[i]
                    t_b = playlist[i+1]
                    if not best.get('preview_path'):
                        best['preview_path'] = renderer.render_preview(t_a['filepath'], t_b['filepath'], best)
                    st.session_state['candidates'].append([best])
                    st.session_state[f"trans_{i}"] = 0
                
                status.update(label="✅ Mix Plan Ready!", state="complete")
                st.rerun()

    # --- 3. FINAL MIX GENERATION (IF PLANNED) ---
    if st.session_state['candidates']:
        st.divider()
        st.header("3. 🎵 Final Mix Generator")
        st.success("✅ Mix planning complete! You can now generate the high-quality final mix.")
        
        final_specs = [opts[0] for opts in st.session_state['candidates']]
        
        col_gen, col_reset = st.columns([3, 1])
        
        if col_gen.button("🎧 GENERATE FINAL CLUB MIX (High Quality)", key="gen_mix", type="primary", use_container_width=True):
            with st.spinner("🎛️ Rendering 20-track mix... This will take a few minutes."):
                out_dir = Path("output")
                out_dir.mkdir(exist_ok=True)
                timestamp = time.strftime("%Y%m%d_%H%M%S")
                out_path = str(out_dir / f"auto_mix_{timestamp}.mp3")
                
                try:
                    result = renderer.render_final_mix(st.session_state['playlist'], final_specs, out_path)
                    
                    if isinstance(result, tuple):
                        mp3_gen, lrc_gen = result
                    else:
                        mp3_gen = result
                        lrc_gen = result.replace(".mp3", ".lrc")

                    st.session_state['final_mix_result'] = {
                        'mp3': mp3_gen,
                        'lrc': lrc_gen,
                        'timestamp': timestamp,
                        'zip': mp3_gen.replace(".mp3", ".zip")
                    }
                    
                    with zipfile.ZipFile(st.session_state['final_mix_result']['zip'], 'w') as zipf:
                        zipf.write(mp3_gen, os.path.basename(mp3_gen))
                        if os.path.exists(lrc_gen):
                            zipf.write(lrc_gen, os.path.basename(lrc_gen))
                    
                    st.success("✅ Mix Rendering Complete!")
                    st.rerun()
                except Exception as e:
                    st.error(f"Render Error: {e}")
                    logger.error(f"Render Error: {e}")

        if col_reset.button("🔄 Re-Plan Mix", use_container_width=True):
            st.session_state['candidates'] = []
            st.rerun()

        # persistent download UI
        if st.session_state['final_mix_result']:
            res = st.session_state['final_mix_result']
            st.info(f"💾 Mix Created at {res['timestamp']}")
            if os.path.exists(res['zip']):
                with open(res['zip'], "rb") as f:
                    st.download_button("📥 Download FULL PACK (MP3 + LRC)", f, file_name=os.path.basename(res['zip']), mime="application/zip", use_container_width=True)

    # --- 4. TRACK SETTINGS & PREPARATION ---
    st.divider()
    st.markdown("### 📋 Track Highlight Settings")
    st.caption("변경 시 현재의 믹스 계획이 초기화되며 새로 계획(Plan)해야 합니다.")

    for i, track in enumerate(st.session_state['playlist']):
        vol_info = f"{track.get('loudness_db', -99):.1f}dB"
        with st.expander(f"🎵 {i+1}. {track['filename']} ({int(track['bpm'])} BPM, {vol_info})", expanded=False):
            st.audio(track['filepath'], format="audio/mp3")
            dur = track['duration']
            cur_in = float(track.get('manual_in', 0.0))
            cur_out = float(track.get('manual_out', dur))
            
            col_in, col_out = st.columns(2)
            new_in = col_in.number_input(f"Mix-In (초)", 0.0, dur, cur_in, step=1.0, key=f"num_in_{i}")
            new_out = col_out.number_input(f"Mix-Out (초)", 0.0, dur, cur_out, step=1.0, key=f"num_out_{i}")
            
            if abs(new_in - cur_in) > 0.01 or abs(new_out - cur_out) > 0.01:
                track['manual_in'] = new_in
                track['manual_out'] = new_out
                st.session_state['candidates'] = []
                st.rerun()
            st.caption(f"🏁 설정 범위: {int(new_in//60)}:{int(new_in%60):02d} ~ {int(new_out//60)}:{int(new_out%60):02d}")
