# DJBOT Architecture

## Pipeline

```text
Audio sources
  ↓ decode (bundled FFmpeg, mono PCM)
Analysis V2
  ↓ cached TrackAnalysis
Candidate generation
  ↓ phrase/downbeat-aligned parameterized transitions
Cheap musical scoring
  ↓ pruned alternatives
Beam set search + Set Director
  ↓ MixPlan
Visual non-destructive editing
  ↓ MixProject (materials, clips, automation)
Cached local preview
  ↓ approval
High-quality PCM canvas render
  ↓ MP3 320 kbps / WAV 24-bit
```

## Processes

The Tauri/Rust shell owns the desktop window and starts a bundled Go sidecar on a random localhost port. It passes a writable OS app-data directory plus the bundled FFmpeg path. The static frontend talks only to that loopback API. Long analysis and render work therefore never blocks the WebView UI thread.

## Analysis

FFmpeg decodes every source to 22.05 kHz mono float PCM. The Go analyzer computes spectral-flux onset, autocorrelation tempo and confidence, locally onset-adjusted beats, energy-accented downbeats, key/profile confidence, loudness, per-beat energy, spectral band ratios and a compact waveform.

Phrase boundaries are bar-quantized novelty candidates from energy change and onset strength, regularized to useful minimum spacing. Sections are classified from position and relative energy into intro, verse, chorus/drop, breakdown and outro. This is deterministic classical MIR: it has no runtime model download and keeps packaging self-contained. Cache filenames include `AnalysisVersion`, so algorithm changes cannot silently reuse incompatible results.

## Planning and search

`MixSettings` selects style and Quick/Balanced/Deep/Max budget. A bounded beam search chooses set order using harmonic distance, tempo, energy continuity and the requested set energy curve. For every adjacent pair, the engine combines section/phrase anchors, 4/8/16/32-bar lengths and supported transition types. Invalid duration combinations are rejected before scoring.

Scores expose beat, downbeat, phrase, harmony, BPM, tempo-naturalness, energy, loudness, spectral collision, vocal collision and smoothness. Set aggregation adds transition diversity and energy-arc fit. Search never renders full candidates; only a user-selected alternative enters the preview renderer.

## Editing model

`MixProject` is a versioned JSON document. `AudioMaterial.Kind` may be `track`, `stem`, or `generated`; a `TimelineClip` references a material and source range independently of playlist order. This permits future vocal/drum/bass stems and generated intros/bridges without replacing the project format.

Transition changes update metadata and automation, never source audio. The UI keeps bounded undo/redo snapshots. Cue edits snap to beat, bar or phrase unless Alt is held. Project saves use a temporary file and atomic rename.

## Rendering

Preview decodes only the transition neighborhood and returns a cached local audio file. Final rendering normalizes sources, applies tempo/filter/fade behavior, overlays float PCM chunks on the set timeline, limits the master, and encodes once. MP3 uses 320 kbps; WAV uses 24-bit PCM. The existing LRC track index is retained.

## Packaging and dependencies

The release workflow builds the Go sidecar and Tauri app on `windows-latest`, downloads an LGPL static FFmpeg build and official yt-dlp executable, and embeds them as resources. NSIS installer and portable ZIP are attached to semantic-version GitHub Releases. Runtime Python, Node, Go and Rust are not required.

## Known boundaries

Section and vocal estimates use deterministic signal features rather than a large neural MIR model; unusual meters, live drummers and tempo-changing recordings may need manual cue correction. FFmpeg `atempo` preserves pitch for modest tempo matching, but the current final renderer does not yet implement a dedicated phase-vocoder or isolated stem mixing. The data model is ready for both.
