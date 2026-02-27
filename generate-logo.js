#!/usr/bin/env node
/**
 * AutoMix DJ Bot — 로고 SVG 생성기
 * 실행: node generate-logo.js
 * 결과: logo-dark.svg, logo-light.svg, icon-512.svg, icon-32.svg
 */

const fs = require('fs');

const COLORS = {
  base:     '#41431B',
  mid:      '#AEB784',
  light:    '#E3DBBB',
  lightest: '#F8F3E1',
};

/**
 * 로고마크: 두 개의 호(arc)가 교차하는 파형 심볼
 * 믹싱 = 두 신호가 만나는 순간을 기하학적으로 표현
 *
 * 구성:
 *   - 왼쪽 arc: 아래로 볼록한 호
 *   - 오른쪽 arc: 위로 볼록한 호
 *   - 교차점에서 겹침 → clip-path로 교차 영역 강조
 */

function buildLogomark(size = 32, strokeColor = COLORS.lightest) {
  const cx = size / 2;
  const cy = size / 2;
  const r = size * 0.34;
  const sw = size * 0.065; // stroke width

  // 왼쪽 원 (중심: cx - offset)
  const offset = size * 0.14;
  const lx = cx - offset;
  const rx = cx + offset;

  return `
  <!-- 왼쪽 호 -->
  <circle
    cx="${lx}" cy="${cy}" r="${r}"
    fill="none"
    stroke="${strokeColor}"
    stroke-width="${sw}"
    stroke-linecap="round"
    opacity="1"
  />
  <!-- 오른쪽 호 -->
  <circle
    cx="${rx}" cy="${cy}" r="${r}"
    fill="none"
    stroke="${strokeColor}"
    stroke-width="${sw}"
    stroke-linecap="round"
    opacity="0.55"
  />
  <!-- 교차 강조 dot -->
  <circle
    cx="${cx}" cy="${cy}" r="${sw * 0.9}"
    fill="${strokeColor}"
  />`.trim();
}

function buildWordmark(color = COLORS.lightest) {
  // SVG text — Syne Bold 폰트 기반 (웹폰트 임베드 필요시 @font-face 추가)
  return `
  <text
    x="0" y="0"
    font-family="Syne, sans-serif"
    font-weight="700"
    font-size="14"
    fill="${color}"
    letter-spacing="-0.3"
    dominant-baseline="middle"
  >AutoMix</text>
  <text
    x="0" y="17"
    font-family="Syne, sans-serif"
    font-weight="400"
    font-size="9"
    fill="${color}"
    opacity="0.55"
    letter-spacing="1.2"
  >DJ BOT</text>`.trim();
}

// ─── 1. 가로형 로고 (어두운 배경용) ────────────────────────────────────────
function logoHorizontalDark() {
  const markSize = 36;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="36" viewBox="0 0 160 36">
  <g transform="translate(0, 0)">
    ${buildLogomark(markSize, COLORS.lightest)}
  </g>
  <g transform="translate(44, 18)">
    ${buildWordmark(COLORS.lightest)}
  </g>
</svg>`;
}

// ─── 2. 가로형 로고 (밝은 배경용) ──────────────────────────────────────────
function logoHorizontalLight() {
  const markSize = 36;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="36" viewBox="0 0 160 36">
  <g transform="translate(0, 0)">
    ${buildLogomark(markSize, COLORS.base)}
  </g>
  <g transform="translate(44, 18)">
    ${buildWordmark(COLORS.base)}
  </g>
</svg>`;
}

// ─── 3. 앱 아이콘 512px ─────────────────────────────────────────────────────
function appIcon512() {
  const size = 512;
  const radius = 110; // macOS 스타일 roundrect
  const markSize = 280;
  const offset = (size - markSize) / 2;

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">
  <!-- 배경 -->
  <rect width="${size}" height="${size}" rx="${radius}" fill="${COLORS.base}"/>
  <!-- 배경 텍스처: 미세한 그리드 -->
  <defs>
    <pattern id="grid" width="24" height="24" patternUnits="userSpaceOnUse">
      <path d="M 24 0 L 0 0 0 24" fill="none" stroke="${COLORS.mid}" stroke-width="0.4" opacity="0.15"/>
    </pattern>
  </defs>
  <rect width="${size}" height="${size}" rx="${radius}" fill="url(#grid)"/>
  <!-- 로고마크 -->
  <g transform="translate(${offset}, ${offset})">
    ${buildLogomark(markSize, COLORS.lightest)}
  </g>
</svg>`;
}

// ─── 4. 파비콘 / 소형 아이콘 32px ──────────────────────────────────────────
function appIcon32() {
  const size = 32;
  const markSize = 20;
  const offset = (size - markSize) / 2;

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">
  <rect width="${size}" height="${size}" rx="7" fill="${COLORS.base}"/>
  <g transform="translate(${offset}, ${offset})">
    ${buildLogomark(markSize, COLORS.lightest)}
  </g>
</svg>`;
}

// ─── 출력 ────────────────────────────────────────────────────────────────────
const outputs = {
  'logo-dark.svg':   logoHorizontalDark(),
  'logo-light.svg':  logoHorizontalLight(),
  'icon-512.svg':    appIcon512(),
  'icon-32.svg':     appIcon32(),
};

for (const [filename, content] of Object.entries(outputs)) {
  fs.writeFileSync(filename, content, 'utf8');
  console.log(`✓ ${filename}`);
}

console.log('\n완료. 폰트 임베드가 필요하면 각 SVG 상단에 아래를 추가하세요:');
console.log(`
<defs>
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Syne:wght@400;700&amp;display=swap');
  </style>
</defs>
`);
