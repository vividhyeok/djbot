/**
 * AutoMix DJ Bot — app.js
 * Full automation: YouTube download → local upload → analyze → plan → preview → render
 */

const { invoke, convertFileSrc } = window.__TAURI__.core;

// ── State ─────────────────────────────────────────────────────
let state = {
  workerPort: null,
  tracks: [],        // { filename, file|null, path, analysis }
  plan: null,        // MixPlan from /plan
  selectedTrans: [], // selected TransitionSpec per pair
  previewPaths: [],  // preview mp3 paths [[opt0, opt1, opt2], ...]
  versions: [],      // array of generated mix data
  currentVersionIndex: -1,
  zoomLevel: 1.0,    // UI zoom level
};

// ── DOM refs ──────────────────────────────────────────────────
const $ = id => document.getElementById(id);
const uploadZone = $('uploadZone');
const fileInput = $('fileInput');
const trackList = $('trackList');
const tracksEl = $('tracks');
const trackCount = $('trackCount');
const smartMixBtn = $('smartMixBtn');
const clearBtn = $('clearBtn');
const progressPanel = $('progressPanel');
const progressLabel = $('progressLabel');
const progressPct = $('progressPct');
const progressFill = $('progressFill');
const progressSub = $('progressSub');
const transPanel = $('transitionsPanel');
const transList = $('transitionsList');
const generateBtn = $('generateBtn');
const resultPanel = $('resultPanel');
const emptyState = $('emptyState');
const statusBadge = $('statusBadge');
const statusText = $('statusText');
const statusDot = statusBadge.querySelector('.dot');
const ytDownloadBtn = $('ytDownloadBtn');
const ytUrl = $('ytUrl');
const ytMax = $('ytMax');
const ytStatus = $('ytStatus');

// ── Worker URL helper ─────────────────────────────────────────
const api = path => `http://127.0.0.1:${state.workerPort}${path}`;

async function workerFetch(path, opts = {}) {
  const r = await fetch(api(path), opts);
  if (!r.ok) throw new Error(`HTTP ${r.status}: ${await r.text()}`);
  return r.json();
}

// ── Startup ───────────────────────────────────────────────────
async function init() {
  for (let i = 0; i < 60; i++) {
    try { state.workerPort = await invoke('get_worker_port'); break; }
    catch { await sleep(500); }
  }
  if (!state.workerPort) { setStatus('error', 'Go 워커 연결 실패'); return; }
  try { await workerFetch('/health'); setStatus('ok', '준비됨'); }
  catch { setStatus('error', '워커 응답 없음'); }
}

// ── Status / progress helpers ─────────────────────────────────
function setStatus(type, msg) {
  statusText.textContent = msg;
  statusDot.className = 'dot' + (type === 'ok' ? ' ok' : type === 'error' ? ' err' : '');
}

function setProgress(label, pct, sub = '') {
  progressLabel.textContent = label;
  progressPct.textContent = Math.round(pct) + '%';
  progressFill.style.width = pct + '%';
  progressSub.textContent = sub;
}

function showPanel(name) {
  [progressPanel, transPanel, resultPanel, emptyState].forEach(p => p?.classList.add('hidden'));
  if (name === 'empty') emptyState.classList.remove('hidden');
  else if (name) $(name + 'Panel')?.classList.remove('hidden');
}

const sleep = ms => new Promise(r => setTimeout(r, ms));
const fmt = sec => `${Math.floor(sec / 60)}:${String(Math.floor(sec % 60)).padStart(2, '0')}`;
const fileUrl = p => p ? convertFileSrc(p) : '';

// ── Download helper (prevents Tauri navigation) ──────────────
// Instead of setting href = asset:// URL which causes Tauri to navigate away,
// we fetch the file via the backend HTTP server and trigger a Blob download.
async function fetchAndDownload(filePath, filename) {
  if (!filePath) return;
  try {
    const res = await fetch(api('/files/serve') + '?path=' + encodeURIComponent(filePath));
    if (!res.ok) throw new Error('파일 서빙 실패: ' + res.status);
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    setTimeout(() => { URL.revokeObjectURL(a.href); a.remove(); }, 1000);
  } catch (e) {
    toast('다운로드 실패: ' + e.message, 'error');
  }
}

// ── Toast Notification ─────────────────────────────────
const toastEl = document.getElementById('toastContainer');
function toast(msg, type = '', duration = 4000) {
  const el = document.createElement('div');
  el.className = 'toast' + (type ? ` ${type}` : '');
  el.textContent = msg;
  toastEl.appendChild(el);
  setTimeout(() => {
    el.style.animation = 'none';
    el.style.opacity = '0';
    el.style.transition = 'opacity .2s';
    setTimeout(() => el.remove(), 200);
  }, duration);
}

// ── YouTube download ──────────────────────────────────────────
ytDownloadBtn.addEventListener('click', async () => {
  const url = ytUrl.value.trim();
  if (!url) { showYtStatus('URL을 입력하세요', 'err'); toast('URL을 입력하세요', 'error'); return; }
  const max = parseInt(ytMax.value) || 20;

  ytDownloadBtn.disabled = true;
  showYtStatus('⏳ 다운로드 중... (몇 분 걸릴 수 있습니다)', '');

  try {
    const data = await workerFetch('/download/youtube', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url, max_tracks: max }),
    });
    if (data.error) throw new Error(data.error);

    const files = data.files ?? [];
    if (files.length === 0) throw new Error('다운로드된 파일 없음');

    for (const f of files) {
      if (!state.tracks.find(t => t.filename === f.filename)) {
        state.tracks.push({ filename: f.filename, file: null, path: f.path, analysis: null });
      }
    }
    showYtStatus(`${files.length}곡 다운로드 완료`, 'ok');
    renderTrackList();
  } catch (e) {
    showYtStatus('다운로드 실패: ' + e.message, 'err');
    toast('YouTube 다운로드 실패: ' + e.message, 'error');
  } finally {
    ytDownloadBtn.disabled = false;
  }
});

function showYtStatus(msg, cls) {
  ytStatus.textContent = msg;
  ytStatus.className = 'yt-status' + (cls ? ` ${cls}` : '');
  ytStatus.classList.remove('hidden');
}

// ── Local file handling ───────────────────────────────────────
function addFiles(files) {
  for (const f of files) {
    if (state.tracks.find(t => t.filename === f.name)) continue;
    state.tracks.push({ filename: f.name, file: f, path: null, analysis: null, enabled: true });
  }
  renderTrackList();
}

uploadZone.addEventListener('dragover', e => { e.preventDefault(); uploadZone.classList.add('drag-over'); });
uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('drag-over'));
uploadZone.addEventListener('drop', e => { e.preventDefault(); uploadZone.classList.remove('drag-over'); addFiles([...e.dataTransfer.files]); });
uploadZone.addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', () => addFiles([...fileInput.files]));
clearBtn.addEventListener('click', () => {
  // Replace browser confirm with immediate action (no native dialog)
  state.tracks = [];
  renderTrackList();
  showPanel('empty');
  toast('트랙 목록을 지웠습니다', '', 2000);
});

// Select-all from section header
const selectAllBtn = document.getElementById('selectAllBtn');
selectAllBtn?.addEventListener('click', () => {
  const allEnabled = state.tracks.every(t => t.enabled);
  state.tracks.forEach(t => t.enabled = !allEnabled);
  renderTrackList();
});

function renderTrackList() {
  const n = state.tracks.length;
  const enabledCount = state.tracks.filter(t => t.enabled).length;
  trackCount.textContent = n;
  if (n === 0) { trackList.classList.add('hidden'); showPanel('empty'); return; }
  trackList.classList.remove('hidden');
  smartMixBtn.disabled = enabledCount < 2;

  // Update select-all button label
  const saBtn = document.getElementById('selectAllBtn');
  if (saBtn) {
    const allEnabled = state.tracks.every(t => t.enabled);
    saBtn.textContent = allEnabled ? '전체 해제' : '전체 선택';
  }

  tracksEl.innerHTML = state.tracks.map((t, i) => `
    <div class="track-item ${t.enabled ? 'enabled' : 'disabled'}" data-index="${i}">
      <div class="track-checkbox ${t.enabled ? 'checked' : ''}" data-check="${i}">
        ${t.enabled ? '<svg width="10" height="10" viewBox="0 0 10 10" fill="none"><polyline points="1.5,5 4,7.5 8.5,2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' : ''}
      </div>
      <div class="track-num">${i + 1}</div>
      <div class="track-info">
        <div class="track-name">${t.filename}</div>
        <div class="track-meta">
          ${t.analysis
      ? `<span>${Math.round(t.analysis.bpm)} BPM</span><span>${t.analysis.key}</span><span>${fmt(t.analysis.duration)}</span>`
      : '<span>대기 중</span>'}
        </div>
      </div>
      <button class="btn-track-delete" data-del="${i}" title="삭제">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
      </button>
    </div>`).join('');

  // Checkbox toggle
  tracksEl.querySelectorAll('[data-check]').forEach(el => {
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = +el.dataset.check;
      state.tracks[idx].enabled = !state.tracks[idx].enabled;
      renderTrackList();
    });
  });

  // Track row click also toggles
  tracksEl.querySelectorAll('.track-item').forEach(el => {
    el.addEventListener('click', () => {
      const idx = +el.dataset.index;
      state.tracks[idx].enabled = !state.tracks[idx].enabled;
      renderTrackList();
    });
  });

  // Individual delete
  tracksEl.querySelectorAll('[data-del]').forEach(el => {
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = +el.dataset.del;
      state.tracks.splice(idx, 1);
      renderTrackList();
      if (state.tracks.length === 0) showPanel('empty');
    });
  });
}

// ── Upload local files to Go worker ──────────────────────────
async function uploadLocalFiles() {
  const localTracks = state.tracks.filter(t => t.file && !t.path);
  if (localTracks.length === 0) return;
  const form = new FormData();
  for (const t of localTracks) form.append('files', t.file, t.filename);
  const res = await fetch(api('/upload'), { method: 'POST', body: form });
  const data = await res.json();
  const fileMap = {};
  for (const f of (data.files ?? [])) fileMap[f.filename] = f.path;
  for (const t of state.tracks) { if (fileMap[t.filename]) t.path = fileMap[t.filename]; }
}

// ── Smart Mix (fully automatic) ──────────────────────────────
smartMixBtn.addEventListener('click', smartMix);

async function smartMix() {
  showPanel('progress');
  setProgress('파일 준비 중...', 5);

  try {
    // 1. Upload local files
    await uploadLocalFiles();

    // 2. Analyze tracks
    const toAnalyze = state.tracks.filter(t => t.enabled && t.path && !t.analysis);
    if (toAnalyze.length > 0) {
      setProgress('분석 중...', 10, `${toAnalyze.length}곡 분석 중...`);
      try {
        const data = await workerFetch('/analyze', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ filepaths: toAnalyze.map(t => t.path) })
        });
        let analyzedOk = 0;
        (data.results ?? []).forEach((r, i) => {
          if (r?.duration > 0) { toAnalyze[i].analysis = r; analyzedOk++; }
        });
        if (analyzedOk === 0) {
          const backendErrors = data.errors ?? [];
          const errMsg = backendErrors.length > 0 ? backendErrors[0].substring(0, 200) : '분석 결과 없음 (ffmpeg 설치 확인)';
          toast(`분석 실패: ${errMsg}`, 'error', 8000);
          showPanel('empty'); return;
        }
        if (analyzedOk < toAnalyze.length) {
          toast(`${toAnalyze.length - analyzedOk}곡 분석 실패 (나머지 ${analyzedOk}곡으로 진행)`, 'error', 5000);
        }
        renderTrackList();
      } catch (e) {
        toast('분석 요청 실패: ' + e.message, 'error', 6000);
        showPanel('empty'); return;
      }
    }

    setProgress('믹스 플래닝 중...', 45);

    // 3. Plan mix
    const analyzedTracks = state.tracks.filter(t => t.enabled && t.analysis).map(t => t.analysis);
    if (analyzedTracks.length < 2) {
      toast(`활성 분석 트랙이 ${analyzedTracks.length}곡입니다. 2곡 이상 필요합니다.`, 'error', 5000);
      showPanel('empty'); return;
    }
    const planData = await workerFetch('/plan', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tracks: analyzedTracks, scenarios: 5 })
    });
    state.plan = planData.plan;
    state.selectedTrans = [...state.plan.selections];

    // 4. Render final mix directly (no preview/selection step)
    await renderFinalMix();

  } catch (e) {
    toast('스마트 믹스 실패: ' + e.message, 'error', 6000);
    showPanel('empty');
  }
}

// ── Render final mix ───────────────────────────────────
async function renderFinalMix() {
  setProgress('최종 믹스 렌더링 중...', 60, '몇 분 걸릴 수 있습니다...');

  const outputDir = await invoke('get_output_dir');
  const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const outputPath = outputDir + '\\' + `club_mix_${ts}.mp3`;

  const tracks = state.plan.sorted_tracks;
  const playlist = tracks.map((t, i) => ({
    filepath: t.filepath,
    filename: t.filepath.split(/[\\/]/).pop(),
    duration: t.duration,
    bpm: t.bpm,
    play_start: state.selectedTrans[i - 1]?.b_in_time ?? 0,
    play_end: state.selectedTrans[i]
      ? Math.min(state.selectedTrans[i].a_out_time + state.selectedTrans[i].duration, t.duration)
      : t.duration,
  }));

  try {
    const data = await workerFetch('/render/mix', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ playlist, transitions: state.selectedTrans, output_path: outputPath })
    });

    setProgress('완료!', 100);
    await sleep(300);

    // Save version
    const versionNum = state.versions.length + 1;
    const versionData = {
      ts,
      title: `버전 ${versionNum}`,
      plan: JSON.parse(JSON.stringify(state.plan)),
      selectedTrans: JSON.parse(JSON.stringify(state.selectedTrans)),
      data: data
    };
    state.versions.push(versionData);
    state.currentVersionIndex = state.versions.length - 1;

    renderResultVersions();
    showPanel('result');
  } catch (e) {
    toast('믹스 생성 실패: ' + e.message, 'error', 6000);
    showPanel('empty');
  }
}

// ── Render Result UI (Versions, Timeline) ────────────────────
function renderResultVersions() {
  const vList = $('versionsList');
  vList.innerHTML = '';

  state.versions.forEach((v, idx) => {
    const tab = document.createElement('button');
    tab.className = `version-tab ${idx === state.currentVersionIndex ? 'active' : ''}`;
    tab.textContent = v.title;
    tab.onclick = () => {
      state.currentVersionIndex = idx;
      renderResultVersions();
    };
    vList.appendChild(tab);
  });

  const curr = state.versions[state.currentVersionIndex];
  if (!curr) return;

  const tracks = curr.plan.sorted_tracks;
  const numTrans = curr.selectedTrans.length;

  $('versionTitle').textContent = curr.title;
  $('resultInfo').textContent = `${tracks.length}곡 · ${numTrans}개 트랜지션 (생성: ${curr.ts.split('-').slice(0, 3).join('-')} ${curr.ts.split('-').slice(3).join(':')})`;

  // Audio player
  const mixAudio = $('mixAudio');
  if (mixAudio.src !== fileUrl(curr.data.mp3_path)) {
    mixAudio.src = fileUrl(curr.data.mp3_path);
    mixAudio.load();
  }

  // Links
  $('downloadMp3Btn').onclick = () => fetchAndDownload(curr.data.mp3_path, `AutoMix_${curr.title}.mp3`);
  $('downloadLrcBtn').onclick = () => fetchAndDownload(curr.data.lrc_path, `AutoMix_${curr.title}.lrc`);

  // Draw timeline
  renderTimeline(curr);
}

function renderTimeline(ver) {
  const tl = $('timelineList');
  tl.innerHTML = '';
  // Keep track of start time in pure seconds
  let currentSeconds = 0;

  ver.plan.sorted_tracks.forEach((t, i) => {
    const transIn = ver.selectedTrans[i - 1];
    const transOut = ver.selectedTrans[i];

    const item = document.createElement('div');
    item.className = 'timeline-item';

    const minutes = Math.floor(currentSeconds / 60);
    const seconds = Math.floor(currentSeconds % 60);
    const timeStr = `${minutes}:${seconds.toString().padStart(2, '0')}`;

    const trackName = t.filepath.split(/[\\/]/).pop().replace('.mp3', '');
    const transName = transOut ? `${transOut.type} (${transOut.name})` : 'END';

    item.innerHTML = `
      <div class="timeline-time">${timeStr}</div>
      <div class="timeline-track">${trackName}</div>
      ${transOut ? `<div class="timeline-trans">${transName}</div>` : ''}
    `;

    // Click to scrub audio
    const seekTime = currentSeconds;
    item.addEventListener('click', () => {
      const audio = $('mixAudio');
      audio.currentTime = seekTime;
      audio.play().catch(() => { });
    });

    tl.appendChild(item);

    // Calc next track start time in seconds
    const playDur = (transOut ? transOut.a_out_time : t.duration) - (transIn ? transIn.b_in_time : 0);
    currentSeconds += playDur;
  });
}

// ── Actions: Zip, Regenerate, New Mix, Clear ─────────────────
$('downloadZipBtn')?.addEventListener('click', async () => {
  const curr = state.versions[state.currentVersionIndex];
  if (!curr) return;

  toast('ZIP 파일을 생성 중입니다...', 'ok', 3000);
  try {
    const url = `http://127.0.0.1:${state.workerPort}/export/zip`;
    const res = await fetch(url, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        mp3_path: curr.data.mp3_path,
        lrc_path: curr.data.lrc_path,
        mix_name: `AutoMix_${curr.title.replace(' ', '')}`
      })
    });

    if (!res.ok) {
      throw new Error(await res.text());
    }

    // Convert binary to blob and download
    const blob = await res.blob();
    const dummy = document.createElement('a');
    dummy.href = URL.createObjectURL(blob);
    dummy.download = `AutoMix_${curr.title.replace(' ', '')}.zip`;
    dummy.click();
    toast('ZIP 다운로드 완료', 'ok', 2000);
  } catch (e) {
    toast('ZIP 생성 실패: ' + e.message, 'error', 4000);
  }
});

// Regenerate: re-plan with a fresh scenario and re-render
$('regenerateBtn')?.addEventListener('click', async () => {
  if (!state.tracks.length) return;
  // Reset analysis cache so we get a fresh plan but skip re-analysis
  state.plan = null;
  state.selectedTrans = [];
  await smartMix();
});

$('newMixBtn')?.addEventListener('click', () => {
  state.tracks = []; state.plan = null;
  state.selectedTrans = []; state.versions = []; state.currentVersionIndex = -1;
  renderTrackList();
  showPanel('empty');
});

$('clearCacheBtn')?.addEventListener('click', async () => {
  try {
    await workerFetch('/cache/clear', { method: 'POST' });
    toast('캐시가 정리되었습니다.', 'ok', 3000);
  } catch (e) {
    toast('캐시 정리 실패', 'error', 3000);
  }
});

// ── UI Zoom (UX Improvement) ──────────────────────────────────
function applyZoom() {
  document.body.style.zoom = state.zoomLevel;
}

window.addEventListener('keydown', (e) => {
  if (e.ctrlKey || e.metaKey) {
    if (e.key === '=' || e.key === '+') {
      e.preventDefault();
      state.zoomLevel = Math.min(state.zoomLevel + 0.1, 2.0);
      applyZoom();
    } else if (e.key === '-') {
      e.preventDefault();
      state.zoomLevel = Math.max(state.zoomLevel - 0.1, 0.5);
      applyZoom();
    } else if (e.key === '0') {
      e.preventDefault();
      state.zoomLevel = 1.0;
      applyZoom();
    }
  }
});

window.addEventListener('wheel', (e) => {
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault();
    if (e.deltaY < 0) {
      state.zoomLevel = Math.min(state.zoomLevel + 0.05, 2.0);
    } else {
      state.zoomLevel = Math.max(state.zoomLevel - 0.05, 0.5);
    }
    applyZoom();
  }
}, { passive: false });

// ── Boot ──────────────────────────────────────────────────────
init();
