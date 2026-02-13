# 🎧 AutoMix DJ Bot

힙합/클럽용 자동 믹스셋을 생성하는 Streamlit 기반 DJ 보조 도구입니다.  
트랙 분석(BPM/키/에너지/하이라이트) → 전환 후보 생성 → 미리듣기 선택 → 최종 MP3+LRC 출력까지 한 번에 처리합니다.

## 주요 기능
- **오디오 분석**: BPM, 키, 에너지 곡선, 구간(인트로/벌스/코러스/아웃트로) 추정
- **스마트 전환 생성**: crossfade, bass swap, cut, filter fade, mashup
- **가중치 기반 개인화**: `test_app.py`에서 학습한 선호 가중치 반영
- **최종 결과물 생성**: 고음질 MP3(320kbps) + LRC(트랙 점프용)
- **다운로드 패키지**: ZIP으로 일괄 다운로드

## 실행 방법

### 1) 환경 준비
```bash
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\\Scripts\\activate
pip install -r requirements.txt
```

> ffmpeg가 필요합니다. 환경에 따라 `setup_ffmpeg.py`를 사용하세요.

### 2) 앱 실행
```bash
streamlit run app.py
```

### 3) RL 학습 UI (선택)
```bash
streamlit run test_app.py
```

## 폴더 구조
- `app.py`: 메인 자동 믹싱 UI
- `test_app.py`: 선호도 학습(RL 스타일) UI
- `src/analyzer_engine.py`: 트랙 분석
- `src/transition_engine.py`: 전환 후보 생성/선택
- `src/mix_renderer.py`: 미리듣기/최종 렌더링
- `src/utils.py`: 공통 유틸(캐시/로깅/경로)

## 최근 개선 사항 (성능 + 사용성)

### 성능 개선
1. **콘텐츠 기반 해시 캐싱 강화**
   - 기존 경로/mtime 기반에서 파일 내용(head/tail chunk + size) 기반으로 개선해,
     업로드 경로가 달라도 동일 음원의 분석 캐시를 재사용합니다.
2. **중복 업로드 자동 스킵**
   - 동일 파일(해시 동일) 재업로드 시 분석을 건너뛰어 처리 시간을 단축합니다.
3. **스마트 믹스 시 계산 반복 최소화**
   - 키 계산용 note 인덱스 맵/에너지 정규화 함수로 반복 로직을 줄였습니다.
4. **전환 후보 생성 시 불필요한 배열 변환 감소**
   - beat grid 스냅 계산에서 numpy 배열 캐시를 사용해 반복 변환을 줄였습니다.

### 사용성 개선
1. **4/8 bar 가중치 동작 복원**
   - 전환 엔진이 4 bar만 강제하던 동작을 제거해 UI 슬라이더 설정(4/8 bar)이 실제 반영됩니다.
2. **에너지 기반 전환 포인트 선정 정확도 개선**
   - 잘못된 키(`energy_curve`) 참조를 `energy`로 정정해 자동 전환 포인트 품질을 개선했습니다.
3. **중복 파일 안내 메시지 추가**
   - 분석 중 중복 파일은 스킵 상태를 UI에 표시해 혼동을 줄였습니다.

## 출력물
- `output/club_mix_YYYYMMDD_HHMMSS.mp3`
- `output/club_mix_YYYYMMDD_HHMMSS.lrc`
- `output/club_mix_YYYYMMDD_HHMMSS.zip`

## 문제 해결 팁
- 분석/렌더링 속도가 느리면:
  - 파일 수를 줄여 먼저 테스트
  - 동일 파일 재업로드 시 캐시 재사용 여부 확인
- ffmpeg 관련 오류 발생 시:
  - 시스템 ffmpeg 설치 확인
  - `setup_ffmpeg.py` 실행

---
필요하시면 다음 단계로 **전환 품질(A/B 테스트 자동화)**, **GPU 분리(옵션)**, **대규모 라이브러리 배치 분석 모드**까지 확장해드릴 수 있습니다.
