# AutoMix DJ Bot 🎧

AutoMix DJ Bot은 유튜브 플레이리스트나 로컬 파일로부터 음악을 가져와, 인공지능 음악 분석을 통해 자동으로 자연스러운 DJ 믹스(Non-stop Mix)를 생성해주는 데스크탑 애플리케이션입니다.

![App Logo](app/src/app-icon.svg)

## ✨ 핵심 기능

- **스마트 음악 분석**: BPM(템포), Key(조성), Energy(에너지 레벨) 및 곡의 구조(Intro/Outro)를 분석합니다.
- **하모닉 믹싱 (Harmonic Mixing)**: Camelot Wheel 기반으로 서로 어울리는 조성을 가진 곡들을 우선적으로 배치하여 음악적인 믹스를 생성합니다.
- **자동 전환 기술**: 곡의 특성에 따라 Crossfade, Bass Swap, Filter Fade 등 다양한 전이 기법을 자동으로 선택합니다.
- **yt-dlp 자가 관리**: 실행 시마다 최신 유튜브 우회 패치를 자동으로 체크하여 다운로드 중단(403 Forbidden) 문제를 최소화합니다.
- **멀티 플랫폼 지원**: Windows, macOS, Linux에서 모두 사용 가능합니다.

## 🚀 시작하기

### 설치 방법
1. [GitHub Releases](https://github.com/vividhyeok/djbot/releases)에서 본인의 OS에 맞는 설치 파일을 다운로드합니다.
2. 설치 후 앱을 실행합니다.

### 사용법
1. **유튜브 링크 입력**: 유튜브 뮤직 플레이리스트 링크를 넣고 `Download`를 누르거나 엔터를 칩니다.
2. **또는 로컬 파일**: 직접 MP3/WAV 파일을 드래그 앤 드롭으로 추가할 수 있습니다.
3. **Smart Mix**: 곡 목록이 준비되면 아래의 큰 버튼을 눌러 믹스 생성을 시작합니다.
4. **결과 확인**: 생성이 완료되면 믹스된 MP3 파일과 가사 동기화용 LRC 파일을 확인할 수 있습니다.

## 🛠️ 기술 스택

- **Frontend**: Tauri, Vanilla JS, CSS3 (Glassmorphism UI)
- **Backend**: Go (Audio processing, CLI worker)
- **Audio Engine**: FFmpeg (Custom PCM renderer)
- **Automation**: GitHub Actions (Cross-platform builds)

## ⚖️ 라이선스

이 프로젝트는 개인적인 용도의 음악 믹싱 도구로 개발되었습니다. 유튜브 다운로드 시 저작권 규정을 준수하시기 바랍니다.

---
Created by [vividhyeok](https://github.com/vividhyeok)
