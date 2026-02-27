# AutoMix DJ Bot — 브랜드 가이드라인

버전 1.0 | 개인 프로젝트

---

## 브랜드 개요

AutoMix DJ Bot은 음악을 자동으로 믹싱하는 데스크탑 앱이다.
사용자가 곡을 넣으면 분석-플래닝-렌더링을 거쳐 완성된 믹스를 만들어준다.

**브랜드 키워드**: 정밀한 / 조용한 자신감 / 도구적 아름다움

화려하지 않지만 정확하다는 인상. DJ 장비의 절제된 미학에서 출발한다.

---

## 컬러 시스템

### 팔레트

| 토큰 | 헥스 | 용도 |
|------|------|------|
| `--color-base` | `#41431B` | 앱 전체 배경, 가장 어두운 베이스 |
| `--color-surface` | `#52561F` | 카드/패널 배경 (base보다 5% 밝게) |
| `--color-mid` | `#AEB784` | 보조 텍스트, 비활성 요소, 구분선 |
| `--color-light` | `#E3DBBB` | 서브 텍스트, 카드 내부 배경 |
| `--color-text` | `#F8F3E1` | 주요 텍스트, 강조 요소 |

### 사용 원칙

- 배경은 항상 `--color-base` 기반. 밝은 배경 금지.
- 주요 텍스트는 `--color-text`. 보조 정보는 `--color-mid`.
- 포인트 컬러를 따로 두지 않는다. 밝기 대비만으로 계층을 만든다.
- 파랑/보라/네온 계열 일절 사용하지 않는다.
- 상태 색상: 성공/활성 → `#AEB784`, 비활성/대기 → `#52561F`

### CSS 변수 선언

```css
:root {
  --color-base:     #41431B;
  --color-surface:  #4d501e;
  --color-mid:      #AEB784;
  --color-light:    #E3DBBB;
  --color-text:     #F8F3E1;

  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-lg: 16px;

  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
  --space-xl: 40px;
}
```

---

## 타이포그래피

### 폰트 패밀리

| 용도 | 폰트 | 비고 |
|------|------|------|
| 앱 타이틀 / 큰 헤딩 | **Syne** (Bold 700) | 기하학적이고 독특한 인상 |
| 본문 / UI 텍스트 (한글) | **Pretendard** (Regular 400, Medium 500) | 가독성 최우선 |
| 본문 / UI 텍스트 (영문) | **DM Sans** (Regular 400, Medium 500) | Pretendard와 자연스럽게 혼용 |
| 숫자 / 타임코드 | **DM Mono** | BPM, 시간, 수치 표시 |

Inter / Roboto / Arial / system-ui 사용 금지.

### 스케일

```
Title    : Syne 700  / 20px / line-height 1.2
Heading  : Pretendard 500 / 15px / line-height 1.4
Body     : Pretendard 400 / 13px / line-height 1.6
Caption  : Pretendard 400 / 11px / color: --color-mid
Mono     : DM Mono 400 / 12px
```

### 원칙

- 텍스트 계층은 크기보다 **색상 대비**로 표현한다.
- 대문자(ALL CAPS)는 캡션 레이블에만 제한적으로 사용.
- 자간(letter-spacing): 캡션 레이블에만 `0.08em` 적용.

---

## 컴포넌트 스타일

### 버튼

```css
/* Primary */
.btn-primary {
  background: var(--color-light);
  color: var(--color-base);
  font-family: 'Pretendard', sans-serif;
  font-weight: 500;
  font-size: 13px;
  padding: 10px 20px;
  border-radius: var(--radius-md);
  border: none;
  cursor: pointer;
  transition: background 150ms ease;
}
.btn-primary:hover {
  background: var(--color-text);
}

/* Ghost */
.btn-ghost {
  background: transparent;
  color: var(--color-mid);
  border: 1px solid var(--color-surface);
  padding: 8px 16px;
  border-radius: var(--radius-md);
}
```

### 인풋

```css
.input {
  background: var(--color-surface);
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-text);
  font-size: 13px;
  padding: 9px 12px;
  outline: none;
  transition: border-color 150ms ease;
}
.input:focus {
  border-color: var(--color-mid);
}
.input::placeholder {
  color: var(--color-mid);
  opacity: 0.6;
}
```

### 카드 / 패널

```css
.card {
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  padding: var(--space-lg);
  /* 그림자 대신 배경 대비로 구분 */
}
```

### 상태 뱃지

```css
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--color-mid);
  letter-spacing: 0.04em;
}
.badge::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-mid);
}
.badge.active::before {
  background: #AEB784;
  box-shadow: 0 0 0 3px rgba(174, 183, 132, 0.2);
}
```

---

## 로고 & 아이콘 시스템

### 로고 컨셉

믹싱 = 두 파형이 겹치는 순간. 로고는 **두 개의 호(arc)가 교차하는 형태**를 기반으로 한다.
원형이 아닌 비대칭 교차점 — 정확하지만 기계적이지 않은 인상.

### 로고 사용 규칙

- 밝은 배경: `--color-base` (#41431B) 단색
- 어두운 배경: `--color-text` (#F8F3E1) 단색
- 최소 사용 크기: 16px (아이콘), 24px (로고타입 포함)
- 로고 주변 여백: 로고 높이의 25% 이상 확보

### 앱 아이콘 디자인 원칙

- 배경: `#41431B`
- 아이콘 요소: `#E3DBBB` 또는 `#F8F3E1`
- 형태: 라운드 rect (macOS 스타일 / 512×512 기준 radius 110px)
- 복잡한 그라데이션/그림자 금지. 단색 플랫 아이콘.

---

## 보이스 & 토

앱 내 텍스트를 쓸 때:

| 하지 말 것 | 할 것 |
|----------|------|
| "AI가 자동으로 믹스를 생성합니다" | 그냥 버튼으로 액션 유도 |
| "분석 중입니다. 잠시만 기다려주세요" | "분석 중" |
| "파일을 업로드하거나 URL을 입력하세요" | placeholder로 대체 |
| "준비가 완료되었습니다!" | "준비됨" |

**원칙**: 사용자가 이미 아는 것은 설명하지 않는다. UI가 말하면 텍스트는 침묵한다.
