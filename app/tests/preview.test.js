import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

function harness(){
  let resolve;const pending=new Promise(r=>resolve=r);const played=[];
  const elements=new Map();
  const document={getElementById(id){if(!elements.has(id))elements.set(id,{pause(){},style:{}});return elements.get(id)}};
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
