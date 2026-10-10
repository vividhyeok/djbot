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

Initial source inspection in progress. Baseline Go tests started. No listening or end-to-end UI verification claimed.
