import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

function harness(){
  let resolve;const pending=new Promise(r=>resolve=r);const played=[];
  const elements=new Map();
  const document={querySelectorAll(){return []},getElementById(id){if(!elements.has(id))elements.set(id,{pause(){},removeAttribute(){},load(){},style:{}});return elements.get(id)}};
  class Audio {constructor(path){this.path=path;this.currentTime=0;this.duration=20}async play(){played.push(this.path)}pause(){}removeAttribute(){}load(){}}
  const ctx=vm.createContext({window:{},document,Audio,AbortController,structuredClone,setTimeout,clearTimeout,fetch:async()=>{const data=await pending;return {ok:true,json:async()=>data}}});
  vm.runInContext(readFileSync(new URL('../src/app.js',import.meta.url),'utf8').replace(/init\(\);\s*$/,''),ctx);
  vm.runInContext("state.port=123;state.selected=0;state.plan={sorted_tracks:[{filepath:'a'},{filepath:'b'}],selections:[{a_out_time:8,duration:2}]};state.arrangement=[{start:0,duration:10},{start:8,duration:10}];",ctx);
  return {ctx,resolve,played};
}

test('stopping during render prevents a late response from starting playback',async()=>{
  const h=harness();const job=vm.runInContext('startLivePreview()',h.ctx);
  vm.runInContext('stopPreview()',h.ctx);h.resolve({output_path:'preview.wav'});await job;
  assert.deepEqual(h.played,[]);assert.equal(vm.runInContext('state.preview',h.ctx),null);
});

test('preview plays the backend output and seeking uses the audio clock',async()=>{
  const h=harness();const job=vm.runInContext('startLivePreview()',h.ctx);
  h.resolve({output_path:'preview.wav'});await job;
  assert.deepEqual(h.played,['preview.wav']);
  vm.runInContext('seekBy(5)',h.ctx);assert.equal(vm.runInContext('state.preview.el.currentTime',h.ctx),5);
  vm.runInContext('stopPreview()',h.ctx);
});

test('cancelling whole mix preparation cannot start late playback',async()=>{
  const h=harness();const job=vm.runInContext('togglePlay()',h.ctx);
  await vm.runInContext('togglePlay()',h.ctx);
  h.resolve({output_path:'listen.mp3'});await job;
  assert.equal(vm.runInContext('state.output',h.ctx),null);
  assert.equal(vm.runInContext('listeningRequest',h.ctx),null);
});

test('saving before analysis preserves imported sources and project identity',()=>{
  const h=harness();
  vm.runInContext(`state.plan=null;state.tracks=[{path:'new.wav',analysis:null},{path:'old.wav',analysis:null}];
    state.projectData={id:'existing',created_at:'2026-01-01',materials:[{id:'material-0',kind:'track',source_path:'old.wav'}],analysis_overrides:[{material_id:'material-0',downbeats:[1]}]};`,h.ctx);
  const p=JSON.parse(vm.runInContext('JSON.stringify(projectDocument())',h.ctx));
  assert.equal(p.id,'existing');assert.equal(p.created_at,'2026-01-01');
  assert.deepEqual(p.materials.map(m=>m.source_path),['new.wav','old.wav']);
  assert.equal(p.materials[1].id,'material-0');assert.notEqual(p.materials[0].id,'material-0');
  assert.equal(p.analysis_overrides[0].material_id,p.materials[1].id);
});

test('saving a plan also retains library tracks not yet analysed',()=>{
  const h=harness();
  vm.runInContext(`state.plan={sorted_tracks:[{filepath:'a',duration:20},{filepath:'b',duration:20}],selections:[{a_out_time:15,b_in_time:0,duration:2}],settings:{}};
    state.tracks=[{path:'a'},{path:'b'},{path:'pending.wav',analysis:null}];`,h.ctx);
  const p=JSON.parse(vm.runInContext('JSON.stringify(projectDocument())',h.ctx));
  assert.equal(p.materials.length,3);assert.equal(p.clips.length,2);
  assert.equal(p.materials[2].source_path,'pending.wav');
});

test('serialization rejects sources which have not been uploaded',()=>{
  const h=harness();vm.runInContext('state.plan=null;state.tracks=[{path:null}]',h.ctx);
  assert.throws(()=>vm.runInContext('projectDocument()',h.ctx),/파일/);
});

test('timeline measures tempo-adjusted overlap in output seconds',()=>{
 const h=harness();
 vm.runInContext(`state.plan={sorted_tracks:[{duration:20},{duration:20}],selections:[{a_out_time:10,b_in_time:2,duration:4,speed_a:1.1,speed_b:.9}]}`,h.ctx);
 const a=JSON.parse(vm.runInContext('JSON.stringify(buildArrangement())',h.ctx));
 assert.equal(a[0].duration,14);assert.equal(a[1].start,10);
 assert.ok(Math.abs(a[1].duration-18.4)<1e-8);assert.equal(a[0].sourceEnd,14.4);
});

test('Auto Mix can run repeatedly and resets stale playback and undo',async()=>{
 const h=harness();
 vm.runInContext(`state.tracks=[{analysis:{filepath:'a',duration:20}},{analysis:{filepath:'b',duration:20}}];
 state.output='old.mp3';state.history=['old'];
 renderLibrary=renderTimeline=selectTransition=updateUndo=updateTransport=toast=status=()=>{};`,h.ctx);
 const first=vm.runInContext('autoMix()',h.ctx);
 h.resolve({plan:{sorted_tracks:[{filepath:'a',duration:20},{filepath:'b',duration:20}],selections:[{a_out_time:10,b_in_time:0,duration:2}],settings:{}}});
 await first;await vm.runInContext('autoMix()',h.ctx);
 assert.equal(vm.runInContext('state.generation',h.ctx),2);
 assert.equal(vm.runInContext('state.planning',h.ctx),false);
 assert.equal(vm.runInContext('state.output',h.ctx),null);
 assert.equal(vm.runInContext('state.history.length',h.ctx),0);
});

test('quick listening also shortens the final track in export and timeline',()=>{
 const h=harness();vm.runInContext(`state.plan={sorted_tracks:[{duration:240},{duration:240}],settings:{listening_mode:'quick'},selections:[{a_out_time:75,b_in_time:0,duration:2}]}`,h.ctx);
 assert.equal(vm.runInContext('playlist()[1].play_end',h.ctx),75);
 assert.equal(vm.runInContext('buildArrangement()[1].sourceEnd',h.ctx),75);
});
