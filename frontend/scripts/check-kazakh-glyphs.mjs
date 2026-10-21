// Proves that the self-hosted fonts render every letter of the Kazakh alphabet.
// The check follows the browser's rule (CSS Fonts 4, composite faces): faces
// whose unicode-range covers a code point are tried in reverse order of
// definition until one actually contains the glyph. Unbounded and Manrope,
// suggested by the brief, fail this check (see docs/DECISIONS.md, D-016).
import { readFileSync } from 'node:fs';
import path from 'node:path';

import * as fontkit from 'fontkit';

const FONTS = [
  { family: 'Inter Variable', css: '@fontsource-variable/inter/index.css' },
  { family: 'Geologica Variable', css: '@fontsource-variable/geologica/index.css' },
];

const KAZAKH_SPECIFIC = 'ӘәҒғҚқҢңӨөҰұҮүҺһІі';
const RUSSIAN = 'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдеёжзийклмнопрстуфхцчшщъыьэюя';
const LETTERS = [...new Set([...KAZAKH_SPECIFIC, ...RUSSIAN])];

const modules = path.join(import.meta.dirname, '..', 'node_modules');

function parseRanges(value) {
  return value.split(',').map((part) => {
    const [from, to] = part.trim().replace(/^U\+/i, '').split('-');
    const start = parseInt(from, 16);
    return [start, to ? parseInt(to, 16) : start];
  });
}

function faces(cssFile) {
  const css = readFileSync(path.join(modules, cssFile), 'utf8');
  const dir = path.dirname(path.join(modules, cssFile));
  return [...css.matchAll(/@font-face\s*{([^}]*)}/g)]
    .map(([, body]) => ({
      style: /font-style:\s*(\w+)/.exec(body)?.[1] ?? 'normal',
      file: path.join(dir, /url\(([^)]+\.woff2)\)/.exec(body)[1].replace(/^\.\//, '')),
      ranges: parseRanges(/unicode-range:\s*([^;]+);/.exec(body)[1]),
    }))
    .filter((face) => face.style === 'normal');
}

let failed = false;
for (const { family, css } of FONTS) {
  const list = faces(css);
  const missing = [];
  for (const letter of LETTERS) {
    const cp = letter.codePointAt(0);
    const candidates = list.filter((f) => f.ranges.some(([a, b]) => cp >= a && cp <= b)).reverse();
    if (!candidates.some((f) => fontkit.openSync(f.file).hasGlyphForCodePoint(cp))) {
      missing.push(letter);
    }
  }
  if (missing.length > 0) {
    failed = true;
    console.error(`✗ ${family}: no glyphs for ${missing.join(' ')}`);
  } else {
    console.log(
      `✓ ${family}: all ${LETTERS.length} Kazakh and Russian letters present (${KAZAKH_SPECIFIC})`,
    );
  }
}
process.exit(failed ? 1 : 0);
