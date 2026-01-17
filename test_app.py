import streamlit as st
import os, sys, json, random
from pathlib import Path
import pandas as pd
import logging

# Logger Setup
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("TestApp")

sys.path.append(os.path.join(os.path.dirname(__file__), 'src'))
from src.analyzer_engine import AudioAnalyzer
from src.transition_engine import TransitionEngine
from src.mix_renderer import MixRenderer
from src.utils import ensure_dirs

st.set_page_config(layout="wide", page_title="DJ Bot RL Trainer")

# --- RL State Management ---
if 'weights' not in st.session_state:
    st.session_state['weights'] = {
        'features': {}, # Granular feature weights (starts empty, learns dynamically)
        'types': {'crossfade': 1.0, 'bass_swap': 1.0, 'cut': 1.0, 'filter_fade': 1.0, 'mashup': 1.0},
        'bars': {4: 1.0, 8: 1.5},
        'structure': {} 
    }
if 'batch_info' not in st.session_state:
    st.session_state['batch_info'] = {
        'generation': 1,
        'current_count': 0,
        'target_count': 25 # Gen 1 = 25 samples
    }
if 'history' not in st.session_state:
    st.session_state['history'] = []

# ---# Init
ensure_dirs()

# --- Init Engines ---
if 'analyzer' not in st.session_state:
    try:
        import demucs
    except ImportError:
        st.error("Demucs module missing. Please run `pip install demucs`.")
        
    st.session_state['analyzer'] = AudioAnalyzer()
    st.session_state['engine'] = TransitionEngine()
    st.session_state['renderer'] = MixRenderer()

# --- Load Tracks ---
# Correct path per user report: C:\Users\user\Desktop\djbot\mp3fortest
# Since we are running from djbot folder, relative path 'mp3fortest' works best.
data_dir = Path("mp3fortest")

# Fallback check
if not data_dir.exists():
    logger.warning(f"Relative path {data_dir.absolute()} not found. Checking cache/uploads.")
    data_dir = Path("cache/uploads")

# Scan dir
# Add more extensions
files = []
extensions = ["*.mp3", "*.wav", "*.m4a", "*.flac", "*.MP3", "*.WAV", "*.M4A", "*.FLAC"]
for ext in extensions:
    files.extend(list(data_dir.glob(ext)))

# Sidebar: Library Management
with st.sidebar:
    st.markdown("### 📚 Library")
    if st.button("🔄 Reload Library"):
        if 'tracks' in st.session_state:
            del st.session_state['tracks']
        st.rerun()

if 'tracks' not in st.session_state:
    st.session_state['tracks'] = [str(f.absolute()) for f in files]
    st.session_state['track_meta'] = {} 
    
st.sidebar.caption(f"Loaded: {len(st.session_state['tracks'])} tracks")

# Pre-analysis Button
if st.sidebar.button("🧠 Analyze All Tracks"):
    progress_bar = st.sidebar.progress(0)
    for i, t_path in enumerate(st.session_state['tracks']):
        if t_path not in st.session_state['track_meta']:
             st.session_state['analyzer'].analyze_track(t_path, {}) # Ignore cache valid?
        progress_bar.progress((i + 1) / len(st.session_state['tracks']))
    st.sidebar.success("Analysis Complete!")

if not st.session_state['tracks']:
    st.error(f"No files found in {data_dir.absolute()}. (See terminal for debug)")
    print(f"[ERROR] No files found in {data_dir.absolute()}")
    st.stop()

# --- Helper: Granular Weight Update ---
def update_weights(meta, reward):
    w = st.session_state['weights']
    
    # 1. Update Granular Features
    features = meta.get('features', [])
    for f in features:
        current = w['features'].get(f, 1.0)
        # Learn: Increase if loved, decrease if hated
        w['features'][f] = max(0.1, current + (0.2 * reward))
    
    # 2. Update Legacy Types/Bars (Keep for backward compatibility/init)
    t = meta['type']
    w['types'][t] = max(0.1, w['types'].get(t, 1.0) + (0.2 * reward))
    b = meta['bars']
    w['bars'][b] = max(0.1, w['bars'].get(b, 1.0) + (0.2 * reward))
    
    # Update Structure (Legacy fallback if not caught in features)
    # Actually 'Struct:...' is in features now, but keep this if structure key exists?
    # It's safer to just rely on features list now. But let's keep legacy keys sync'd if needed OR just remove.
    # Removing legacy structure update to avoid double counting if feature exists.
    
    st.session_state['weights'] = w

def get_next_pair():
    return random.sample(files, 2)

# --- Main UI ---
st.title("🧠 DJ Bot Reinforcement Learning")
gen = st.session_state['batch_info']['generation']
prog = st.session_state['batch_info']['current_count']
tgt = st.session_state['batch_info']['target_count']

st.progress(prog / tgt, text=f"Generation {gen}: {prog}/{tgt} Samples Rated")

col_main, col_stats = st.columns([2, 1])

# --- Logic: Get Current Candidate ---
if 'rl_candidate' not in st.session_state:
    # Pick a random pair
    f_a, f_b = get_next_pair()
    
    # Analyze
    with st.spinner("Analyzing & Thinking..."):
        # We need analysis for generation
        # Cache check?
        if 'track_meta' not in st.session_state: st.session_state['track_meta'] = {}
        
        def ga(p):
            if str(p) in st.session_state['track_meta']: return st.session_state['track_meta'][str(p)]
            a = st.session_state['analyzer'].analyze_track(str(p), {})
            st.session_state['track_meta'][str(p)] = a
            return a
            
        m_a = ga(f_a)
        m_b = ga(f_b)
        
        # Generative Step (Weighted)
        # Generate just 1 candidate for the user to rate (A/B testing style)
        # user asked for "50 random choices" then "20 new".
        # Better UX: Show 1, Rate, Next.
        cands = st.session_state['engine'].generate_random_candidates(
            m_a, m_b, count=1, weights=st.session_state['weights']
        )
        cand = cands[0]
        
        # Render
        preview = st.session_state['renderer'].render_preview(str(f_a), str(f_b), cand)
        cand['preview_path'] = preview
        
        st.session_state['rl_candidate'] = cand
        st.session_state['rl_pair_names'] = (f_a.name, f_b.name)

# --- Display Candidate ---
cand = st.session_state['rl_candidate']
with col_main:
    st.subheader(f"{st.session_state['rl_pair_names'][0]} ➡️ {st.session_state['rl_pair_names'][1]}")
    st.info(f"**Proposed Mix**: {cand['name']}")
    
    if os.path.exists(cand['preview_path']):
        st.audio(cand['preview_path'], autoplay=False)
    else:
        st.error("Render failed.")

    # Interaction
    c1, c2, c3 = st.columns(3)
    
    def next_step(reward):
        # 1. Learn
        update_weights(cand['meta'], reward)
        
        # Auto-Save to disk (User Request: Ensure progress is never lost)
        with open("preference_weights.json", "w") as f:
            json.dump(st.session_state['weights'], f)
        
        # 2. Log
        st.session_state['history'].append({
            'gen': gen, 'type': cand['meta']['type'], 'reward': reward
        })
        
        # 3. Advance Batch
        st.session_state['batch_info']['current_count'] += 1
        
        # 4. Clear Candidate
        del st.session_state['rl_candidate'] 
        st.rerun()
    
    # Check Gen Complete
    if prog >= tgt:
        st.success(f"🎉 Batch {gen} Complete! ({tgt} Samples)")
        if st.button("Continue Training (Next 25) ⏭️", type="primary"):
            st.session_state['batch_info']['generation'] += 1
            st.session_state['batch_info']['current_count'] = 0
            st.session_state['batch_info']['target_count'] = 25
            st.rerun()
        st.stop() # Stop rendering the rest of the UI until clicked
    
    if c1.button("❤️ Love (+2)", type='primary', use_container_width=True):
        next_step(2.0)
        
    if c2.button("👍 Like (+1)", use_container_width=True):
        next_step(1.0)
        
    if c3.button("👎 Pass (-1)", use_container_width=True):
        next_step(-1.0)

# --- Stats Column ---
with col_stats:
    st.markdown("### 🧠 Detailed Taste Profile")
    st.write(f"**Generation {gen}**")
    
    # Show Top Positive Features
    w_feat = st.session_state['weights']['features']
    if w_feat:
        df = pd.DataFrame.from_dict(w_feat, orient='index', columns=['Score'])
        
        st.markdown("#### ✅ Top Preferences")
        st.bar_chart(df.sort_values('Score', ascending=False).head(10))
        
        st.markdown("#### ❌ Dislikes")
        st.bar_chart(df.sort_values('Score', ascending=True).head(5))
    else:
        st.info("Start rating to build your granular profile!")
        st.info("Start rating to build your granular profile!")
