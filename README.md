# DJBOT

DJBOT은 플레이리스트를 분석해 의도적으로 구성된 DJ mix 초안을 만드는 Windows 데스크톱 앱입니다. 반복적인 beat/phrase 정렬과 후보 탐색은 알고리즘이 맡고, 사용자는 마음에 들지 않는 전환만 고칩니다.

## 기본 workflow

1. YouTube playlist URL 또는 로컬 음악 파일을 가져옵니다.
2. **분석**에서 BPM, key, beat/downbeat, phrase, section, energy, loudness, spectral 특징을 추출합니다.
3. 스타일과 탐색 깊이를 선택하고 **Auto Mix**를 누릅니다.
4. Visual Timeline에서 바로 구조를 확인하고 전환별 추천 대안을 미리 듣습니다.
5. 필요한 cue, 길이, transition type만 수정한 뒤 MP3 또는 WAV로 내보냅니다.

분석 결과와 preview는 캐시됩니다. 원본 오디오는 변경하지 않으며 `.djbot` 프로젝트에는 source reference, clip, transition, automation, mix setting만 저장합니다.

## 주요 기능

- 순수 Go Analyzer V2: Python/pip 불필요, 병렬 분석과 버전별 cache
- BPM/key confidence, onset-following beat grid, downbeat, novelty 기반 phrase/section
- Crossfade, Bass Swap, Filter Fade, Echo Out, Cut, Drop, Long Blend, Mashup
- Quick / Balanced / Deep / Max 다단계 metadata search
- beat, downbeat, phrase, harmony, BPM, tempo, energy, loudness, spectral/vocal collision 점수
- Auto, Smooth, Hype, Club, Chill, Journey set energy direction
- waveform timeline, Beat/Bar/Phrase snap, transition alternatives, undo/redo
- 전환 구간 빠른 preview와 별도 고품질 final render
- MP3 320 kbps, WAV 24-bit export
- future stem/generated material을 고려한 `AudioMaterial` / `TimelineClip` 모델

## 설치

[GitHub Releases](https://github.com/vividhyeok/djbot/releases)에서 최신 `DJBOT-vX.Y.Z-Windows-x64-Setup.exe`를 내려받아 실행합니다. Portable ZIP도 함께 제공됩니다. 릴리스에는 Go worker, FFmpeg, yt-dlp가 포함되므로 Python, pip, Node, Go 또는 별도 FFmpeg 설치가 필요 없습니다.

YouTube 서비스 측 변경으로 다운로드가 일시적으로 실패할 수 있습니다. DJBOT은 번들된 yt-dlp를 백그라운드에서 업데이트할 수 있습니다. 저작권과 서비스 약관을 준수해 본인이 사용할 권리가 있는 오디오만 가져오세요.

## 개발

필요 도구: Go 1.22+, Node 20+, Rust stable, Tauri 2 Windows prerequisites, FFmpeg.

```powershell
cd backend
go test ./...

cd ..\app
npm ci
npm run check
npm run tauri dev
```

로컬 패키징은 `scripts/build.ps1 -Release`로 실행합니다. `app/src-tauri/binaries`에 target triple 이름의 Go sidecar와 `ffmpeg.exe`, `yt-dlp.exe`가 필요합니다. 공식 release workflow가 이를 자동 준비합니다.

## Architecture

상세 구조와 데이터 흐름은 [docs/architecture.md](docs/architecture.md)를 참고하세요.

- `backend/analysis_v2.go`, `dsp.go`: MIR feature extraction
- `backend/planner.go`: set ordering, transition candidate generation/scoring/search
- `backend/renderer.go`: cached transition preview, PCM canvas final rendering
- `backend/project.go`: versioned non-destructive project persistence
- `app/src`: desktop timeline/editor UI
- `app/src-tauri`: worker lifecycle and bundled dependency resolution

## Third-party dependencies and licenses

- [Tauri 2](https://github.com/tauri-apps/tauri): Apache-2.0 / MIT
- [FFmpeg](https://ffmpeg.org/): release workflow bundles the BtbN **LGPL** Windows build. FFmpeg remains a separate executable invoked as a subprocess. FFmpeg build/license information is available from `ffmpeg -L`.
- [yt-dlp](https://github.com/yt-dlp/yt-dlp): Unlicense; separate executable.

배포자가 workflow의 FFmpeg URL을 GPL build로 변경하는 경우 해당 GPL 재배포 의무를 별도로 준수해야 합니다. 이 저장소는 사용자에게 Python audio package를 설치하도록 요구하지 않습니다.

## Release

semantic version tag를 push하면 테스트, dependency bundling, Tauri NSIS build, GitHub Release와 asset 첨부가 자동 실행됩니다.

```bash
git tag v1.5.0
git push origin v1.5.0
```

생성 asset:

- `DJBOT-v1.5.0-Windows-x64-Setup.exe`
- `DJBOT-v1.5.0-Windows-x64-Portable.zip`
