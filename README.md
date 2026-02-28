# DJ Bot - AutoMix 🎧

YouTube 플레이리스트를 분석해 자동으로 DJ 믹스를 생성하는 데스크탑 앱.  
**Tauri v2 (프론트엔드) + Go (백엔드)** 구조로, FFmpeg/yt-dlp를 활용합니다.

---

## ✨ 주요 기능

| 기능 | 설명 |
|------|------|
| YouTube 플레이리스트 다운로드 | yt-dlp로 최대 N곡 일괄 다운로드 |
| 오디오 분석 | BPM, 키, 에너지, 세그먼트(Intro/Chorus/Outro 등) 분석 |
| 자동 믹스 플래닝 | Greedy NNS로 트랙 순서 최적화 + 5가지 트랜지션 후보 생성 |
| PCM Canvas 렌더링 | 실측 기반 단일 루프로 MP3 + LRC(타임스탬프) 동시 생성 |
| 멀티 버전 믹스 | 여러 버전을 생성·비교·선택 가능 |
| ZIP / 단건 다운로드 | MP3, LRC 파일을 ZIP 또는 개별로 다운로드 |
| 캐시 초기화 | 임시 파일 및 캐시 일괄 삭제 |

---

## 🛠 Tech Stack

- **Frontend**: Tauri v2, Vanilla JS, CSS
- **Backend**: Go (HTTP worker, sidecar 방식)
- **Audio**: FFmpeg (`dynaudnorm` 정규화, f32le PCM canvas 합성)
- **Downloader**: yt-dlp

---

## 🚀 개발 환경 설정

### 사전 설치

- [Node.js](https://nodejs.org/) 18+
- [Go](https://go.dev/) 1.21+
- [Rust & Cargo](https://www.rust-lang.org/) (Tauri 빌드)
- [FFmpeg](https://ffmpeg.org/) — PATH에 등록
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — PATH에 등록

### 개발 서버 실행

```powershell
# 1. 프로세스 정리 (이미 실행 중이면)
taskkill /F /IM tauri-app.exe /T
taskkill /F /IM goworker-x86_64-pc-windows-msvc.exe /T

# 2. Go worker 빌드
cd backend
go build -o ..\app\src-tauri\binaries\goworker-x86_64-pc-windows-msvc.exe .

# 3. Tauri dev 서버 실행
cd ..\app
npm install   # 최초 1회
npm run tauri dev
```

> **주의**: 개발 모드에서 Go worker는 `cache/`, `output/` 폴더를 프로젝트 루트(`djbot/`)에 생성합니다.  
> (Tauri가 `src-tauri/` 디렉토리 변경을 감지해 재시작하는 것을 방지하기 위함)

### 프로덕션 빌드 (MSI)

```powershell
cd app
npm run tauri build
# 결과물: app/src-tauri/target/release/bundle/msi/
```

---

## 📐 아키텍처

```
[Frontend: app/src]          [Backend: backend/]
  app.js ──────────────────▶ main.go (HTTP router)
    │  POST /plan              planner.go  ← 트랙 정렬 + 트랜지션 후보
    │  POST /render/mix        renderer.go ← PCM canvas 단일 루프 렌더링
    │  POST /download/youtube  downloader.go
    │  POST /analyze           analyzer.go
    └── Tauri invoke ─────▶  lib.rs (sidecar 관리, data-dir 전달)
```

### 믹싱 엔진 상세

**트랙 순서 결정**: 키 거리 + BPM 차이 + 에너지 기반 Greedy NNS  
**트랜지션 종류**: `crossfade` / `bass_swap` / `filter_fade` / `mashup` / `cut` (5종)  
**렌더링 방식**: 각 트랙을 f32le PCM으로 추출 → float32 canvas 배열에 additive overlay  
**LRC 동기화**: 이론값 대신 실제 `offsetSamples`에서 역산 → 드리프트 없음

---

## 📁 프로젝트 구조

```
djbot/
├── app/                       # Tauri 앱 (프론트엔드)
│   ├── src/                   # HTML / JS / CSS
│   └── src-tauri/             # Tauri 설정 및 Rust 코드
├── backend/                   # Go HTTP worker
│   ├── main.go                # 라우터 및 서버 시작
│   ├── planner.go             # 믹스 플래닝
│   ├── renderer.go            # PCM canvas 렌더링 + LRC 생성
│   ├── analyzer.go            # FFmpeg 기반 오디오 분석
│   ├── downloader.go          # yt-dlp 래퍼
│   └── simulate_test.go       # 타임라인 시뮬레이션 테스트
├── cache/                     # 런타임 임시 파일 (gitignore)
├── output/                    # 생성된 MP3/LRC (gitignore)
└── README.md
```
