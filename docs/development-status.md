# Production mixing work log

Branch: `improve/production-mixing`, baseline `383e824`.
The separate `C:/Users/user/djbot-work` checkout contains user changes and is not modified.

## Audit findings (2026-10-11)

- Critical: cache clearing deletes imported source files and exported mixes, breaking saved projects.
- Critical: worker listens on all interfaces with unrestricted CORS and file-serving endpoints.
- High: final renderer ignores tempo, rewrites overlap to >=8 seconds, applies normalization twice, and silently skips failed tracks.
- High: preview, final render and UI use different timelines, fades and effects. Full-set playback requires export.
- High: renderer allocates the entire float mix and another byte copy in RAM; temporary files leak on failure.
- High: upload overwrites same-named sources; cached analysis can return another source path.
- High: planner can loop indefinitely for zero BPM and return missing transitions for short tracks. Independent pairs can produce conflicting middle-track ranges.
- Medium: empty beat energy and very short waveform inputs can panic.
- Medium: tests simulate copied timeline formulas instead of rendering and checking audio.
- Medium: application shutdown kills workers by executable name, including other app instances.

## Priorities

1. Protect source/project data; establish regression tests and baseline.
2. Define shared sample-accurate timeline and DSP for preview and export, bounded-memory rendering, and safe planner fallbacks.
3. Connect full-mix playback and edits to the shared render path; validate persistence and usability.
4. Exercise actual FFmpeg audio rendering, multi-track mixes, and Windows packaging; commit and push verified changes.

## Verification

Baseline and added Go tests pass on Windows with installed FFmpeg. No listening or end-to-end UI verification claimed.

## Completed iterations

- `cd6360b`, `24a7c28`: preserve source/import/export data, analysis edge cases, loopback binding, bundled download dependency; pushed.
- Tempo matching now uses metrical half/double-time alternatives without halving audio playback. Low-confidence or distant tempos use short, natural-speed fallback transitions. Entry/exit candidates retain substantially more of the original recording.
- Added `BuildRenderTimeline`: explicit source seconds vs output seconds, tempo-adjusted transition regions with natural-speed body, exact overlap and validation. **Not yet wired to renderer or UI**; existing rendering parity defects remain pending.
- Regression coverage: 70/140 BPM, 120/124 beat-period equality, uncertain/missing BPM, short tracks, three-track timeline and conflicting middle cues. Full `go test ./...` passes.

## Next implementation

Replace legacy `RenderFinalMix`/`RenderPreview` internals with a shared renderer using `BuildRenderTimeline`. Apply effects locally to transition regions, preserve sample count, fail instead of skipping invalid tracks, avoid full-set memory canvas. Then connect frontend timeline/playback to the shared backend plan and render path. Verify actual exported PCM before claiming parity.

## Renderer iteration

`RenderFinalMix` now uses `render_pipeline.go` and the validated timeline. It stages independently decoded float-PCM clips and lets FFmpeg mix them, replacing the Go full-set canvas. Output is encoded to a sibling temporary file before rename; missing/failed sources abort instead of being skipped. Timings and LRC derive from the same timeline. Fade curves/custom amount and transition gains are sample-evaluated. One bounded RMS compensation replaces double normalization (not claimed as LUFS normalization).

Actual FFmpeg WAV/MP3 regression: two 12-second synthetic tones, 1.05/.95 transition speeds, 2-second overlap; decoded duration within 2 ms, peak bounds checked, LRC cue at 8 seconds, input playlist unchanged, temporary files cleaned. Existing output preservation on missing source verified. Full Go suite and vet pass.

Pending: transition EQ/filter/echo/type-specific envelopes and arbitrary automation are not yet implemented in the new renderer; legacy preview and frontend still use their old paths. This is an intermediate development branch, not a release-ready build. Next iteration must close these regressions and connect preview, then test multi-track/full app flows. Old renderer remains temporarily under `renderFinalMixLegacy` for reference and should be removed once replacement verified.

## Effects and backend preview iteration

- Added low/mid/high EQ and filter automation using 50 Hz FFmpeg commands, with 256-sample audio frames. Effects are enabled only within the transition interval. Bass-swap/filter-fade get default automation when unspecified.
- Actual audio regression confirms low-EQ attenuation within the transition, unchanged body RMS, and unchanged sample length. Filter command path executes successfully.
- Backend `/render/preview` now uses the same final renderer for an 8-second neighborhood, with original-track cached analysis for consistent gain. Preview cache includes content hashes and renderer revision; publication serialized. Full Go suite passes including FFmpeg preview rendering.
- **Still pending:** frontend currently bypasses backend preview with its own Web Audio implementation. Must connect it to shared rendering. Echo, cut/drop envelopes, pitch, volume automation, effect-boundary smoothing, actual preview/export sample comparison, multi-track tests, Windows build and UX verification remain required. No listening validation claimed.

## Frontend preview integration

Frontend `startLivePreview` now requests `/render/preview` and plays the shared renderer's output. Stop aborts the pending request and invalidates late responses; seeking uses the media clock; edits debounce a new render. Existing full-mix audio is paused before auditioning. Node regression tests execute the actual frontend functions with mocked media/network and verify late-response suppression and media seeking. `npm test` (2 tests) and syntax check pass. Desktop audio/UI still unverified; full-set playback without export remains pending. Legacy Web Audio helper functions remain unused and should be removed during cleanup.

## Windows build and sample comparison iteration

- Windows release build and NSIS installer generation succeeded locally (`app/src-tauri/target/release/bundle/nsis/DJBOT_1.6.0_x64-setup.exe`). Packaging rerun after backend audio correction; this is a development artifact, not a published release or installation test.
- Shutdown now tracks the owned child and terminates only its PID tree, not all same-named workers. Startup/close race guarded. Windows child processes hidden.
- Local build script now checks test/build exit codes and restores working directory on failure. CI runs frontend tests and reruns Go tests with the actual bundled FFmpeg on PATH; branch pushes build artifacts without publishing a release.
- New actual-PCM preview/export comparison initially failed (max sample error .0607). Skipping `atempo` for speed 1 reduced error; aligning control frames to 10 ms (441 samples) made the 3-second bass-swap fixture pass at <.0002 max error. This fixture is not proof of arbitrary-cue/all-effect parity; expand it.
- `go test ./...`, `go vet ./...`, frontend syntax and 2 Node regression tests pass. rustfmt component is absent; compilation succeeds without it.
- Local preexisting FFmpeg reports GPL configuration, whereas CI fetches LGPL. Local artifact is for validation; dependencies/notices and distribution license consistency remain to check before publishing.

## Whole-set listening iteration

Local repackaging finished successfully (session 41256); installer predates this iteration. Added `/render/listen` with content/plan keyed MP3 cache. Default Play now prepares and plays the entire mix without asking for an export filename; transition loop remains dedicated audition. Preparation can be cancelled without late-response playback, and edited plans invalidate pending results. Initial listening still waits for full rendering: progressive/bounded-latency playback remains required. Backend suite and frontend checks pass; cache/HTTP integration and desktop playback need verification. Windows CI run 38063234612 was still in progress at last inspection.

## Multi-track regression and CI repair

CI 38063234612 failed because Node 20 on Windows did not expand `tests/*.test.js`; changed test command to Node's built-in discovery (`node --test`). Added generated-plan tests for 2/3/12 tracks at .4/3/12/180 seconds. They exposed conflicting middle-track cues for short material: fallback exit anchor was zero. Fixed fallback to retain at least 75% of the track before exit. All combinations now satisfy the real timeline validator. An actual 12-track, 3-second synthetic-tone WAV render verifies duration (<2 ms drift), finite/unclipped samples and audible content in each track region. This does not establish musical quality on real music. Frontend tests and full Go suite pass locally.
