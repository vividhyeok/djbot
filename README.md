# AutoMix DJ Bot 🎧

> 유튜브 플레이리스트나 내 음악 파일을 **자동으로 DJ 믹스**로 만들어주는 데스크탑 앱

---

## 이런 분께 딱입니다

- DJ 소프트웨어를 배우지 않고도 자연스러운 Non-stop 믹스를 만들고 싶은 분
- 유튜브 뮤직 플레이리스트를 MP3 믹스 파일 하나로 합치고 싶은 분
- 파티, 운동, 공부용 배경음악 믹스가 필요한 분

---

## 주요 기능

| 기능 | 설명 |
|------|------|
| **스마트 음악 분석** | BPM(템포), Key(조성), Energy(에너지)를 자동 분석 |
| **하모닉 믹싱** | Camelot Wheel 기반으로 음악적으로 어울리는 순서로 곡을 배치 |
| **자동 전환 효과** | Crossfade, Bass Swap, Filter Fade 중 곡 특성에 맞는 방식을 자동 선택 |
| **yt-dlp 자가 관리** | yt-dlp가 없으면 자동으로 다운로드, 이후 항상 최신 버전으로 유지 |
| **멀티 플랫폼** | Windows / macOS / Linux 모두 지원 |

---

## 시작하기

### 1단계 — ffmpeg 설치 (필수)

앱이 오디오 분석과 믹싱에 ffmpeg를 사용합니다.
**없으면 앱이 동작하지 않으니 반드시 먼저 설치해 주세요.**

<details>
<summary>Windows</summary>

**방법 A (권장) — winget 사용:**
```
winget install Gyan.FFmpeg
```

**방법 B — 수동 설치:**
1. https://ffmpeg.org/download.html 에서 Windows 빌드 다운로드
2. 압축 해제 후 `ffmpeg.exe`가 있는 `bin` 폴더를 환경변수 PATH에 추가
3. 터미널에서 `ffmpeg -version`으로 확인

**방법 C — Chocolatey 사용:**
```
choco install ffmpeg
```

**방법 D — Scoop 사용:**
```
scoop install ffmpeg
```
</details>

<details>
<summary>macOS</summary>

**Homebrew 사용 (권장):**
```bash
brew install ffmpeg
```

Homebrew가 없다면: https://brew.sh
</details>

<details>
<summary>Linux (Ubuntu / Debian 계열)</summary>

```bash
sudo apt update && sudo apt install ffmpeg
```

Fedora / RHEL:
```bash
sudo dnf install ffmpeg
```
</details>

---

### 2단계 — 앱 다운로드 및 설치

[GitHub Releases](https://github.com/vividhyeok/djbot/releases/latest)에서 내 운영체제에 맞는 파일을 받습니다.

| 운영체제 | 파일 |
|---------|------|
| Windows | `AutoMix-DJ-Bot_x.x.x_x64-setup.exe` |
| macOS (Apple Silicon M1/M2/M3) | `AutoMix-DJ-Bot_x.x.x_aarch64.dmg` |
| macOS (Intel) | `AutoMix-DJ-Bot_x.x.x_x64.dmg` |
| Linux | `AutoMix-DJ-Bot_x.x.x_amd64.AppImage` |

> **yt-dlp는 별도 설치가 필요 없습니다.** 앱 최초 실행 시 자동으로 다운로드됩니다.

---

### 3단계 — 앱 사용법

#### 유튜브 플레이리스트로 믹스 만들기

1. 앱을 실행하고 상단 입력창에 **유튜브 뮤직 플레이리스트 URL**을 붙여넣습니다
2. `Download` 버튼을 누르거나 Enter를 칩니다
   → 곡들이 자동으로 다운로드됩니다 (최대 30곡)
3. 다운로드가 완료되면 곡 목록이 나타납니다
4. 하단의 **Smart Mix** 버튼을 누릅니다
5. 잠시 기다리면 믹스 파일이 완성됩니다

#### 내 파일로 믹스 만들기

1. MP3 / WAV 파일을 앱 화면에 **드래그 앤 드롭**합니다
2. **Smart Mix** 버튼을 누릅니다

#### 결과물 확인

완성된 파일은 앱 데이터 폴더의 `output/` 에 저장됩니다:

| 파일 | 설명 |
|------|------|
| `mix_YYYYMMDD_HHMMSS.mp3` | 믹스된 음악 파일 |
| `mix_YYYYMMDD_HHMMSS.lrc` | 가사/트랙 타임라인 (LRC 형식) |

---

## 자주 묻는 질문 (FAQ)

### Q. "yt-dlp가 다운로드 중입니다. 잠시 후 다시 시도해 주세요"라는 메시지가 떠요

최초 실행 시 yt-dlp를 자동으로 다운로드합니다.
1~2분 기다린 뒤 다시 Download를 눌러 주세요.

### Q. 유튜브 다운로드가 403 Forbidden 오류로 실패해요

앱이 자동으로 Chrome → Edge → Firefox 쿠키를 순서대로 시도합니다.
- 브라우저에서 유튜브에 로그인한 상태라면 보통 해결됩니다
- 앱을 껐다가 다시 시작하면 yt-dlp 자동 업데이트가 실행되어 해결되는 경우도 많습니다

### Q. "ffmpeg not found" 오류가 나요

위의 [ffmpeg 설치](#1단계--ffmpeg-설치-필수) 방법대로 설치해 주세요.
설치 후 앱을 재시작하면 자동으로 인식합니다.

### Q. macOS에서 앱이 실행되지 않아요 ("손상되었습니다" 메시지)

터미널에서 다음 명령을 실행하세요:
```bash
xattr -cr "/Applications/AutoMix DJ Bot.app"
```

### Q. 믹스 파일이 어디에 저장되나요?

| 운영체제 | 저장 위치 |
|---------|---------|
| Windows | `C:\Users\<이름>\AppData\Roaming\com.djbot.automix\output\` |
| macOS | `~/Library/Application Support/com.djbot.automix/output/` |
| Linux | `~/.local/share/com.djbot.automix/output/` |

---

## 개발자용: 소스에서 빌드하기

### 필요 도구

- [Go 1.22+](https://go.dev)
- [Rust (stable)](https://rustup.rs)
- [Node.js 20+](https://nodejs.org)
- [ffmpeg](https://ffmpeg.org)

### Windows에서 빌드

```powershell
# 개발 모드 (hot-reload)
.\scripts\build.ps1

# 배포용 인스톨러 빌드
.\scripts\build.ps1 -Release
```

### macOS / Linux에서 빌드

```bash
# Go 백엔드 빌드
cd backend
go build -o ../app/src-tauri/binaries/goworker-$(rustc -vV | grep host | cut -d' ' -f2) .

# Tauri 앱 실행 (개발 모드)
cd ../app
npm install
npm run tauri dev

# 배포용 빌드
npm run tauri build
```

---

## 기술 스택

- **Frontend**: Tauri v2, Vanilla JavaScript, CSS3 (Glassmorphism UI)
- **Backend**: Go 1.22 (HTTP API server, 오디오 분석 엔진)
- **오디오 처리**: FFmpeg (디코딩, 인코딩), 커스텀 PCM 믹서 (Go)
- **빌드/배포**: GitHub Actions (크로스 플랫폼 자동 빌드)
- **유튜브 다운로드**: yt-dlp (자동 관리, 자동 업데이트)

---

## 주의사항 및 라이선스

- 이 도구는 개인적 용도의 음악 믹싱을 위해 만들어졌습니다
- 유튜브 콘텐츠 다운로드 시 해당 콘텐츠의 저작권 및 유튜브 이용약관을 준수하시기 바랍니다
- 개인 소장 또는 비상업적 용도로만 사용하세요

---

Created by [vividhyeok](https://github.com/vividhyeok)
