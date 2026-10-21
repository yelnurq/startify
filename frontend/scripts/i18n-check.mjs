// Verifies that every locale has exactly the same translation keys, no empty
// values and the same {{interpolation}} placeholders. Exits non-zero on problems.
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const root = path.join(import.meta.dirname, '..', 'src', 'locales');
const reference = 'ru';
const locales = readdirSync(root).filter((name) => !name.startsWith('.'));

function flatten(value, prefix = '', out = new Map()) {
  for (const [key, child] of Object.entries(value)) {
    const full = prefix ? `${prefix}.${key}` : key;
    if (child !== null && typeof child === 'object') flatten(child, full, out);
    else out.set(full, String(child));
  }
  return out;
}

function placeholders(text) {
  return [...text.matchAll(/\{\{\s*(\w+)[^}]*\}\}/g)]
    .map((m) => m[1])
    .sort()
    .join(',');
}

function load(locale) {
  const dir = path.join(root, locale);
  const catalog = new Map();
  for (const file of readdirSync(dir).filter((f) => f.endsWith('.json'))) {
    const ns = path.basename(file, '.json');
    const data = JSON.parse(readFileSync(path.join(dir, file), 'utf8'));
    for (const [key, value] of flatten(data)) catalog.set(`${ns}:${key}`, value);
  }
  return catalog;
}

const catalogs = new Map(locales.map((l) => [l, load(l)]));
const base = catalogs.get(reference);
const problems = [];

for (const [locale, catalog] of catalogs) {
  for (const key of base.keys()) {
    if (!catalog.has(key)) problems.push(`${locale}: missing ${key}`);
  }
  for (const [key, value] of catalog) {
    if (!base.has(key)) problems.push(`${locale}: extra key ${key} (not in ${reference})`);
    if (value.trim() === '') problems.push(`${locale}: empty value for ${key}`);
    if (base.has(key) && placeholders(value) !== placeholders(base.get(key))) {
      problems.push(`${locale}: placeholders of ${key} differ from ${reference}`);
    }
  }
}

if (problems.length > 0) {
  console.error(`i18n check failed (${problems.length} problems):\n  ${problems.join('\n  ')}`);
  process.exit(1);
}
console.log(
  `i18n check passed: ${base.size} keys × ${locales.length} locales (${locales.join(', ')})`,
);
