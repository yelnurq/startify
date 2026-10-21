// Verifies the translation catalogues: every locale has the same keys as the
// reference (ru), exactly the plural forms its language needs, no empty values
// and the same {{interpolation}} placeholders. Exits non-zero on problems.
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const root = path.join(import.meta.dirname, '..', 'src', 'locales');
const reference = 'ru';
const locales = readdirSync(root).filter((name) => !name.startsWith('.'));
const PLURAL = /_(zero|one|two|few|many|other)$/;

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

/** Base key → { plural forms present, a sample value }. */
function group(catalog) {
  const groups = new Map();
  for (const [key, value] of catalog) {
    const base = key.replace(PLURAL, '');
    const form = PLURAL.exec(key)?.[1];
    const entry = groups.get(base) ?? { forms: new Set(), value };
    if (form) entry.forms.add(form);
    groups.set(base, entry);
  }
  return groups;
}

const catalogs = new Map(locales.map((l) => [l, load(l)]));
const base = group(catalogs.get(reference));
const problems = [];

for (const [locale, catalog] of catalogs) {
  const needed = new Intl.PluralRules(locale).resolvedOptions().pluralCategories;
  const groups = group(catalog);
  for (const key of base.keys()) {
    if (!groups.has(key)) problems.push(`${locale}: missing ${key}`);
  }
  for (const [key, { forms }] of groups) {
    if (!base.has(key)) problems.push(`${locale}: extra key ${key} (not in ${reference})`);
    const plural = forms.size > 0 || (base.get(key)?.forms.size ?? 0) > 0;
    if (plural) {
      for (const form of needed) {
        if (!forms.has(form)) problems.push(`${locale}: ${key} lacks plural form "${form}"`);
      }
    }
  }
  for (const [key, value] of catalog) {
    if (value.trim() === '') problems.push(`${locale}: empty value for ${key}`);
    const ref = base.get(key.replace(PLURAL, ''))?.value;
    if (ref !== undefined && placeholders(value) !== placeholders(ref)) {
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
