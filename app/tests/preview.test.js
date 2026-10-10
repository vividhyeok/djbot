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
