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
  $('durationSeconds').onchange=e=>{const duration=+e.target.value;if(Number.isFinite(duration)&&duration>0)editTransition(t=>resizeTransition(t,duration))};
  $('regenerateTransitions').onclick=()=>autoMix('transitions');$('regenerateSelected').onclick=regenerateSelectedTransition;
  $('listenBefore').onclick=()=>{const t=state.beforeEdits?.[state.selected];if(t)startLivePreview(t);else toast('아직 이 전환을 수정하지 않았습니다.')};
  $('listenAfter').onclick=()=>startLivePreview();$('revertTransition').onclick=revertSelectedTransition;
  document.querySelectorAll('[data-bars]').forEach(b=>b.onclick=()=>editBars(+b.dataset.bars));
  for(const [id,key] of [['fadeOutSeconds','fade_out_seconds'],['fadeInSeconds','fade_in_seconds']])$(id).onchange=e=>editTransition(t=>t[key]=+e.target.value);
  bindCueWave('cueWaveA','a_out_time',0);bindCueWave('cueWaveB','b_in_time',1);
  $('focusTransition').onclick=focusTransition;$('fitTimeline').onclick=()=>{state.zoom=1;renderTimeline();$('timeline').scrollLeft=0};$('auditionTransition').onclick=()=>{if(state.selected<0)selectTransition(0);startLivePreview()};
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
function setZoom(v){state.zoom=clamp(v,.35,20);$('zoomLabel').textContent=Math.round(state.zoom*100)+'%';renderTimeline()}
function focusTransition(){if(!state.plan)return;if(state.selected<0)selectTransition(0);setZoom(12);const next=state.arrangement[state.selected+1];$('timeline').scrollLeft=Math.max(0,42+next.start*pixelsPerSecond()-$('timeline').clientWidth*.35)}
function addFiles(files){for(const f of files)if(!state.tracks.some(t=>t.filename===f.name))state.tracks.push({filename:f.name,file:f,path:null,analysis:null});renderLibrary()}
function renderLibrary(){
  $('trackCount').textContent=`${state.tracks.length}곡`;$('trackList').innerHTML=state.tracks.map((t,i)=>`<div class="track-row ${i===state.selectedTrack?'selected':''}" data-track="${i}"><span class="track-index">${i+1}</span><div><div class="track-title">${esc(t.filename)}</div><div class="track-meta">${t.analysis?`<span><i class="confidence ${(t.analysis.bpm_confidence||0)<.55?'low':''}"></i> ${Math.round(t.analysis.bpm)} BPM</span><span>${esc(t.analysis.key||'Unknown')}</span><span>${fmt(t.analysis.duration)}</span>`:'<span>분석 대기</span>'}</div></div><button class="remove-track" data-remove="${i}" aria-label="제거">×</button></div>`).join('');
  document.querySelectorAll('[data-track]').forEach(r=>r.onclick=e=>{if(e.target.closest('[data-remove]'))return;state.selectedTrack=+r.dataset.track;renderLibrary();quickAudition(state.selectedTrack)});document.querySelectorAll('[data-remove]').forEach(b=>b.onclick=()=>{stopPreview();state.tracks.splice(+b.dataset.remove,1);state.plan=null;renderLibrary();renderTimeline()});
  $('analyzeBtn').disabled=!state.tracks.length;$('autoMixBtn').disabled=state.tracks.length<2;$('emptyState').classList.toggle('hidden',!!state.plan);$('timeline').classList.toggle('hidden',!state.plan);
}
async function upload(){const pending=state.tracks.filter(t=>t.file&&!t.path);if(!pending.length)return;const form=new FormData();pending.forEach(t=>form.append('files',t.file,t.filename));const r=await fetch(api('/upload'),{method:'POST',body:form});if(!r.ok)throw new Error(await r.text());const d=await r.json();for(const f of d.files||[]){const t=state.tracks.find(x=>x.filename===f.filename);if(t)t.path=f.path}}
async function downloadYouTube(){if(!state.port)return toast('데스크톱 앱에서 사용할 수 있습니다.','error');const url=$('ytUrl').value.trim();if(!url)return toast('YouTube URL을 입력하세요.','error');$('ytBtn').disabled=true;status('YouTube 오디오 가져오는 중');try{const d=await request('/download/youtube',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({url,max_tracks:50})});for(const f of d.files||[])if(!state.tracks.some(t=>t.path===f.path))state.tracks.push({filename:f.filename,path:f.path,file:null,analysis:null});renderLibrary();toast(`${d.files?.length||0}곡을 가져왔습니다.`,'ok');status('준비됨','ok')}catch(e){friendlyError('YouTube 가져오기',e)}finally{$('ytBtn').disabled=false}}
async function analyze(){if(!state.port)return toast('분석은 데스크톱 앱에서 실행하세요.','error');await upload();const pending=state.tracks.filter(t=>t.path&&!t.analysis);if(!pending.length)return toast('모든 곡이 이미 분석되었습니다.','ok');$('analyzeBtn').disabled=true;status(`분석 중 0 / ${pending.length}`);try{const d=await request('/analyze',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({filepaths:pending.map(t=>t.path)})});for(const a of d.results||[]){if(!a?.filepath)continue;const t=state.tracks.find(x=>x.path===a.filepath);if(t)t.analysis=a}renderLibrary();if(d.errors?.length)toast(`${d.errors.length}곡을 분석하지 못했습니다. Details: ${d.errors[0]}`,'error');else toast('분석이 완료되었습니다.','ok');status('분석 완료','ok')}catch(e){friendlyError('분석',e)}finally{$('analyzeBtn').disabled=false}}
function mixSettings(){return{transition_bars:+$('transitionPreset').value||16,listening_mode:$('listeningMode').value||'balanced',variation:state.generation||0,style:$('styleSelect').value,search_depth:$('depthSelect').value,preserve_order:$('preserveOrder').checked,transition_length:+$('transitionLength').value,phrase_priority:+$('phrasePriority').value,harmonic_priority:+$('harmonicPriority').value,vocal_overlap:.7,tempo_flexibility:.5,energy_movement:.5}}
async function autoMix(scope='all'){
  if(state.planning)return;state.planning=true;$('autoMixBtn').disabled=true;
  try{
    if(state.tracks.some(t=>!t.analysis))await analyze();
    const locked=state.plan?.selections||[],hasLocks=locked.some(t=>t.locked);
    const keepOrder=hasLocks||scope==='transitions';
    const tracks=keepOrder&&state.plan?state.plan.sorted_tracks:state.tracks.filter(t=>t.analysis).map(t=>t.analysis);
    if(tracks.length<2)return toast('분석된 곡이 2개 이상 필요합니다.','error');
    status('새 믹스 구성 중');const settings=mixSettings();if(keepOrder)settings.preserve_order=true;
    const d=await request('/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tracks,settings,locked:hasLocks?locked:[]})});
    if(!d.plan?.selections?.length)throw new Error('전환을 생성하지 못했습니다.');
    stopPreview();listeningRequest?.abort();listeningRequest=null;$('audio').pause();$('audio').removeAttribute('src');$('audio').load();
    state.plan=d.plan;state.beforeEdits={};state.generation=(state.generation||0)+1;state.history=[];state.future=[];state.selected=0;state.output=null;state.playhead=0;state.loop=false;
    renderLibrary();renderTimeline();selectTransition(0);updateUndo();setPlaying(false);updateTransport();
    $('exportBtn').disabled=false;$('autoMixBtn').textContent='↻ 다시 Auto Mix';
    $('mixSummary').textContent=`${tracks.length}곡 · 버전 ${state.generation}`;
    toast(hasLocks?'잠긴 전환과 곡 순서를 유지해 다시 구성했습니다.':'새 믹스가 준비되었습니다. 재생을 누르세요.','ok');status('믹스 준비됨','ok');
  }catch(e){friendlyError('Auto Mix',e)}finally{state.planning=false;$('autoMixBtn').disabled=state.tracks.length<2}
}

function finalSourceEnd(a,incoming){
  const mode=state.plan?.settings?.listening_mode;
  const target=mode==='quick'?Math.min(75,a.duration*.55):mode==='balanced'?Math.min(150,a.duration*.78):a.duration;
  const entryEnd=(incoming?.b_in_time||0)+(incoming?incoming.duration*(incoming.speed_b||1):0);
  return Math.min(a.duration,Math.max(target,entryEnd+Math.min(10,a.duration*.1)));
}
function buildArrangement(){
  if(!state.plan)return[];let cursor=0;
  return state.plan.sorted_tracks.map((a,i)=>{
    const incoming=state.plan.selections[i-1],outgoing=state.plan.selections[i];
    const sourceStart=incoming?.b_in_time||0,entryEnd=sourceStart+(incoming?incoming.duration*(incoming.speed_b||1):0);
    const exitStart=outgoing?outgoing.a_out_time:finalSourceEnd(a,incoming);
    const sourceEnd=outgoing?exitStart+outgoing.duration*(outgoing.speed_a||1):exitStart;
    const duration=(incoming?.duration||0)+Math.max(0,exitStart-entryEnd)+(outgoing?.duration||0);
    const item={analysis:a,index:i,lane:i%2,start:cursor,sourceStart,sourceEnd,duration};
    cursor+=duration-(outgoing?.duration||0);return item;
  });
}
function totalDuration(){const a=state.arrangement;return a.length?Math.max(...a.map(x=>x.start+x.duration)):0}
function pixelsPerSecond(){const total=Math.max(1,totalDuration()),width=Math.max(200,($('timeline').clientWidth||1000)-100);return clamp((width/total)*state.zoom,.15,80)}
function renderTimeline(){
  if(!state.plan){$('timeline').classList.add('hidden');$('emptyState').classList.remove('hidden');return}state.arrangement=buildArrangement();const pps=pixelsPerSecond(),total=totalDuration(),width=Math.max(950,total*pps+100),stage=$('timelineStage');stage.style.width=`${width}px`;stage.style.height=`${460}px`;
  const step=total>900?300:total>300?120:60;$('timeRuler').innerHTML=Array.from({length:Math.ceil(total/step)+1},(_,i)=>`<span class="tick" style="left:${42+i*step*pps}px">${fmt(i*step)}</span>`).join('');
  let html='<div class="lane-label" style="top:28px">A</div><div class="lane-label" style="top:218px">B</div>';
  for(const item of state.arrangement){const a=item.analysis,left=42+item.start*pps,w=Math.max(30,item.duration*pps),top=44+item.lane*190,wave=waveformHTML((a.waveform||[]).slice(Math.floor(item.sourceStart/a.duration*(a.waveform||[]).length),Math.ceil(item.sourceEnd/a.duration*(a.waveform||[]).length)),w);const sections=(a.segments||[]).map(s=>{const l=clamp((s.start-item.sourceStart)/item.duration,0,1)*100,sw=clamp((s.end-s.start)/item.duration,0,1)*100;return `<i class="section-band" title="${esc(s.label)}" style="left:${l}%;width:${sw}%"></i>`}).join('');html+=`<div class="clip ${item.index===state.selectedTrack?'selected':''}" data-clip="${item.index}" style="left:${left}px;top:${top}px;width:${w}px"><div class="clip-header"><b>${item.index+1}. ${esc(filename(a.filepath))}</b><span>${Math.round(a.bpm)} BPM · ${esc(a.key||'?')}</span></div><div class="waveform">${wave}</div>${sections}</div>`}
  for(let i=0;i<state.plan.selections.length;i++){const t=state.plan.selections[i],next=state.arrangement[i+1],left=42+next.start*pps,w=Math.max(16,t.duration*pps),top=168,paths=automationPaths(t);html+=`<div class="transition-zone ${i===state.selected?'selected':''} ${t.locked?'locked':''}" data-transition="${i}" style="left:${left}px;top:${top}px;width:${w}px"><b class="handle left" data-drag="left"></b><span class="label">${t.duration.toFixed(1)}초<br>${esc(t.type.replaceAll('_',' '))}</span><svg class="automation-overlay" viewBox="0 0 100 50" preserveAspectRatio="none"><path stroke="#f0a45d" d="${paths.a}"/><path stroke="#69adf4" d="${paths.b}"/></svg><b class="handle right" data-drag="right"></b></div>`}
  $('timelineCanvas').innerHTML=html;document.querySelectorAll('[data-transition]').forEach(el=>{el.onclick=()=>selectTransition(+el.dataset.transition);el.onpointerdown=startTransitionDrag});document.querySelectorAll('[data-clip]').forEach(el=>el.onclick=e=>{if(e.target.closest('.cue'))return;state.selectedTrack=+el.dataset.clip;state.selected=-1;renderLibrary();renderTimeline();showOverview()});
  $('timelineStage').onpointerdown=e=>{if(e.target.closest('.clip,.transition-zone,.timeline-tools'))return;const x=e.clientX-$('timelineStage').getBoundingClientRect().left-42;seekMix(clamp(x/pps,0,total))};updatePlayhead();
}
function waveformHTML(data,width){const count=clamp(Math.floor(width/3),30,500),step=Math.max(1,Math.floor(data.length/count));return data.filter((_,i)=>i%step===0).slice(0,count).map(v=>`<i style="height:${Math.max(2,v*110)}px"></i>`).join('')}
function automationPaths(t){const pts=Array.from({length:21},(_,i)=>({x:i/20*100,a:transitionEnvelope(t,false,i/20),b:transitionEnvelope(t,true,i/20)}));return{a:pts.map((q,i)=>`${i?'L':'M'}${q.x.toFixed(1)} ${(45-q.a*40).toFixed(1)}`).join(' '),b:pts.map((q,i)=>`${i?'L':'M'}${q.x.toFixed(1)} ${(45-q.b*40).toFixed(1)}`).join(' ')}}
function curvePower(type,amount=.5){if(type==='sharp')return .45;if(type==='smooth')return 2.1;if(type==='custom')return .35+amount*3;return 1}
function fadeAt(x,p,type){if(type==='equal_power')return Math.cos(x*Math.PI/2);return Math.pow(1-x,p)}
function startTransitionDrag(e){
  const el=e.currentTarget,i=+el.dataset.transition,side=e.target.dataset.drag;if(!side)return;
  e.preventDefault();state.selected=i;checkpoint();const original=structuredClone(currentTransition()),startX=e.clientX,pps=pixelsPerSecond();
  el.setPointerCapture(e.pointerId);let error='';
  el.onpointermove=ev=>{
    const delta=(ev.clientX-startX)/pps*(ev.shiftKey?.2:1),draft=structuredClone(original);
    if(side==='left')draft.a_out_time=snapTime(original.a_out_time+delta,state.plan.sorted_tracks[i],ev.altKey);
    else resizeTransition(draft,Math.max(.05,original.duration+delta));
    try{validateTransitionEdit(draft,i);draft.manual=true;state.plan.selections[i]=draft;error='';renderInspector();if(side==='right')el.style.width=`${Math.max(16,draft.duration*pps)}px`}
    catch(err){error=err.message}
  };
  const finish=()=>{el.onpointermove=null;el.onpointerup=null;el.onpointercancel=null;renderTimeline();renderInspector();applyPreviewNow();if(error)toast(error,'error')};
  el.onpointerup=finish;el.onpointercancel=()=>{state.plan.selections[i]=original;finish()};
}
function showSnap(time,label){const pps=pixelsPerSecond(),item=state.arrangement[state.selected];$('snapGuide').style.left=`${42+((item?.start||0)+time-(item?.sourceStart||0))*pps}px`;$('snapGuide').querySelector('span').textContent=label;$('snapGuide').classList.remove('hidden')}
function snapTime(v,track,bypass=false){const mode=state.snap&&!bypass?$('snapSelect').value:'none',arr=mode==='phrase'?track.phrases:mode==='bar'?track.downbeats:mode==='beat'?track.beat_times:null;if(!arr?.length)return v;const nearest=arr.reduce((a,b)=>Math.abs(b-v)<Math.abs(a-v)?b:a,arr[0]),radius=mode==='phrase'?2:Math.min(1,30/(track.bpm||120));return Math.abs(nearest-v)<=radius?nearest:v}

function selectTransition(i){if(!state.plan?.selections[i])return;state.selected=i;state.selectedTrack=-1;renderLibrary();renderTimeline();$('inspectorEmpty').classList.add('hidden');$('inspector').classList.remove('hidden');$('inspectorKind').textContent=`Transition ${i+1}`;renderInspector()}
function showOverview(){$('inspectorEmpty').classList.remove('hidden');$('inspector').classList.add('hidden');$('inspectorKind').textContent=state.selectedTrack>=0?'Track':'Mix overview'}
function renderInspector(){const t=currentTransition();if(!t)return;const defaults={curve:'equal_power',curve_amount:.5,gain_a:1,gain_b:1,filter_cutoff:400,filter_resonance:.7,echo_feedback:.35,echo_mix:.25};for(const[k,v]of Object.entries(defaults))if(t[k]===undefined)t[k]=v;$('transitionName').textContent=t.name||t.type;$('transitionType').value=t.type;$('durationSeconds').value=t.duration.toFixed(2);$('durationRange').value=t.bars||Math.max(1,Math.round(t.duration*(t.target_bpm||120)/240));$('durationBars').value=t.bars||+(t.duration*(t.target_bpm||120)/240).toFixed(2);$('aExit').value=(t.a_out_time||0).toFixed(2);$('bEntry').value=(t.b_in_time||0).toFixed(2);$('curveType').value=t.curve;$('gainA').value=t.gain_a;$('gainB').value=t.gain_b;$('filterCutoff').value=t.filter_cutoff;$('filterResonance').value=t.filter_resonance;$('echoFeedback').value=t.echo_feedback;$('echoMix').value=t.echo_mix;fillAutomationInputs(t);$('lockBtn').textContent=t.locked?'🔒 Locked':'🔓 Lock';$('lockBtn').setAttribute('aria-pressed',String(!!t.locked));$('loopBtn').setAttribute('aria-pressed',String(state.loop));$('filterParams').classList.toggle('hidden',t.type!=='filter_fade'&&t.type!=='drop');$('echoParams').classList.add('hidden');renderEditingDetails();drawCurve();renderAlternatives();if(state.evaluationMode){const s=t.score||{};$('scoreGrid').innerHTML=['Beat','Downbeat','Phrase','Harmony','Tempo','Energy','Vocal'].map(k=>`<span>${k}</span><b>${Math.round(s[k.toLowerCase()]||0)}</b>`).join('')}}
function fillAutomationInputs(t){const last=(arr,fallback)=>arr?.length?arr[arr.length-1].value:fallback;$('lowA').value=last(t.automation?.low_a,1);$('lowB').value=last(t.automation?.low_b,1);$('filterA').value=last(t.automation?.filter_a,1);$('filterB').value=last(t.automation?.filter_b,1)}
function renderAlternatives(){const t=currentTransition(),alts=state.plan.alternatives?.[state.selected]||[t];$('alternatives').innerHTML=alts.map((a,i)=>`<button class="alternative ${a.id===t.id?'active':''}" data-alt="${i}"><span>${i===0?'Recommended':`Alternative ${String.fromCharCode(64+i)}`}</span><b>${state.evaluationMode?Math.round(a.score?.total||0):a.bars+' bars'}</b><span>Preview</span></button>`).join('');document.querySelectorAll('[data-alt]').forEach(b=>b.onclick=()=>applyAlternative(alts[+b.dataset.alt]))}
function drawCurve(){const t=currentTransition(),pts=Array.from({length:41},(_,i)=>({x:i/40*260,a:transitionEnvelope(t,false,i/40),b:transitionEnvelope(t,true,i/40)}));$('curveA').setAttribute('d',pts.map((q,i)=>`${i?'L':'M'}${q.x} ${90-q.a*80}`).join(' '));$('curveB').setAttribute('d',pts.map((q,i)=>`${i?'L':'M'}${q.x} ${90-q.b*80}`).join(' '));$('curveHandle').setAttribute('cy',String(15+(t.curve_amount??.5)*70))}
function bindCurveHandle(){const svg=$('curveSvg'),h=$('curveHandle');h.onpointerdown=e=>{checkpoint();h.setPointerCapture(e.pointerId);const start=e.clientY,orig=currentTransition().curve_amount??.5;h.onpointermove=ev=>{const scale=ev.shiftKey?.2:1;currentTransition().curve='custom';currentTransition().curve_amount=clamp(orig+(ev.clientY-start)/70*scale,0,1);currentTransition().manual=true;$('curveType').value='custom';drawCurve();renderTimeline();applyPreviewNow()};h.onpointerup=()=>h.onpointermove=null}}
function editTransition(fn){
  const old=currentTransition();if(!old)return;const draft=structuredClone(old);
  try{fn(draft);validateTransitionEdit(draft,state.selected)}catch(e){toast(e.message,'error');renderInspector();return}
  state.beforeEdits??={};state.beforeEdits[state.selected]??=structuredClone(old);
  checkpoint();draft.manual=true;state.plan.selections[state.selected]=draft;renderTimeline();renderInspector();applyPreviewNow();
}
function resizeTransition(t,duration){const ratio=duration/t.duration;t.duration=duration;t.bars=Math.round(duration*(t.target_bpm||120)/240);t.automation=scaleAutomation(t.automation,duration);if(t.fade_out_seconds)t.fade_out_seconds*=ratio;if(t.fade_in_seconds)t.fade_in_seconds*=ratio}
function editBars(bars){if(!Number.isInteger(bars)||bars<1)return;editTransition(t=>{resizeTransition(t,bars*240/(t.target_bpm||120));t.bars=bars})}
function editTime(key,v,e){if(!Number.isFinite(v))return;const idx=state.selected+(key==='b_in_time'?1:0),track=state.plan.sorted_tracks[idx];editTransition(t=>t[key]=clamp(snapTime(v,track,e.altKey),0,track.duration))}
function applyTypeDefaults(t){t.automation=t.automation||{};if(t.type==='bass_swap'){t.automation.low_a=[{time:0,value:1},{time:t.duration*.52,value:0}];t.automation.low_b=[{time:0,value:0},{time:t.duration*.52,value:1}]}else if(t.type==='filter_fade'){t.automation.filter_a=[{time:0,value:1},{time:t.duration,value:0}];t.automation.filter_b=[{time:0,value:.2},{time:t.duration,value:1}]}}
function scaleAutomation(a={},dur){const out=structuredClone(a);for(const arr of Object.values(out))if(Array.isArray(arr)&&arr.length){const old=Math.max(...arr.map(p=>p.time),1);arr.forEach(p=>p.time=p.time/old*dur)}return out}
function bindLiveRange(id,key,fallback){$(id).onpointerdown=checkpoint;$(id).oninput=e=>{const t=currentTransition();if(!t)return;t[key]=Number.isFinite(+e.target.value)?+e.target.value:fallback;t.manual=true;applyPreviewNow();if(id.startsWith('gain'))renderTimeline()}}
function bindAutomationRange(id,key){$(id).onpointerdown=checkpoint;$(id).oninput=e=>{const t=currentTransition();if(!t)return;t.automation=t.automation||{};t.automation[key]=[{time:0,value:+e.target.value},{time:t.duration,value:+e.target.value}];t.manual=true;applyPreviewNow();renderTimeline()}}
function checkpoint(){if(!state.plan)return;state.beforeEdits??={};if(state.selected>=0)state.beforeEdits[state.selected]??=structuredClone(currentTransition());const snap=JSON.stringify(state.plan.selections);if(state.history.at(-1)!==snap)state.history.push(snap);if(state.history.length>80)state.history.shift();state.future=[];updateUndo()}
function undo(){if(!state.history.length||!state.plan)return;state.future.push(JSON.stringify(state.plan.selections));state.plan.selections=JSON.parse(state.history.pop());renderTimeline();if(state.selected>=0)renderInspector();applyPreviewNow();updateUndo()}
function redo(){if(!state.future.length||!state.plan)return;state.history.push(JSON.stringify(state.plan.selections));state.plan.selections=JSON.parse(state.future.pop());renderTimeline();if(state.selected>=0)renderInspector();applyPreviewNow();updateUndo()}
function updateUndo(){$('undoBtn').disabled=!state.history.length;$('redoBtn').disabled=!state.future.length}
function applyAlternative(a){editTransition(t=>replaceTransitionContents(t,a))}
async function regenerateAlternatives(){if(!state.port)return;const d=await request('/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tracks:state.plan.sorted_tracks,settings:{...mixSettings(),preserve_order:true}})});state.plan.alternatives[state.selected]=d.plan.alternatives[state.selected];renderInspector();toast('새 전환 후보를 준비했습니다.','ok')}

function sourceFor(path){return convertFileSrc(path)}
function setupMedia(path){const el=new Audio(sourceFor(path));el.preload='auto';const ctx=state.preview?.ctx||new AudioContext(),src=ctx.createMediaElementSource(el),low=ctx.createBiquadFilter(),filter=ctx.createBiquadFilter(),gain=ctx.createGain();low.type='lowshelf';low.frequency.value=220;filter.type='lowpass';src.connect(low).connect(filter).connect(gain).connect(ctx.destination);return{el,src,low,filter,gain}}
async function startLivePreview(override){
  stopPreview();const i=state.selected,t=override||currentTransition();if(i<0||!t||!state.port)return;
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
let editPreviewTimer=0;
function applyPreviewNow(){
  state.output=null;listeningRequest?.abort();listeningRequest=null;
  const playing=!!state.preview||!$('audio').paused;
  $('audio').pause();$('audio').removeAttribute('src');
  if(!playing||!currentTransition())return;
  clearTimeout(editPreviewTimer);editPreviewTimer=setTimeout(()=>startLivePreview(),250);
  status('편집한 전환 미리듣기 업데이트 중');
}
function stopPreview(){clearTimeout(editPreviewTimer);const p=state.preview;if(!p)return;state.preview=null;clearTimeout(p.timer);p.controller?.abort();if(p.el){p.el.pause();p.el.removeAttribute('src');p.el.load()}setPlaying(false)}
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
function updatePlayhead(){const pps=pixelsPerSecond();$('playhead').style.transform=`translateX(${42+state.playhead*pps}px)`;if(state.follow){const tl=$('timeline'),x=42+state.playhead*pps;if(x-tl.scrollLeft>tl.clientWidth*.8||x<tl.scrollLeft)tl.scrollLeft=Math.max(0,x-tl.clientWidth*.4)}}
function updateTransport(){
  const a=$('audio');if(!state.preview&&state.output)state.playhead=a.currentTime||0;
  const cur=state.playhead,dur=totalDuration();
  $('timeDisplay').textContent=`${fmt(cur)} / ${fmt(dur)}`;$('seekBar').value=dur?cur/dur*1000:0;updatePlayhead();
  document.querySelectorAll('[data-clip]').forEach(el=>{const clip=state.arrangement[+el.dataset.clip];el.classList.toggle('playing',!!clip&&cur>=clip.start&&cur<clip.start+clip.duration)});
}

function playlist(){return state.plan.sorted_tracks.map((t,i)=>({filepath:t.filepath,filename:filename(t.filepath),duration:t.duration,bpm:t.bpm,loudness_db:t.loudness_db,play_start:i?state.plan.selections[i-1].b_in_time:0,play_end:state.plan.selections[i]?t.duration:finalSourceEnd(t,state.plan.selections[i-1])}))}
async function exportMix(){if(!state.port)return toast('Export는 데스크톱 앱에서 실행하세요.','error');const format=$('exportFormat').value,dir=await invoke('get_output_dir'),safe=($('exportFilename').value||'DJBOT Mix').replace(/[<>:"/\\|?*]/g,'_'),path=`${dir}\\${safe}.${format}`;$('renderProgress').classList.remove('hidden');$('exportConfirm').disabled=true;status('고품질 믹스 렌더링 중');try{const d=await request('/render/mix',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({playlist:playlist(),transitions:state.plan.selections,output_path:path,format})});if(d.error)throw new Error(d.error);const out=d.mp3_path||d.wav_path;state.output=out;$('audio').src=sourceFor(out);$('exportModal').classList.add('hidden');toast(`완료: ${filename(out)}`,'ok');status('Export 완료','ok');try{await tauri.opener.revealItemInDir(out)}catch{}}catch(e){friendlyError('Export',e)}finally{$('renderProgress').classList.add('hidden');$('exportConfirm').disabled=false}}
function projectDocument(){
  state.arrangement=buildArrangement();
  const previous=state.projectData||{};
  const ordered=state.plan?state.plan.sorted_tracks.map(a=>({path:a.filepath,analysis:a})):[];
  for(const t of state.tracks)if(!ordered.some(x=>x.path===t.path))ordered.push(t);
  if(ordered.some(t=>!t.path))throw new Error('음악 파일을 먼저 가져와야 합니다.');
  const used=new Set((previous.materials||[]).map(m=>m.id));let serial=0;
  const materials=ordered.map(t=>{
    const old=previous.materials?.find(m=>m.kind==='track'&&m.source_path===t.path);
    let id=old?.id;if(!id){do{id=`material-${serial++}`}while(used.has(id));used.add(id)}
    return {...old,id,kind:'track',source_path:t.path,analysis:t.analysis||undefined};
  });
  const clips=state.arrangement.map(x=>({id:`clip-${x.index}`,material_id:materials[x.index].id,lane:x.lane,timeline_start:x.start,source_start:x.sourceStart,source_end:x.sourceEnd,gain:1,muted:false}));
  return{schema_version:2,id:previous.id,created_at:previous.created_at,name:state.projectName,materials,clips,transitions:state.plan?.selections||[],settings:state.plan?.settings||mixSettings(),analysis_overrides:previous.analysis_overrides||[],evaluation_mode:state.evaluationMode};
}
async function saveProject(){
  if(!state.port)return toast('프로젝트 저장은 데스크톱 앱에서 실행하세요.');
  if(!state.tracks.length&&!state.plan)return toast('저장할 프로젝트가 없습니다.','error');
  if(state.saving)return;
  if(state.projectName==='Untitled Mix'){const n=prompt('프로젝트 이름','My Mix');if(!n)return;state.projectName=n;$('projectName').textContent=n}
  state.saving=true;
  try{
    await upload();
    const d=await request('/project/save',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(projectDocument())});
    state.projectData=d.project;
    toast(`저장됨: ${filename(d.path)}`,'ok');
  }catch(e){friendlyError('프로젝트 저장',e)}finally{state.saving=false}
}
async function loadProject(){
  if(!state.port)return;
  try{
    const list=await request('/projects');if(!list.projects.length)return toast('저장된 프로젝트가 없습니다.');
    const name=prompt(`프로젝트 이름:\n${list.projects.join('\n')}`,list.projects[0]);if(!name)return;
    const p=await request('/project/load?name='+encodeURIComponent(name));
    if(!Array.isArray(p.materials)||!Array.isArray(p.transitions))throw new Error('invalid project');
    const tracks=p.materials.filter(m=>m.kind==='track').map(m=>({filename:filename(m.source_path),path:m.source_path,file:null,analysis:m.analysis}));
    const planTracks=p.clips?.length?p.clips.map(c=>tracks.find(t=>t.path===p.materials.find(m=>m.id===c.material_id)?.source_path)):tracks;
    if(p.transitions.length&&(planTracks.some(t=>!t?.analysis)||p.transitions.length!==planTracks.length-1))throw new Error('invalid project transitions');
    stopPreview();listeningRequest?.abort();listeningRequest=null;$('audio').pause();$('audio').removeAttribute('src');$('audio').load();
    state.output=null;state.playhead=0;state.history=[];state.future=[];state.loop=false;state.selectedTrack=-1;
    state.projectData=p;state.beforeEdits={};state.projectName=p.name;$('projectName').textContent=p.name;state.tracks=tracks;
    state.plan=p.transitions.length?{sorted_tracks:planTracks.map(t=>t.analysis),selections:p.transitions,alternatives:p.transitions.map(t=>[t]),score:{},evaluated:0,settings:p.settings}:null;
    const settings=p.settings||{};
    for(const [id,key] of Object.entries({transitionPreset:'transition_bars',listeningMode:'listening_mode',styleSelect:'style',depthSelect:'search_depth',transitionLength:'transition_length',phrasePriority:'phrase_priority',harmonicPriority:'harmonic_priority'})){if(settings[key]!==undefined)$(id).value=settings[key]}
    $('preserveOrder').checked=!!settings.preserve_order;
    state.evaluationMode=!!p.evaluation_mode;$('evaluationMode').checked=state.evaluationMode;$('evaluationPanel').classList.toggle('hidden',!state.evaluationMode);
    state.selected=state.plan?0:-1;$('exportBtn').disabled=!state.plan;setPlaying(false);updateUndo();renderLibrary();renderTimeline();
    if(state.plan)selectTransition(0);else showOverview();updateTransport();toast('프로젝트를 복원했습니다.','ok');
  }catch(e){friendlyError('프로젝트 열기',e)}
}
function friendlyError(action,e){const raw=String(e?.message||e),hint=/ffmpeg/i.test(raw)?'FFmpeg 번들을 확인하세요.':/yt-dlp/i.test(raw)?'네트워크 연결 후 다시 시도하세요.':/decode|unsupported|invalid/i.test(raw)?'지원되는 오디오 파일인지 확인하세요.':'잠시 후 다시 시도하고 계속되면 Details를 확인하세요.';toast(`${action} 실패 — ${hint} (${raw.slice(0,180)})`,'error');status(`${action} 실패`,'err')}
function replaceTransitionContents(target,source){for(const key of Object.keys(target))delete target[key];Object.assign(target,structuredClone(source))}
function validateTransitionEdit(t,i){
  const a=state.plan.sorted_tracks[i],b=state.plan.sorted_tracks[i+1],prev=state.plan.selections[i-1],next=state.plan.selections[i+1];
  const minA=prev?prev.b_in_time+prev.duration*(prev.speed_b||1):0,maxB=next?next.a_out_time:b.duration;
  const sa=t.speed_a||1,sb=t.speed_b||1,maxDuration=Math.min((a.duration-minA)/sa,maxB/sb);
  if(!Number.isFinite(t.duration)||t.duration<=0||t.duration>maxDuration+.001)throw new Error(`앞뒤 전환과 겹치지 않는 최대 길이는 ${maxDuration.toFixed(1)}초입니다. 곡 감상 길이를 늘리거나 다른 시작점을 선택하세요.`);
  if(!Number.isFinite(t.a_out_time)||!Number.isFinite(t.b_in_time))throw new Error('시작 위치를 숫자로 입력하세요.');
  t.a_out_time=clamp(t.a_out_time,minA,a.duration-t.duration*sa);t.b_in_time=clamp(t.b_in_time,0,maxB-t.duration*sb);
  for(const k of ['fade_out_seconds','fade_in_seconds'])if(t[k]!==undefined&&(!Number.isFinite(t[k])||t[k]<0||t[k]>t.duration))throw new Error('각 페이드 길이는 전체 전환 길이 이내여야 합니다.');
  t.bars=Math.round(t.bars||0);
}
function renderEditingDetails(){
  const t=currentTransition();if(!t)return;
  const cap=t.curve==='sequential'?t.duration/2:t.duration;
  for(const [id,k] of [['fadeOutSeconds','fade_out_seconds'],['fadeInSeconds','fade_in_seconds']]){$(id).max=t.duration;$(id).value=Math.min(t[k]||t.duration,cap).toFixed(2)}
  $('transitionExplanation').textContent=t.manual?'직접 편집한 전환입니다. 재생 중 수정하면 해당 전환을 다시 준비합니다.':t.explanation||'두 곡의 시작 위치와 겹치는 시간을 조절하세요.';
  for(const [id,key,offset] of [['cueWaveA','a_out_time',0],['cueWaveB','b_in_time',1]]){
    const a=state.plan.sorted_tracks[state.selected+offset],svg=$(id),data=a.waveform||[];
    const wave=Array.from({length:150},(_,i)=>{const v=data[Math.floor(i*data.length/150)]||0,h=Math.max(1,v*50);return `<rect x="${i*2}" y="${35-h/2}" width="1.7" height="${h}"/>`}).join('');
    const x=t[key]/a.duration*300,w=t.duration*(offset?(t.speed_b||1):(t.speed_a||1))/a.duration*300;
    svg.innerHTML=`<g fill="${offset?'#75cbb6':'#78b9ea'}">${wave}</g><rect x="${x}" y="0" width="${w}" height="70" fill="#f0a45d33"/><line x1="${x}" x2="${x}" y1="0" y2="70" stroke="#ffe5aa" stroke-width="3"/>`;
    svg.setAttribute('aria-valuemin','0');svg.setAttribute('aria-valuemax',a.duration);svg.setAttribute('aria-valuenow',t[key]);svg.setAttribute('aria-valuetext',`${filename(a.filepath)} ${t[key].toFixed(1)}초`);
  }
}
function bindCueWave(id,key,offset){
  const svg=$(id);svg.onpointerdown=e=>{
    if(!currentTransition())return;e.preventDefault();svg.setPointerCapture(e.pointerId);
    const track=state.plan.sorted_tracks[state.selected+offset],rect=svg.getBoundingClientRect();let value=currentTransition()[key];
    const move=ev=>{value=clamp((ev.clientX-rect.left)/rect.width,0,1)*track.duration;const line=svg.querySelector('line');if(line){line.setAttribute('x1',value/track.duration*300);line.setAttribute('x2',value/track.duration*300)}$(offset?'bEntry':'aExit').value=value.toFixed(2)};
    move(e);svg.onpointermove=move;svg.onpointerup=ev=>{svg.onpointermove=null;svg.onpointerup=null;editTime(key,value,ev)};svg.onpointercancel=()=>{svg.onpointermove=null;svg.onpointerup=null;renderEditingDetails()};
  };
  svg.onkeydown=e=>{if(!currentTransition()||!['ArrowLeft','ArrowRight'].includes(e.key))return;e.preventDefault();editTime(key,currentTransition()[key]+(e.key==='ArrowRight'?1:-1)*(e.shiftKey?.1:1),{altKey:true})};
}
async function regenerateSelectedTransition(){
  const i=state.selected,t=currentTransition();if(!t||!state.port)return;
  if(t.locked)return toast('이 전환은 잠겨 있습니다. 잠금을 풀고 다시 생성하세요.');
  if(state.regenerating)return;state.regenerating=true;$('regenerateSelected').disabled=true;
  const plan=state.plan;
  try{
    const d=await request('/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tracks:plan.sorted_tracks.slice(i,i+2),settings:{...mixSettings(),preserve_order:true,variation:(state.pairVariation=(state.pairVariation||0)+1)}})});
    if(state.plan!==plan||state.selected!==i)return;
    const replacement=d.plan?.selections?.[0];if(!replacement)throw new Error('전환 후보가 없습니다.');
    validateTransitionEdit(replacement,i);state.beforeEdits??={};state.beforeEdits[i]??=structuredClone(t);checkpoint();plan.selections[i]=replacement;plan.alternatives[i]=d.plan.alternatives[0];
    renderTimeline();renderInspector();applyPreviewNow();toast('다른 곡과 전환은 유지했습니다. 수정 후 듣기로 확인하세요.','ok');
  }catch(e){friendlyError('선택 전환 재생성',e)}finally{state.regenerating=false;$('regenerateSelected').disabled=false}
}
function revertSelectedTransition(){const old=state.beforeEdits?.[state.selected];if(!old)return toast('되돌릴 편집이 없습니다.');editTransition(t=>replaceTransitionContents(t,old))}
function transitionEnvelope(t,incoming,x){
  let d=(incoming?t.fade_in_seconds:t.fade_out_seconds)||t.duration;
  if(t.curve==='sequential')d=Math.min(d,t.duration/2);
  const time=x*t.duration,start=incoming?t.duration-d:0,u=clamp((time-start)/d,0,1),p=curvePower(t.curve,t.curve_amount);
  if(['equal_power','sequential',''].includes(t.curve||''))return incoming?Math.sin(u*Math.PI/2):Math.cos(u*Math.PI/2);
  return Math.pow(incoming?u:1-u,p);
}
init();
