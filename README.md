# 🎧 DJ Bot — 자동 DJ 믹스 생성기

YouTube 재생목록 또는 로컬 음악 파일을 입력하면, 자동으로 분석 → 하모닉 정렬 → 크로스페이드 믹싱하여 하나의 DJ 믹스 MP3를 생성합니다.

## ✨ 주요 기능

- **YouTube 재생목록 다운로드** — YouTube/YouTube Music 재생목록 URL 입력 → 자동 다운로드
- **Camelot Wheel 하모닉 정렬** — Key + BPM 기반 최적 트랙 순서 자동 결정
- **8바 크로스페이드 믹싱** — BPM 기반 자연스러운 전환 (곡당 ~60초 하이라이트)
- **Go 기반 고속 분석** — BPM, Key, 에너지, 구조(Intro/Verse/Chorus/Bridge/Outro) 자동 감지
- **LRC 트랙리스트** — 믹스 MP3와 함께 타임스탬프 트랙리스트 생성
- **중복 곡 자동 제거** — 같은 제목의 중복 트랙 필터링

## 🏗️ 아키텍처

```
djbot/
├── app_auto.py              # Streamlit 메인 앱 (자동 믹스)
├── app.py                   # Streamlit 메인 앱 (수동 믹스)
├── src/
│   ├── analyzer_engine.py   # 오디오 분석 (BPM, Key, 에너지, 구조)
│   ├── transition_engine.py # 전환 후보 생성 및 선택
│   ├── mix_renderer.py      # 믹스 렌더러 (WAV 변환 + pydub 크로스페이드)
│   ├── youtube_downloader.py# YouTube 재생목록 다운로드 (yt-dlp)
│   ├── go_bridge.py         # Go worker 프로세스 관리
│   ├── stem_separator.py    # 스템 분리 (선택사항)
│   └── utils.py             # 유틸리티 함수 및 로깅
├── goworker/
│   ├── main.go              # Go HTTP 서버 (분석 API)
│   ├── analyzer.go          # BPM/Key/에너지 분석
│   ├── dsp.go               # DSP 함수 (FFT, 필터)
│   ├── renderer.go          # Go 믹스 렌더러
│   ├── types.go             # 타입 정의
│   └── goworker.exe         # 컴파일된 바이너리
└── output/                  # 생성된 믹스 파일
```

## 🔧 설치

### 요구사항
- Python 3.10+
- Go 1.21+ (goworker 빌드 시)

### Python 패키지

```bash
pip install streamlit pydub librosa yt-dlp imageio-ffmpeg scipy numpy soundfile
```

> `imageio-ffmpeg`이 ffmpeg 바이너리를 자동으로 설치합니다. 별도 ffmpeg 설치 불필요.

### Go Worker 빌드 (선택사항)

```bash
cd goworker
go build -o goworker.exe .
```

> 사전 빌드된 `goworker.exe`가 이미 포함되어 있습니다.

## 🚀 실행

```bash
# 자동 믹스 모드 (권장)
python -m streamlit run app_auto.py

# 수동 믹스 모드
python -m streamlit run app.py
```

브라우저에서 `http://localhost:8501` 접속

## 📖 사용법

### 자동 믹스 (app_auto.py)

1. 좌측 사이드바에서 **YouTube 재생목록 URL** 입력
2. **"🚀 다운로드 & 자동 믹스"** 클릭
3. 자동으로 다운로드 → 분석 → 하모닉 정렬 → 믹스 계획
4. **"🎧 최종 믹스 렌더링"** 클릭으로 MP3 생성
5. ZIP 다운로드 (MP3 + LRC 트랙리스트)

### 수동 믹스 (app.py)

1. MP3/WAV 파일 업로드
2. Go worker가 자동 분석 (BPM, Key, 구조)
3. 트랙 순서 조정 및 전환 편집
4. In/Out 포인트 수동 설정 가능

## 🎛️ 믹싱 알고리즘

### Camelot Wheel 하모닉 정렬
Key 호환성을 Camelot Wheel 기준으로 점수화:
- **같은 키 / 관계조**: 100점
- **인접 위치 (±1)**: 80점
- **2단계 거리**: 40점

### BPM 매칭
- **±3 BPM 이내**: 50점
- **±8 BPM 이내**: 35점
- **±25 BPM 초과**: -30점 (페널티)

### 크로스페이드
- 8바 기준 크로스페이드 (BPM에 비례)
- pydub 네이티브 크로스페이드 사용 → 정확한 타이밍
- 모든 트랙 WAV 프리컨버팅 → 타이밍 드리프트 제거

## 📋 기술 스택

| 구성요소 | 기술 |
|---------|------|
| UI | Streamlit 1.52 |
| 오디오 분석 | Go worker (FFT, autocorrelation) |
| 오디오 렌더링 | pydub + ffmpeg (via imageio-ffmpeg) |
| 다운로드 | yt-dlp |
| Key 감지 | Krumhansl-Schmuckler algorithm |
| 비트 감지 | Onset detection + autocorrelation |

## ⚠️ 참고

- YouTube Music URL (`music.youtube.com`)은 자동으로 `www.youtube.com`으로 변환됩니다
- Chrome이 실행 중이면 쿠키 접근이 제한될 수 있습니다 (403 에러 시 Chrome 종료 후 재시도)
- 첫 실행 시 WAV 변환에 시간이 걸리지만, 이후 캐시됩니다
