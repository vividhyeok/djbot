# DJBOT

**Automation first. Control when wanted.**

DJBOT은 음악을 분석해 믹스 초안을 만들고, 결과를 DAW처럼 직접 다듬을 수 있는 Windows DJ workstation이다. 원본 오디오는 변경하지 않는다.

## 기본 흐름

1. 로컬 오디오 또는 YouTube playlist를 가져온다.
2. BPM, key, beat/downbeat, phrase, section, energy, loudness를 분석한다.
3. Style과 Search depth를 선택하고 **Auto Mix**를 실행한다.
4. 절대시간 타임라인에서 곡 배치와 실제 overlap을 확인한다.
5. 전환을 loop하며 cue, 길이, fade curve, gain, EQ, filter를 직접 조절한다.
6. MP3 320 kbps 또는 WAV 24-bit로 Export한다.

## 편집 기능

- 두 lane absolute-time arrangement와 실제 waveform peak
- transition edge drag, A OUT/B IN cue와 Beat/Bar/Phrase Snap
- Shift+drag 미세 조절, Alt+drag Snap 일시 해제
- Linear, Equal Power, Smooth, Sharp, Custom fade
- transition loop와 Web Audio 기반 저지연 gain/EQ/filter 피드백
- Bass Swap, Filter Fade, Echo Out, Cut, Drop, Long Blend, Mashup
- Alternative 후보, Undo/Redo, transition Lock
- Quick / Balanced / Deep / Max metadata search와 set energy direction
- confidence-aware transition scoring
- versioned `.djbot` project serialization

## 설치

[GitHub Releases](https://github.com/vividhyeok/djbot/releases)에서 Windows installer 또는 portable ZIP을 받는다. 배포 파일에는 Go worker, FFmpeg, yt-dlp가 포함되므로 Python, Node, Go, Rust 또는 별도 FFmpeg 설치가 필요 없다.

## 개발

요구 도구: Go 1.22+, Node 20+, Rust stable, Tauri 2 Windows prerequisites, FFmpeg.

```powershell
cd backend
go test ./...

cd ..\app
npm ci
npm run check
```

로컬 패키징:

```powershell
.\scripts\build.ps1 -Release
```

## 구조

- `backend/analysis_v2.go`, `dsp.go`: MIR feature extraction과 confidence
- `backend/planner.go`: set ordering, transition generation/scoring/search
- `backend/renderer.go`: cached preview와 final PCM render
- `backend/project.go`: non-destructive project persistence
- `app/src`: Library / Timeline / Inspector / Transport UI와 live preview
- `app/src-tauri`: worker lifecycle 및 dependency bundling

자세한 구조는 [docs/architecture.md](docs/architecture.md), 실제 청취 검증 절차는 [docs/testing.md](docs/testing.md)를 참고한다.

## Release

`vX.Y.Z` tag를 push하면 backend test, frontend check, bundled dependency 준비, Tauri NSIS build, portable ZIP 생성과 GitHub Release 첨부가 실행된다.

- `DJBOT-vX.Y.Z-Windows-x64-Setup.exe`
- `DJBOT-vX.Y.Z-Windows-x64-Portable.zip`

FFmpeg는 별도 실행 파일로 포함되는 LGPL build를 사용한다. yt-dlp는 upstream official executable을 사용한다.
