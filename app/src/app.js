const tauri=window.__TAURI__||{};
const invoke=tauri.core?.invoke, convertFileSrc=tauri.core?.convertFileSrc||((p)=>p);
const $=id=>document.getElementById(id), sleep=ms=>new Promise(r=>setTimeout(r,ms));
const state={port:null,tracks:[],plan:null,selected:-1,selectedTrack:-1,history:[],future:[],zoom:1,snap:true,follow:false,projectName:'Untitled Mix',output:null,arrangement:[],loop:false,preview:null,playhead:0,evaluationMode:false};
const api=p=>`http://127.0.0.1:${state.port}${p}`;
async function request(path,options={}){const r=await fetch(api(path),options);if(!r.ok)throw new Error(await r.text());return r.json()}
function toast(message,type=''){const e=document.createElement('div');e.className=`toast ${type}`;e.textContent=message;$('toasts').append(e);setTimeout(()=>e.remove(),4000)}
function status(message,type=''){$('statusText').textContent=message;$('statusDot').className=type}
function esc(s=''){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
function fmt(sec=0){sec=Math.max(0,Number(sec)||0);return `${String(Math.floor(sec/60)).padStart(2,'0')}:${String(Math.floor(sec%60)).padStart(2,'0')}`}
function filename(p=''){return p.split(/[\\/]/).pop()}
function clamp(v,a,b){return Math.max(a,Math.min(b,v))}
function currentTransition(){return state.plan?.selections[state.selected]}

async function init(){
  if(invoke){for(let i=0;i<50;i++){try{state.port=await invoke('get_worker_port');break}catch{await sleep(250)}}}
  if(state.port){try{await request('/health');status('준비됨','ok')}catch{status('오디오 엔진 응답 없음','err')}}else status('UI 미리보기 모드','');
  bind();renderLibrary();
}
function bind(){
  $('importBtn').onclick=$('emptyImport').onclick=()=>$('fileInput').click();$('fileInput').onchange=e=>addFiles([...e.target.files]);
  const dz=$('dropZone');dz.ondragover=e=>{e.preventDefault();dz.classList.add('drag')};dz.ondragleave=()=>dz.classList.remove('drag');dz.ondrop=e=>{e.preventDefault();dz.classList.remove('drag');addFiles([...e.dataTransfer.files])};
  $('ytBtn').onclick=downloadYouTube;$('analyzeBtn').onclick=analyze;$('autoMixBtn').onclick=autoMix;
  $('advancedBtn').onclick=()=>{const show=$('advancedBar').classList.toggle('hidden');$('advancedBtn').setAttribute('aria-expanded',String(!show))};
  $('evaluationMode').onchange=e=>{state.evaluationMode=e.target.checked;$('evaluationPanel').classList.toggle('hidden',!state.evaluationMode);renderInspector()};
  $('zoomIn').onclick=()=>setZoom(state.zoom*1.2);$('zoomOut').onclick=()=>setZoom(state.zoom/1.2);$('followBtn').onclick=()=>toggleFlag('follow');$('snapBtn').onclick=()=>toggleFlag('snap');
  $('undoBtn').onclick=undo;$('redoBtn').onclick=redo;$('saveBtn').onclick=saveProject;$('loadBtn').onclick=loadProject;
  $('transitionType').onchange=e=>editTransition(t=>{t.type=e.target.value;t.name=e.target.selectedOptions[0].textContent;applyTypeDefaults(t)});
  $('durationRange').oninput=$('durationBars').onchange=e=>editBars(+e.target.value);
  $('aExit').onchange=e=>editTime('a_out_time',+e.target.value,e);$('bEntry').onchange=e=>editTime('b_in_time',+e.target.value,e);
  $('curveType').onchange=e=>editTransition(t=>t.curve=e.target.value);bindLiveRange('gainA','gain_a',1);bindLiveRange('gainB','gain_b',1);
  bindAutomationRange('lowA','low_a');bindAutomationRange('lowB','low_b');bindAutomationRange('filterA','filter_a');bindAutomationRange('filterB','filter_b');
  bindLiveRange('filterCutoff','filter_cutoff',400);bindLiveRange('filterResonance','filter_resonance',.7);bindLiveRange('echoFeedback','echo_feedback',.35);bindLiveRange('echoMix','echo_mix',.25);
  $('lockBtn').onclick=()=>editTransition(t=>t.locked=!t.locked);$('loopBtn').onclick=toggleLoop;$('regenerateAlt').onclick=regenerateAlternatives;$('automationBtn').onclick=()=>document.body.classList.toggle('show-automation');
  bindCurveHandle();$('exportBtn').onclick=()=>$('exportModal').classList.remove('hidden');$('exportCancel').onclick=()=>$('exportModal').classList.add('hidden');$('exportConfirm').onclick=exportMix;
  $('playBtn').onclick=togglePlay;$('prevBtn').onclick=()=>seekBy(-5);$('nextBtn').onclick=()=>seekBy(5);$('seekBar').oninput=e=>seekMix(totalDuration()*e.target.value/1000);
  $('audio').ontimeupdate=updateTransport;$('audio').onended=()=>setPlaying(false);
  window.onkeydown=handleShortcut;
}
function toggleFlag(name){state[name]=!state[name];$(`${name}Btn`).setAttribute('aria-pressed',String(state[name]));if(name==='snap')$('snapSelect').value=state.snap?'phrase':'none'}
function handleShortcut(e){if(/INPUT|SELECT|TEXTAREA/.test(e.target.tagName))return;const mod=e.ctrlKey||e.metaKey,k=e.key.toLowerCase();if(mod&&k==='s'){e.preventDefault();saveProject()}else if(mod&&e.shiftKey&&k==='z'){e.preventDefault();redo()}else if(mod&&k==='z'){e.preventDefault();undo()}else if(e.code==='Space'){e.preventDefault();togglePlay()}else if(k==='l'){toggleLoop()}else if(k==='s'){toggleFlag('snap')}else if(e.key==='ArrowLeft'){seekBy(e.shiftKey?-15:-3)}else if(e.key==='ArrowRight'){seekBy(e.shiftKey?15:3)}else if(e.key==='+'||e.key==='='){setZoom(state.zoom*1.2)}else if(e.key==='-'){setZoom(state.zoom/1.2)}else if(e.key==='Delete'&&state.selected>=0){toast('전환은 트랙 사이에 필요합니다. 다른 Alternative를 선택하세요.') }}
function setZoom(v){state.zoom=clamp(v,.35,3);$('zoomLabel').textContent=Math.round(state.zoom*100)+'%';renderTimeline()}
function addFiles(files){for(const f of files)if(!state.tracks.some(t=>t.filename===f.name))state.tracks.push({filename:f.name,file:f,path:null,analysis:null});renderLibrary()}
function renderLibrary(){
  $('trackCount').textContent=`${state.tracks.length}곡`;$('trackList').innerHTML=state.tracks.map((t,i)=>`<div class="track-row ${i===state.selectedTrack?'selected':''}" data-track="${i}"><span class="track-index">${i+1}</span><div><div class="track-title">${esc(t.filename)}</div><div class="track-meta">${t.analysis?`<span><i class="confidence ${(t.analysis.bpm_confidence||0)<.55?'low':''}"></i> ${Math.round(t.analysis.bpm)} BPM</span><span>${esc(t.analysis.key||'Unknown')}</span><span>${fmt(t.analysis.duration)}</span>`:'<span>분석 대기</span>'}</div></div><button class="remove-track" data-remove="${i}" aria-label="제거">×</button></div>`).join('');
  document.querySelectorAll('[data-track]').forEach(r=>r.onclick=e=>{if(e.target.closest('[data-remove]'))return;state.selectedTrack=+r.dataset.track;renderLibrary();quickAudition(state.selectedTrack)});document.querySelectorAll('[data-remove]').forEach(b=>b.onclick=()=>{stopPreview();state.tracks.splice(+b.dataset.remove,1);state.plan=null;renderLibrary();renderTimeline()});
  $('analyzeBtn').disabled=!state.tracks.length;$('autoMixBtn').disabled=state.tracks.length<2;$('emptyState').classList.toggle('hidden',!!state.plan);$('timeline').classList.toggle('hidden',!state.plan);
}
async function upload(){const pending=state.tracks.filter(t=>t.file&&!t.path);if(!pending.length)return;const form=new FormData();pending.forEach(t=>form.append('files',t.file,t.filename));const r=await fetch(api('/upload'),{method:'POST',body:form});if(!r.ok)throw new Error(await r.text());const d=await r.json();for(const f of d.files||[]){const t=state.tracks.find(x=>x.filename===f.filename);if(t)t.path=f.path}}
async function downloadYouTube(){if(!state.port)return toast('데스크톱 앱에서 사용할 수 있습니다.','error');const url=$('ytUrl').value.trim();if(!url)return toast('YouTube URL을 입력하세요.','error');$('ytBtn').disabled=true;status('YouTube 오디오 가져오는 중');try{const d=await request('/download/youtube',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({url,max_tracks:50})});for(const f of d.files||[])if(!state.tracks.some(t=>t.path===f.path))state.tracks.push({filename:f.filename,path:f.path,file:null,analysis:null});renderLibrary();toast(`${d.files?.length||0}곡을 가져왔습니다.`,'ok');status('준비됨','ok')}catch(e){friendlyError('YouTube 가져오기',e)}finally{$('ytBtn').disabled=false}}
async function analyze(){if(!state.port)return toast('분석은 데스크톱 앱에서 실행하세요.','error');await upload();const pending=state.tracks.filter(t=>t.path&&!t.analysis);if(!pending.length)return toast('모든 곡이 이미 분석되었습니다.','ok');$('analyzeBtn').disabled=true;status(`분석 중 0 / ${pending.length}`);try{const d=await request('/analyze',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({filepaths:pending.map(t=>t.path)})});for(const a of d.results||[]){if(!a?.filepath)continue;const t=state.tracks.find(x=>x.path===a.filepath);if(t)t.analysis=a}renderLibrary();if(d.errors?.length)toast(`${d.errors.length}곡을 분석하지 못했습니다. Details: ${d.errors[0]}`,'error');else toast('분석이 완료되었습니다.','ok');status('분석 완료','ok')}catch(e){friendlyError('분석',e)}finally{$('analyzeBtn').disabled=false}}
function mixSettings(){return{style:$('styleSelect').value,search_depth:$('depthSelect').value,preserve_order:$('preserveOrder').checked,transition_length:+$('transitionLength').value,phrase_priority:+$('phrasePriority').value,harmonic_priority:+$('harmonicPriority').value,vocal_overlap:.7,tempo_flexibility:.5,energy_movement:.5}}
async function autoMix(){if(state.tracks.some(t=>!t.analysis))await analyze();const tracks=state.tracks.filter(t=>t.analysis).map(t=>t.analysis);if(tracks.length<2)return toast('분석된 곡이 2개 이상 필요합니다.','error');status('세트 순서와 전환을 계획하는 중');$('autoMixBtn').disabled=true;try{const locked=state.plan?.selections||[];const d=await request('/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tracks,settings:mixSettings(),locked})});checkpoint();state.plan=d.plan;state.selected=0;state.output=null;renderLibrary();renderTimeline();selectTransition(0);$('exportBtn').disabled=false;$('mixSummary').textContent=`${state.plan.sorted_tracks.length}곡 · ${state.plan.evaluated.toLocaleString()} candidates`;toast(locked.some(t=>t.locked)?'잠긴 전환을 유지하고 나머지를 다시 구성했습니다.':'Auto Mix가 준비되었습니다.','ok');status('믹스 준비됨','ok')}catch(e){friendlyError('Auto Mix',e)}finally{$('autoMixBtn').disabled=false}}

function buildArrangement(){if(!state.plan)return[];let cursor=0;return state.plan.sorted_tracks.map((a,i)=>{const sourceStart=i?state.plan.selections[i-1].b_in_time:0;const sourceEnd=state.plan.selections[i]?Math.min(a.duration,state.plan.selections[i].a_out_time+state.plan.selections[i].duration):a.duration;const item={analysis:a,index:i,lane:i%2,start:cursor,sourceStart,sourceEnd,duration:Math.max(1,sourceEnd-sourceStart)};cursor+=item.duration-(state.plan.selections[i]?.duration||0);return item})}
function totalDuration(){const a=state.arrangement;return a.length?Math.max(...a.map(x=>x.start+x.duration)):0}
function pixelsPerSecond(){const total=Math.max(1,totalDuration());return clamp((900/total)*state.zoom,1.4,18)}
function renderTimeline(){
  if(!state.plan){$('timeline').classList.add('hidden');$('emptyState').classList.remove('hidden');return}state.arrangement=buildArrangement();const pps=pixelsPerSecond(),total=totalDuration(),width=Math.max(950,total*pps+100),stage=$('timelineStage');stage.style.width=`${width}px`;stage.style.height=`${28+state.arrangement.length%2*0+2*92+60}px`;
  const step=total>900?300:total>300?120:60;$('timeRuler').innerHTML=Array.from({length:Math.ceil(total/step)+1},(_,i)=>`<span class="tick" style="left:${42+i*step*pps}px">${fmt(i*step)}</span>`).join('');
  let html='<div class="lane-label" style="top:28px">A</div><div class="lane-label" style="top:120px">B</div>';
  for(const item of state.arrangement){const a=item.analysis,left=42+item.start*pps,w=Math.max(30,item.duration*pps),top=38+item.lane*92,wave=waveformHTML(a.waveform||[],w);const sections=(a.segments||[]).map(s=>{const l=clamp((s.start-item.sourceStart)/item.duration,0,1)*100,sw=clamp((s.end-s.start)/item.duration,0,1)*100;return `<i class="section-band" title="${esc(s.label)}" style="left:${l}%;width:${sw}%"></i>`}).join('');html+=`<div class="clip ${item.index===state.selectedTrack?'selected':''}" data-clip="${item.index}" style="left:${left}px;top:${top}px;width:${w}px"><div class="clip-header"><b>${item.index+1}. ${esc(filename(a.filepath))}</b><span>${Math.round(a.bpm)} BPM · ${esc(a.key||'?')}</span></div><div class="waveform">${wave}</div>${sections}</div>`}
  for(let i=0;i<state.plan.selections.length;i++){const t=state.plan.selections[i],next=state.arrangement[i+1],left=42+next.start*pps,w=Math.max(16,t.duration*pps),top=66,paths=automationPaths(t);html+=`<div class="transition-zone ${i===state.selected?'selected':''} ${t.locked?'locked':''}" data-transition="${i}" style="left:${left}px;top:${top}px;width:${w}px"><b class="handle left" data-drag="left"></b><span class="label">${t.bars} bars<br>${esc(t.type.replaceAll('_',' '))}</span><svg class="automation-overlay" viewBox="0 0 100 50" preserveAspectRatio="none"><path stroke="#f0a45d" d="${paths.a}"/><path stroke="#69adf4" d="${paths.b}"/></svg><b class="handle right" data-drag="right"></b></div>`}
  $('timelineCanvas').innerHTML=html;document.querySelectorAll('[data-transition]').forEach(el=>{el.onclick=()=>selectTransition(+el.dataset.transition);el.onpointerdown=startTransitionDrag});document.querySelectorAll('[data-clip]').forEach(el=>el.onclick=e=>{if(e.target.closest('.cue'))return;state.selectedTrack=+el.dataset.clip;state.selected=-1;renderLibrary();renderTimeline();showOverview()});
  $('timelineStage').onpointerdown=e=>{if(e.target.closest('.clip,.transition-zone,.timeline-tools'))return;const x=e.clientX-$('timelineStage').getBoundingClientRect().left-42;seekMix(clamp(x/pps,0,total))};updatePlayhead();
}
function waveformHTML(data,width){const count=clamp(Math.floor(width/3),30,500),step=Math.max(1,Math.floor(data.length/count));return data.filter((_,i)=>i%step===0).slice(0,count).map(v=>`<i style="height:${Math.max(2,v*44)}px"></i>`).join('')}
function automationPaths(t){const amount=t.curve_amount??.5,p=curvePower(t.curve,amount);const pts=Array.from({length:21},(_,i)=>{const x=i/20,y=fadeAt(x,p,t.curve);return{x:x*100,y}});return{a:pts.map((q,i)=>`${i?'L':'M'}${q.x.toFixed(1)} ${(5+q.y*40).toFixed(1)}`).join(' '),b:pts.map((q,i)=>`${i?'L':'M'}${q.x.toFixed(1)} ${(45-q.y*40).toFixed(1)}`).join(' ')}}
function curvePower(type,amount=.5){if(type==='sharp')return .45;if(type==='smooth')return 2.1;if(type==='custom')return .35+amount*3;return 1}
function fadeAt(x,p,type){if(type==='equal_power')return Math.cos(x*Math.PI/2);return Math.pow(1-x,p)}
function startTransitionDrag(e){const el=e.currentTarget,i=+el.dataset.transition,side=e.target.dataset.drag;if(!side)return;e.preventDefault();state.selected=i;renderInspector();checkpoint();const startX=e.clientX,orig=structuredClone(currentTransition()),pps=pixelsPerSecond(),startLeft=parseFloat(el.style.left),startWidth=parseFloat(el.style.width);el.setPointerCapture(e.pointerId);el.onpointermove=ev=>{const scale=ev.shiftKey?.2:1,delta=(ev.clientX-startX)/pps*scale,t=currentTransition();if(side==='left'){const val=snapTime(orig.a_out_time+delta,state.plan.sorted_tracks[i],ev.altKey);t.a_out_time=clamp(val,0,state.plan.sorted_tracks[i].duration-t.duration);el.style.left=`${startLeft+delta*pps}px`}else{const dur=Math.max(60/(t.target_bpm||120),orig.duration+delta);t.duration=dur;t.bars=Math.max(1,Math.round(dur*(t.target_bpm||120)/240));el.style.width=`${Math.max(16,startWidth+delta*pps)}px`}t.manual=true;showSnap(t.a_out_time,side==='left'?'A OUT':'Length');renderInspector();applyPreviewNow()};el.onpointerup=()=>{el.onpointermove=null;$('snapGuide').classList.add('hidden');renderTimeline()}}
function showSnap(time,label){const pps=pixelsPerSecond(),item=state.arrangement[state.selected];$('snapGuide').style.left=`${42+((item?.start||0)+time-(item?.sourceStart||0))*pps}px`;$('snapGuide').querySelector('span').textContent=label;$('snapGuide').classList.remove('hidden')}
function snapTime(v,track,bypass=false){const mode=state.snap&&!bypass?$('snapSelect').value:'none',arr=mode==='phrase'?track.phrases:mode==='bar'?track.downbeats:mode==='beat'?track.beat_times:null;if(!arr?.length)return v;return arr.reduce((a,b)=>Math.abs(b-v)<Math.abs(a-v)?b:a,arr[0])}

function selectTransition(i){if(!state.plan?.selections[i])return;state.selected=i;state.selectedTrack=-1;renderLibrary();renderTimeline();$('inspectorEmpty').classList.add('hidden');$('inspector').classList.remove('hidden');$('inspectorKind').textContent=`Transition ${i+1}`;renderInspector()}
function showOverview(){$('inspectorEmpty').classList.remove('hidden');$('inspector').classList.add('hidden');$('inspectorKind').textContent=state.selectedTrack>=0?'Track':'Mix overview'}
function renderInspector(){const t=currentTransition();if(!t)return;const defaults={curve:'equal_power',curve_amount:.5,gain_a:1,gain_b:1,filter_cutoff:400,filter_resonance:.7,echo_feedback:.35,echo_mix:.25};for(const[k,v]of Object.entries(defaults))if(t[k]===undefined)t[k]=v;$('transitionName').textContent=t.name||t.type;$('transitionType').value=t.type;$('durationRange').value=t.bars||8;$('durationBars').value=t.bars||8;$('aExit').value=(t.a_out_time||0).toFixed(2);$('bEntry').value=(t.b_in_time||0).toFixed(2);$('curveType').value=t.curve;$('gainA').value=t.gain_a;$('gainB').value=t.gain_b;$('filterCutoff').value=t.filter_cutoff;$('filterResonance').value=t.filter_resonance;$('echoFeedback').value=t.echo_feedback;$('echoMix').value=t.echo_mix;fillAutomationInputs(t);$('lockBtn').textContent=t.locked?'🔒 Locked':'🔓 Lock';$('lockBtn').setAttribute('aria-pressed',String(!!t.locked));$('loopBtn').setAttribute('aria-pressed',String(state.loop));$('filterParams').classList.toggle('hidden',t.type!=='filter_fade'&&t.type!=='drop');$('echoParams').classList.toggle('hidden',t.type!=='echo_out');drawCurve();renderAlternatives();if(state.evaluationMode){const s=t.score||{};$('scoreGrid').innerHTML=['Beat','Downbeat','Phrase','Harmony','Tempo','Energy','Vocal'].map(k=>`<span>${k}</span><b>${Math.round(s[k.toLowerCase()]||0)}</b>`).join('')}}
function fillAutomationInputs(t){const last=(arr,fallback)=>arr?.length?arr[arr.length-1].value:fallback;$('lowA').value=last(t.automation?.low_a,1);$('lowB').value=last(t.automation?.low_b,1);$('filterA').value=last(t.automation?.filter_a,1);$('filterB').value=last(t.automation?.filter_b,1)}
function renderAlternatives(){const t=currentTransition(),alts=state.plan.alternatives?.[state.selected]||[t];$('alternatives').innerHTML=alts.map((a,i)=>`<button class="alternative ${a.id===t.id?'active':''}" data-alt="${i}"><span>${i===0?'Recommended':`Alternative ${String.fromCharCode(64+i)}`}</span><b>${state.evaluationMode?Math.round(a.score?.total||0):a.bars+' bars'}</b><span>Preview</span></button>`).join('');document.querySelectorAll('[data-alt]').forEach(b=>b.onclick=()=>applyAlternative(alts[+b.dataset.alt]))}
function drawCurve(){const t=currentTransition(),p=curvePower(t.curve,t.curve_amount),pts=Array.from({length:41},(_,i)=>{const x=i/40,y=fadeAt(x,p,t.curve);return{x:x*260,y}});$('curveA').setAttribute('d',pts.map((q,i)=>`${i?'L':'M'}${q.x} ${10+q.y*80}`).join(' '));$('curveB').setAttribute('d',pts.map((q,i)=>`${i?'L':'M'}${q.x} ${90-q.y*80}`).join(' '));$('curveHandle').setAttribute('cy',String(15+(t.curve_amount??.5)*70))}
function bindCurveHandle(){const svg=$('curveSvg'),h=$('curveHandle');h.onpointerdown=e=>{checkpoint();h.setPointerCapture(e.pointerId);const start=e.clientY,orig=currentTransition().curve_amount??.5;h.onpointermove=ev=>{const scale=ev.shiftKey?.2:1;currentTransition().curve='custom';currentTransition().curve_amount=clamp(orig+(ev.clientY-start)/70*scale,0,1);currentTransition().manual=true;$('curveType').value='custom';drawCurve();renderTimeline();applyPreviewNow()};h.onpointerup=()=>h.onpointermove=null}}
function editTransition(fn){if(!currentTransition())return;checkpoint();fn(currentTransition());currentTransition().manual=true;renderTimeline();renderInspector();applyPreviewNow()}
function editBars(bars){if(!bars||bars<1)return;editTransition(t=>{t.bars=bars;t.duration=bars*4*60/(t.target_bpm||120);t.automation=scaleAutomation(t.automation,t.duration)})}
function editTime(key,v,e){if(!Number.isFinite(v))return;const idx=state.selected+(key==='b_in_time'?1:0),track=state.plan.sorted_tracks[idx];editTransition(t=>t[key]=clamp(snapTime(v,track,e.altKey),0,track.duration))}
function applyTypeDefaults(t){t.automation=t.automation||{};if(t.type==='bass_swap'){t.automation.low_a=[{time:0,value:1},{time:t.duration*.52,value:0}];t.automation.low_b=[{time:0,value:0},{time:t.duration*.52,value:1}]}else if(t.type==='filter_fade'){t.automation.filter_a=[{time:0,value:1},{time:t.duration,value:0}];t.automation.filter_b=[{time:0,value:.2},{time:t.duration,value:1}]}}
function scaleAutomation(a={},dur){const out=structuredClone(a);for(const arr of Object.values(out))if(Array.isArray(arr)&&arr.length){const old=Math.max(...arr.map(p=>p.time),1);arr.forEach(p=>p.time=p.time/old*dur)}return out}
function bindLiveRange(id,key,fallback){$(id).onpointerdown=checkpoint;$(id).oninput=e=>{const t=currentTransition();if(!t)return;t[key]=Number.isFinite(+e.target.value)?+e.target.value:fallback;t.manual=true;applyPreviewNow();if(id.startsWith('gain'))renderTimeline()}}
function bindAutomationRange(id,key){$(id).onpointerdown=checkpoint;$(id).oninput=e=>{const t=currentTransition();if(!t)return;t.automation=t.automation||{};t.automation[key]=[{time:0,value:+e.target.value},{time:t.duration,value:+e.target.value}];t.manual=true;applyPreviewNow();renderTimeline()}}
function checkpoint(){if(!state.plan)return;const snap=JSON.stringify(state.plan.selections);if(state.history.at(-1)!==snap)state.history.push(snap);if(state.history.length>80)state.history.shift();state.future=[];updateUndo()}
function undo(){if(!state.history.length||!state.plan)return;state.future.push(JSON.stringify(state.plan.selections));state.plan.selections=JSON.parse(state.history.pop());renderTimeline();if(state.selected>=0)renderInspector();applyPreviewNow();updateUndo()}
function redo(){if(!state.future.length||!state.plan)return;state.history.push(JSON.stringify(state.plan.selections));state.plan.selections=JSON.parse(state.future.pop());renderTimeline();if(state.selected>=0)renderInspector();applyPreviewNow();updateUndo()}
function updateUndo(){$('undoBtn').disabled=!state.history.length;$('redoBtn').disabled=!state.future.length}
function applyAlternative(a){checkpoint();state.plan.selections[state.selected]=structuredClone(a);renderTimeline();renderInspector();if(state.loop)startLivePreview()}
async function regenerateAlternatives(){if(!state.port)return;const d=await request('/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tracks:state.plan.sorted_tracks,settings:{...mixSettings(),preserve_order:true}})});state.plan.alternatives[state.selected]=d.plan.alternatives[state.selected];renderInspector();toast('새 전환 후보를 준비했습니다.','ok')}

function sourceFor(path){return convertFileSrc(path)}
function setupMedia(path){const el=new Audio(sourceFor(path));el.preload='auto';const ctx=state.preview?.ctx||new AudioContext(),src=ctx.createMediaElementSource(el),low=ctx.createBiquadFilter(),filter=ctx.createBiquadFilter(),gain=ctx.createGain();low.type='lowshelf';low.frequency.value=220;filter.type='lowpass';src.connect(low).connect(filter).connect(gain).connect(ctx.destination);return{el,src,low,filter,gain}}
async function startLivePreview(){
  stopPreview();const i=state.selected,t=currentTransition();if(i<0||!t||!state.port)return;
  $('audio').pause();
  const p={el:null,timer:0,controller:new AbortController(),index:i};state.preview=p;
  status('출력과 동일한 전환 미리듣기 준비 중');setPlaying(true);
  try{
    const d=await request('/render/preview',{method:'POST',headers:{'Content-Type':'application/json'},signal:p.controller.signal,body:JSON.stringify({track_a_path:state.plan.sorted_tracks[i].filepath,track_b_path:state.plan.sorted_tracks[i+1].filepath,spec:structuredClone(t)})});
    if(state.preview!==p)return;if(d.error)throw new Error(d.error);
    const el=new Audio(sourceFor(d.output_path));p.el=el;el.loop=state.loop;
    p.origin=(state.arrangement[i+1]?.start||0)-Math.min(8,t.a_out_time);
    el.ontimeupdate=()=>{if(state.preview!==p)return;state.playhead=Math.max(0,p.origin+el.currentTime);updatePlayhead();updateTransport()};
    el.onended=()=>{if(state.preview===p)stopPreview()};
    el.onerror=()=>{if(state.preview===p){stopPreview();toast('미리듣기 오디오를 읽을 수 없습니다.','error')}};
    await el.play();if(state.preview!==p){el.pause();return}status('전환 미리듣기 재생 중','ok');
  }catch(e){if(state.preview!==p)return;stopPreview();if(e.name!=='AbortError')friendlyError('전환 미리듣기',e)}
}
function animatePreview(){const p=state.preview;if(!p)return;const t=currentTransition(),elapsed=(performance.now()-p.start)/1000,x=clamp((elapsed-p.margin)/Math.max(.01,t.duration),0,1);applyAudioNodes(p.a,t,fadeAt(x,curvePower(t.curve,t.curve_amount),t.curve)*(t.gain_a||1),'a',x);applyAudioNodes(p.b,t,(1-fadeAt(x,curvePower(t.curve,t.curve_amount),t.curve))*(t.gain_b||1),'b',x);state.playhead=(state.arrangement[state.selected]?.start||0)+Math.max(0,elapsed);updatePlayhead();updateTransport();if(elapsed>=p.margin+t.duration+4&&state.loop){startLivePreview();return}p.raf=requestAnimationFrame(animatePreview)}
function applyAudioNodes(node,t,gain,side,x){if(!node)return;const arr=t.automation?.[`low_${side}`],low=interpolate(arr,x*t.duration,1);node.low.gain.value=(low-1)*18;const farr=t.automation?.[`filter_${side}`],fv=interpolate(farr,x*t.duration,1);node.filter.frequency.value=400*Math.pow(40,clamp(fv,0,1));node.filter.Q.value=t.filter_resonance||.7;node.gain.gain.value=clamp(gain,0,1.5)}
function interpolate(arr,time,fallback){if(!arr?.length)return fallback;if(time<=arr[0].time)return arr[0].value;for(let i=1;i<arr.length;i++){if(time<=arr[i].time){const p=(time-arr[i-1].time)/Math.max(.001,arr[i].time-arr[i-1].time);return arr[i-1].value+(arr[i].value-arr[i-1].value)*p}}return arr.at(-1).value}
function applyPreviewNow(){state.output=null;const p=state.preview;if(!p||!currentTransition())return;clearTimeout(p.timer);p.timer=setTimeout(()=>{if(state.preview===p)startLivePreview()},350)}
function stopPreview(){const p=state.preview;if(!p)return;state.preview=null;clearTimeout(p.timer);p.controller?.abort();if(p.el){p.el.pause();p.el.removeAttribute('src');p.el.load()}setPlaying(false)}
function toggleLoop(){if(state.selected<0)return toast('먼저 전환을 선택하세요.');state.loop=!state.loop;$('loopBtn').setAttribute('aria-pressed',String(state.loop));if(state.loop)startLivePreview();else stopPreview()}
let listeningRequest=null;
async function togglePlay(){
  if(state.preview){stopPreview();return}
  if(listeningRequest){listeningRequest.abort();listeningRequest=null;setPlaying(false);status('재생 준비 취소');return}
  const audio=$('audio');
  if(state.plan && !state.output){
    const controller=new AbortController();listeningRequest=controller;setPlaying(true);status('전체 믹스 청취 준비 중');
    const snapshot=JSON.stringify({playlist:playlist(),transitions:state.plan.selections});
    try{
      const result=await request('/render/listen',{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:snapshot});
      if(listeningRequest!==controller)return;
      if(snapshot!==JSON.stringify({playlist:playlist(),transitions:state.plan.selections})){status('믹스가 변경되었습니다. 다시 재생하세요.');return}
      state.output=result.output_path;audio.src=sourceFor(state.output);await audio.play();status('전체 믹스 재생 중','ok');
    }catch(e){if(e.name!=='AbortError')friendlyError('믹스 재생',e)}
    finally{if(listeningRequest===controller){listeningRequest=null;setPlaying(!audio.paused)}}
    return;
  }
  if(!audio.src)return toast('음악을 추가하고 Auto Mix를 실행하세요.');
  try{if(audio.paused)await audio.play();else audio.pause();setPlaying(!audio.paused)}catch(e){friendlyError('재생',e)}
}
function setPlaying(on){$('playBtn').textContent=on?'Ⅱ':'▶'}
function quickAudition(i){if(!state.plan&&state.tracks[i]?.path){const a=$('audio');a.src=sourceFor(state.tracks[i].path);a.play().catch(()=>{});setPlaying(true)}}
function seekBy(delta){if(state.preview){const el=state.preview.el;if(el)el.currentTime=clamp(el.currentTime+delta,0,Number.isFinite(el.duration)?el.duration:0)}else if($('audio').src)$('audio').currentTime=Math.max(0,$('audio').currentTime+delta);else seekMix(state.playhead+delta)}
function seekMix(sec){state.playhead=clamp(sec,0,totalDuration());updatePlayhead();if(state.output&&$('audio').duration)$('audio').currentTime=state.playhead}
function updatePlayhead(){const pps=pixelsPerSecond();$('playhead').style.transform=`translateX(${42+state.playhead*pps}px)`;if(state.follow&&state.preview){const tl=$('timeline'),x=42+state.playhead*pps;if(x-tl.scrollLeft>tl.clientWidth*.8)tl.scrollLeft=x-tl.clientWidth*.4}}
function updateTransport(){const a=$('audio'),cur=state.preview?state.playhead:(a.currentTime||state.playhead),dur=state.output?(a.duration||totalDuration()):totalDuration();$('timeDisplay').textContent=`${fmt(cur)} / ${fmt(dur)}`;$('seekBar').value=dur?cur/dur*1000:0}

function playlist(){return state.plan.sorted_tracks.map((t,i)=>({filepath:t.filepath,filename:filename(t.filepath),duration:t.duration,bpm:t.bpm,loudness_db:t.loudness_db,play_start:i?state.plan.selections[i-1].b_in_time:0,play_end:state.plan.selections[i]?Math.min(t.duration,state.plan.selections[i].a_out_time+state.plan.selections[i].duration):t.duration}))}
async function exportMix(){if(!state.port)return toast('Export는 데스크톱 앱에서 실행하세요.','error');const format=$('exportFormat').value,dir=await invoke('get_output_dir'),safe=($('exportFilename').value||'DJBOT Mix').replace(/[<>:"/\\|?*]/g,'_'),path=`${dir}\\${safe}.${format}`;$('renderProgress').classList.remove('hidden');$('exportConfirm').disabled=true;status('고품질 믹스 렌더링 중');try{const d=await request('/render/mix',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({playlist:playlist(),transitions:state.plan.selections,output_path:path,format})});if(d.error)throw new Error(d.error);const out=d.mp3_path||d.wav_path;state.output=out;$('audio').src=sourceFor(out);$('exportModal').classList.add('hidden');toast(`완료: ${filename(out)}`,'ok');status('Export 완료','ok');try{await tauri.opener.revealItemInDir(out)}catch{}}catch(e){friendlyError('Export',e)}finally{$('renderProgress').classList.add('hidden');$('exportConfirm').disabled=false}}
function projectDocument(){state.arrangement=buildArrangement();const materials=state.plan?state.plan.sorted_tracks.map((a,i)=>({id:`material-${i}`,kind:'track',source_path:a.filepath,analysis:a})):state.tracks.filter(t=>t.analysis).map((t,i)=>({id:`material-${i}`,kind:'track',source_path:t.path,analysis:t.analysis}));const clips=state.arrangement.map(x=>({id:`clip-${x.index}`,material_id:`material-${x.index}`,lane:x.lane,timeline_start:x.start,source_start:x.sourceStart,source_end:x.sourceEnd,gain:1,muted:false}));return{schema_version:2,name:state.projectName,materials,clips,transitions:state.plan?.selections||[],settings:state.plan?.settings||mixSettings(),analysis_overrides:[],evaluation_mode:state.evaluationMode}}
async function saveProject(){if(!state.port)return toast('프로젝트 저장은 데스크톱 앱에서 실행하세요.');if(!state.tracks.length&&!state.plan)return toast('저장할 프로젝트가 없습니다.','error');if(state.projectName==='Untitled Mix'){const n=prompt('프로젝트 이름','My Mix');if(!n)return;state.projectName=n;$('projectName').textContent=n}try{const d=await request('/project/save',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(projectDocument())});toast(`저장됨: ${filename(d.path)}`,'ok')}catch(e){friendlyError('프로젝트 저장',e)}}
async function loadProject(){
  if(!state.port)return;
  try{
    const list=await request('/projects');if(!list.projects.length)return toast('저장된 프로젝트가 없습니다.');
    const name=prompt(`프로젝트 이름:\n${list.projects.join('\n')}`,list.projects[0]);if(!name)return;
    const p=await request('/project/load?name='+encodeURIComponent(name));
    if(!Array.isArray(p.materials)||!Array.isArray(p.transitions))throw new Error('invalid project');
    const tracks=p.materials.filter(m=>m.kind==='track').map(m=>({filename:filename(m.source_path),path:m.source_path,file:null,analysis:m.analysis}));
    if(p.transitions.length&&(tracks.some(t=>!t.analysis)||p.transitions.length!==tracks.length-1))throw new Error('invalid project transitions');
    stopPreview();listeningRequest?.abort();listeningRequest=null;$('audio').pause();$('audio').removeAttribute('src');$('audio').load();
    state.output=null;state.playhead=0;state.history=[];state.future=[];state.loop=false;state.selectedTrack=-1;
    state.projectName=p.name;$('projectName').textContent=p.name;state.tracks=tracks;
    state.plan=p.transitions.length?{sorted_tracks:tracks.map(t=>t.analysis),selections:p.transitions,alternatives:p.transitions.map(t=>[t]),score:{},evaluated:0,settings:p.settings}:null;
    const settings=p.settings||{};
    for(const [id,key] of Object.entries({styleSelect:'style',depthSelect:'search_depth',transitionLength:'transition_length',phrasePriority:'phrase_priority',harmonicPriority:'harmonic_priority'})){if(settings[key]!==undefined)$(id).value=settings[key]}
    $('preserveOrder').checked=!!settings.preserve_order;
    state.evaluationMode=!!p.evaluation_mode;$('evaluationMode').checked=state.evaluationMode;$('evaluationPanel').classList.toggle('hidden',!state.evaluationMode);
    state.selected=state.plan?0:-1;$('exportBtn').disabled=!state.plan;setPlaying(false);updateUndo();renderLibrary();renderTimeline();
    if(state.plan)selectTransition(0);else showOverview();updateTransport();toast('프로젝트를 복원했습니다.','ok');
  }catch(e){friendlyError('프로젝트 열기',e)}
}
function friendlyError(action,e){const raw=String(e?.message||e),hint=/ffmpeg/i.test(raw)?'FFmpeg 번들을 확인하세요.':/yt-dlp/i.test(raw)?'네트워크 연결 후 다시 시도하세요.':/decode|unsupported|invalid/i.test(raw)?'지원되는 오디오 파일인지 확인하세요.':'잠시 후 다시 시도하고 계속되면 Details를 확인하세요.';toast(`${action} 실패 — ${hint} (${raw.slice(0,180)})`,'error');status(`${action} 실패`,'err')}
init();
