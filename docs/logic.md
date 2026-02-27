# AutoMix DJ Bot — 믹싱 로직 상세 문서

> 개인 학습용. 프로젝트의 분석-플래닝-렌더링 파이프라인 전체를 설명한다.

---

## 1. 전체 파이프라인 흐름

```
[사용자] MP3/WAV 업로드
        ↓
[/upload] — 파일 저장 (cache/uploads/)
        ↓
[/analyze] — 오디오 분석 (BPM, Key, Energy, Highlights, Segments)
        ↓
[/plan] — 믹스 플래닝 (트랙 정렬 + 트랜지션 후보 생성 + 최적 선택)
        ↓
[/render/preview] — 각 트랜지션 구간 미리듣기 MP3 렌더
        ↓
[사용자 선택]
        ↓
[/render/mix] — 최종 믹스 MP3 + LRC 렌더
```

---

## 2. 오디오 분석 (`backend/analyzer.go` + `dsp.go`)

### 2.1 파일 디코딩
```
audio file → ffmpeg → mono float32 PCM @ 22050 Hz
```
- ffmpeg를 subprocess로 실행해 PCM f32le 형식으로 파이프 수신
- `encoding/binary.Read`로 float32 슬라이스를 한 번에 역직렬화

### 2.2 BPM 추정 (`estimateBPM`)
1. **Onset Envelope 계산** (`computeOnsetEnvelope`):
   - hop_size=512, frame_size=1024의 슬라이딩 윈도우
   - 각 프레임에 Hann 윈도우 적용 후 FFT
   - 스펙트럼 매그니튜드의 **양방향 차분(Spectral Flux)**: 이전 프레임 대비 증가분만 합산
   - 결과: 각 hop마다 onset strength 값 배열

2. **자기상관(Autocorrelation) BPM 검출**:
   - BPM 범위 60-200에 해당하는 lag 범위를 onset envelope에서 자기상관
   - 가장 높은 상관계수를 가진 lag → beat period → BPM
   - Octave error 보정: BPM이 200 초과면 반으로, 60 미만이면 2배

### 2.3 비트 타임 추정 (`estimateBeatTimes`)
- 검출된 BPM으로 `beat_period = 60 / BPM` 계산
- 0초부터 duration까지 beat_period 간격으로 균등 배치
- (완벽하지 않음 — librosa처럼 onset 기반 phase 정렬을 하지 않음)

> **한계**: 현재 beat_times는 단순 등간격 배치이므로 실제 비트와 수초 이상 어긋날 수 있다. 향후 onset-peak 기반 phase alignment로 개선 가능.

### 2.4 에너지 계산 (`computeBeatEnergy`)
- hop_size=512, frame_size=2048로 RMS 프레임 배열 생성
- 각 비트 구간(beat[i] ~ beat[i+1])에 해당하는 RMS 프레임들의 평균 → 비트별 에너지
- 전체 에너지를 최대값으로 나눠 0~1 정규화

### 2.5 키(Key) 검출 (`detectKey`)
- Chroma 기반 Krumhansl-Schmuckler 키 프로파일 매칭
- 4096-sample frame, 2048 hop으로 FFT
- 65~4000 Hz 주파수 bin을 12개 음계(pitch class)로 매핑
- 12 rotation × (장조/단조) = 24개 키에 대해 **피어슨 상관계수** 계산
- 가장 높은 상관계수 → 검출 키 (예: "C Major", "A# Minor")

### 2.6 세그먼트 분류 (`classifySegments`)
- 32비트 = 하나의 "phrase" 단위
- 각 phrase 블록의 평균 에너지 계산
- 에너지 백분위 기반 임계값:
  - 30th percentile 이하 → **Bridge** (조용한 중간 파트)
  - 70th percentile 이상 → **Chorus** (고에너지 파트)
  - 시작 15% 이내 + 에너지 낮음 → **Intro**
  - 끝 15% 이내 + 에너지 낮음 → **Outro**
  - 나머지 → **Verse**

### 2.7 하이라이트 검출 (`detectHighlights`)
- 64비트(= 16바, 약 32초) 슬라이딩 윈도우, step=16비트(4바)
- 각 윈도우의 평균 에너지 계산 → **score**
- 시작 인덱스를 4비트(1바) 경계에 스냅
- 상위 3개 반환
- Python fallback에선 에너지 65% + 보컬 추정 35% 혼합 스코어 사용

---

## 3. 믹스 플래닝 (`backend/planner.go`)

### 3.1 트랙 순서 최적화 (`sortPlaylist`)
Greedy nearest-neighbour 알고리즘:
1. 첫 번째 트랙에서 시작
2. 남은 트랙들에 대해 스코어 계산:
   ```
   score = (60 - key_distance × 10)  // 하모닉 근접도 (0~60점)
           + (20 - |BPM_diff|)        // BPM 근접도 (0~20점)
           + avgEnergy × 20           // 에너지 가중치
   ```
   - `key_distance`: 두 키의 반음 차이 (0~6, Circle of Fifths)
3. 스코어 최대인 트랙을 다음으로 선택 → 반복

### 3.2 트랜지션 후보 생성 (`generateCandidates`)
**5개 시나리오 × 8개 후보**를 생성해 총 40개 중 최적 선택:

**Stratified Sampling**: 무작위 선택 대신 구조 레이블 기반 선택
- A 트랙에서: Chorus / Verse / Bridge / Outro 중 하나 선택
- B 트랙에서: Intro / Verse / Chorus / Bridge 중 하나 선택
- Outro→Intro 조합은 5% 확률로만 허용 (지루한 믹스 방지)

**구간 타이밍 계산**:
```
a_out_time = exit_segment.time + 20초 → 16비트 그리드 스냅
b_in_time  = entry_segment.time      → 16비트 그리드 스냅
```

**BPM 동기화**:
```
target_bpm = (bpm_a + bpm_b) / 2
speed_a = target_bpm / bpm_a   // A 재생 속도 (atempo 필터에 전달)
speed_b = target_bpm / bpm_b   // B 재생 속도
```

**트랜지션 타입** (userWeight 가중 랜덤 선택):
| 타입 | 설명 |
|------|------|
| `crossfade` | 볼륨 크로스페이드 |
| `bass_swap` | A에 High-pass → 베이스 제거 후 B 진입 |
| `cut` | 하드 드롭 (비트에 맞춰 즉시 전환) |
| `filter_fade` | A에 Low-pass → 고음 제거 페이드 |
| `mashup` | A+B 동시 재생 (A -1dB, B +1dB HP) |

### 3.3 최적 후보 선택 (`selectBest`)
```
score = type_weight × (1 - speed_deviation) - chrono_penalty
```
- `chrono_penalty = -500`: a_out_time이 이전 b_in_time보다 4초 미만일 경우 (시간 역행 방지)
- 타이브레이킹: `+random(0, 0.01)` (완전 동점 방지)

### 3.4 재생 구간 계산 (`ComputePlayBounds`)
각 트랙의 실제 재생 구간을 하이라이트 및 트랜지션 타이밍으로 결정:

```
첫 번째 트랙:
  play_start = highlight.start_time에서 8바(32비트) 전 → beat 스냅
  play_end   = transitions[0].a_out_time

중간 트랙들:
  play_start = transitions[i-1].b_in_time
  play_end   = transitions[i].a_out_time

마지막 트랙:
  play_start = transitions[-1].b_in_time
  play_end   = highlight.end_time + 4바 버퍼 → beat 스냅
```

---

## 4. 렌더링 (`backend/renderer.go`)

### 4.1 트랜지션 프리뷰 렌더 (`RenderPreview`)
ffmpeg `filter_complex`로 단일 ffmpeg 호출:

```
A 소스: [tOut - 10초, tOut + overlap]
B 소스: [bIn, bIn + overlap + 10초]
```

타입별 filter_complex 예시 (crossfade):
```
[0:a]atempo=1.02,afade=t=out:st=9.8:d=16[a];
[1:a]atempo=0.98,adelay=9800|9800,afade=t=in:d=16[b];
[a][b]amix=inputs=2:duration=longest:normalize=0[out]
```

### 4.2 최종 믹스 렌더 (`RenderFinalMix`)
**세그먼트 분할-합성** 방식:

```
[곡 1 body]   = playlist[0].PlayStart ~ (trans[0].a_out_time - trans[0].duration)
[전환 zone 1] = renderTransitionSegment() → WAV
[곡 2 body]   = (trans[0].b_in_time + trans[0].duration) ~ (trans[1].a_out_time - trans[1].duration)
...
[마지막 곡 body] = (trans[-1].b_in_time + duration) ~ PlayEnd
```

모든 WAV를 ffmpeg concat 필터로 이어붙인 뒤 MP3 320kbps 인코딩.

### 4.3 LRC 파일 생성 (`writeLRC`)
각 트랙 시작 오프셋(ms)을 LRC 타임태그 형식으로 기록:
```
[00:00.00] Song A
[02:45.50] Song B
[05:20.12] Song C
```
- 곡명: `Filename`에서 확장자 제거
- LRC 클릭 → 플레이어에서 해당 타임점프 지원

### 4.4 BPM 동기화 (atempo 필터)
```go
func buildAtempoFilter(speed float64) string
```
- `atempo` 필터는 0.5~100.0 범위만 지원
- 범위 초과 시 체인: `atempo=0.5,atempo=0.5,...`
- 1.0 ± 1%는 `anull` (패스스루)로 처리

---

## 5. 그리드 스냅 (마디 경계 정렬)

믹스가 마디 중간에 시작/종료되면 어색하므로 모든 타이밍은 박자 그리드에 스냅:

```go
func snapGrid(timeSec float64, beats []float64, grid int) float64
```
1. `beats[]`에서 `timeSec`에 가장 가까운 비트 인덱스 탐색
2. 해당 인덱스를 `grid`(기본 16비트 = 4바)의 배수로 반올림
3. 경계 초과 시 클램핑

---

## 6. 알려진 한계 및 개선 가능 영역

| 현황 | 문제 | 개선 방안 |
|------|------|-----------|
| beat_times 등간격 배치 | 실제 비트와 어긋날 수 있음 | onset-peak phase alignment |
| BPM 단순 자기상관 | 반템포/배템포 오류 가능 | FFT 기반 tempogram |
| 보컬 추정 밴드패스 필터 | 부정확 (기타/스네어 포함) | Demucs stem separation |
| 하이라이트 에너지 only | 곡 구조를 모름 | Madmom spectral novelty |
| BPM 동기화 atempo only | 음질 저하 가능 | WSOLA (librosa time_stretch) |
