# DJBOT 사용자 QA 가이드

이 문서는 실제 음악으로 핵심 작업 흐름을 검증하기 위한 체크리스트다. 가능하면 장르와 BPM이 다른 8~12곡을 사용하고, 동일한 프로젝트에서 Quick/Deep 결과를 비교한다.

## 1. 분석 정확도

1. 파일을 가져오고 **분석**을 누른다.
2. Library의 BPM/key를 알고 있는 값과 비교한다.
3. 타임라인에서 beat, downbeat, phrase에 맞춰 Snap되는지 확인한다.
4. 라이브 연주나 박자가 불안정한 곡은 confidence가 낮게 취급되는지 확인한다.

## 2. 전환 평가

1. **Auto Mix** 후 전환을 선택한다.
2. **Loop Transition**을 켜고 앞 곡 말미부터 다음 곡 진입까지 반복 청취한다.
3. Advanced의 **Evaluation**을 켠 경우 GOOD / OK / BAD로 판단한다.
4. BAD라면 beat, phrase, vocal clash, energy, length, tempo, entry/exit 중 원인을 기록한다.

## 3. 탐색 깊이 비교

같은 곡과 설정으로 Quick, Balanced, Deep, Max를 각각 실행한다. 후보 수뿐 아니라 곡 순서, cue, transition type, 전체 energy arc가 실제로 달라지는지 확인한다. 수동 편집을 비교 전에 저장하거나 잠근다.

## 4. 후보 A/B/C 비교

전환 Inspector의 Alternative를 선택하고 같은 loop 구간에서 비교한다. 볼륨 차이만으로 더 좋게 느껴지지 않는지 주의한다. Evaluation Mode에서는 점수를 보기 전에 먼저 귀로 선택한다.

## 5. 수동 편집

- overlap 양 끝 handle을 드래그한다.
- Shift+drag로 미세 조절한다.
- Alt+drag로 Snap을 잠시 해제한다.
- A OUT/B IN이 phrase 또는 bar에 붙는지 확인한다.
- fade curve, gain, LOW, filter를 재생 중 바꾸고 즉시 들리는지 확인한다.
- 마음에 든 전환을 Lock한 뒤 Auto Mix를 다시 실행해 유지되는지 확인한다.
- Ctrl+Z / Ctrl+Shift+Z로 변경이 예상대로 되돌아가는지 확인한다.

## 6. 저장과 복구

프로젝트를 저장하고 앱을 완전히 종료한 뒤 다시 연다. 곡 순서, cue, transition, automation, curve, lock, mix settings가 복원되어야 한다. 원본 오디오 파일은 수정되지 않아야 한다.

## 7. Export와 parity

1. 선택한 전환 loop를 충분히 듣는다.
2. MP3 320 kbps와 WAV 24-bit를 각각 Export한다.
3. 같은 구간에서 cue, fade curve, EQ/filter 동작과 상대 볼륨을 Preview와 비교한다.
4. clipping, gap, click, 시간 drift가 없는지 전체 mix를 확인한다.

## 권장 회귀 시나리오

| 시나리오 | 기대 결과 |
|---|---|
| Import → Analyze → Auto Mix → Play | UI freeze 없이 재생 |
| Loop → overlap drag | 화면과 소리가 빠르게 갱신 |
| EQ/filter 변경 | 재생 중 즉시 변화 |
| Alternative 선택 → Undo | 이전 후보로 복원 |
| Lock → Auto Mix | 잠긴 전환 유지 |
| Save → restart → Open | 모든 수동 편집 복원 |
| Quick → Max | 평가 후보 수와 탐색 결과 차이 |
