import { describe, expect, it } from 'vitest';

import { itemDiff, wordDiff } from './diff';
import { inlineSpans, parseBlocks } from './markdown';
import { mergeStep, resolveConflicts } from './merge';
import { parseNumberInput, unitEconomics } from './numbers';
import { parseMentorContent, suggestionValue } from './suggestions';
import { changedKeys, isEmptyValue, sameValue, valueToText } from './values';

describe('values', () => {
  it('treats every blank shape as equal', () => {
    expect(isEmptyValue(undefined)).toBe(true);
    expect(isEmptyValue('  ')).toBe(true);
    expect(isEmptyValue([])).toBe(true);
    expect(isEmptyValue(0)).toBe(false);
    expect(sameValue(undefined, '')).toBe(true);
    expect(sameValue(null, [])).toBe(true);
    // Texts compare as the server stores them, trimmed.
    expect(sameValue('a', 'a ')).toBe(true);
    expect(sameValue('a', 'b')).toBe(false);
  });

  it('compares table rows whatever the key order', () => {
    expect(sameValue([{ a: '1', b: 2 }], [{ b: 2, a: '1' }])).toBe(true);
    expect(sameValue([{ a: '1' }], [{ a: '2' }])).toBe(false);
  });

  it('lists changed keys', () => {
    expect(changedKeys({ a: 'x', b: 'y', c: '' }, { a: 'x', b: 'z', d: 5 })).toEqual(['b', 'd']);
  });

  it('turns rows into readable text', () => {
    const field = {
      key: 'list',
      type: 'table' as const,
      required: false,
      label: {},
      columns: [
        { key: 'name', type: 'text' as const, required: true, label: {} },
        { key: 'price', type: 'text' as const, required: false, label: {} },
      ],
    };
    expect(valueToText([{ price: '100', name: 'Kaspi' }, { name: 'Halyk' }], field)).toBe(
      'Kaspi · 100\nHalyk',
    );
  });
});

describe('mergeStep', () => {
  const base = { a: 'one', b: 'two', c: 'three' };

  it('takes their changes and keeps mine when they touch different fields', () => {
    const mine = { ...base, a: 'ONE' };
    const theirs = { ...base, b: 'TWO', d: 'new' };
    expect(mergeStep(base, mine, theirs)).toEqual({
      merged: { a: 'ONE', b: 'TWO', c: 'three', d: 'new' },
      conflicts: [],
    });
  });

  it('reports a field both changed differently, keeping mine until resolved', () => {
    const result = mergeStep(base, { ...base, c: 'mine' }, { ...base, c: 'theirs' });
    expect(result.conflicts).toEqual(['c']);
    expect(result.merged.c).toBe('mine');
    expect(resolveConflicts(result.merged, { ...base, c: 'theirs' }, { c: 'theirs' }).c).toBe(
      'theirs',
    );
  });

  it('does not report identical changes as a conflict', () => {
    expect(mergeStep(base, { ...base, a: 'same' }, { ...base, a: 'same' }).conflicts).toEqual([]);
  });
});

describe('wordDiff', () => {
  it('marks added and removed words and rebuilds both texts', () => {
    const before = 'Студенты тратят 40 минут на поиск аудитории';
    const after = 'Студенты первого курса тратят 30 минут на поиск аудитории';
    const diff = wordDiff(before, after);
    const rebuilt = (kinds: string[]) =>
      diff
        .filter((s) => kinds.includes(s.kind))
        .map((s) => s.text)
        .join('');
    expect(rebuilt(['same', 'added'])).toBe(after);
    expect(rebuilt(['same', 'removed']).replace(/\s+/g, ' ')).toBe(before);
    expect(diff.filter((s) => s.kind === 'added').map((s) => s.text.trim())).toEqual([
      'первого курса',
      '30',
    ]);
    expect(diff.filter((s) => s.kind === 'removed').map((s) => s.text.trim())).toEqual(['40']);
  });

  it('handles empty sides', () => {
    expect(wordDiff('', 'новый текст')).toEqual([{ kind: 'added', text: 'новый текст' }]);
    expect(wordDiff('старый', '')).toEqual([{ kind: 'removed', text: 'старый' }]);
    expect(wordDiff('', '')).toEqual([]);
  });

  it('diffs list items', () => {
    expect(itemDiff(['a', 'b'], ['b', 'c'])).toEqual([
      { kind: 'removed', text: 'a' },
      { kind: 'same', text: 'b' },
      { kind: 'added', text: 'c' },
    ]);
  });
});

describe('parseMentorContent', () => {
  it('splits prose and closed suggestions', () => {
    const content =
      'Хорошее начало.\n\n<suggestion field="statement">Фермеры теряют урожай</suggestion>\n\nЧто думаете?';
    expect(parseMentorContent(content)).toEqual([
      { kind: 'text', text: 'Хорошее начало.' },
      { kind: 'suggestion', field: 'statement', text: 'Фермеры теряют урожай', complete: true },
      { kind: 'text', text: 'Что думаете?' },
    ]);
  });

  it('shows an unclosed suggestion as incomplete while streaming', () => {
    expect(parseMentorContent('Вот вариант:\n<suggestion field="uvp">Проект помо', true)).toEqual([
      { kind: 'text', text: 'Вот вариант:' },
      { kind: 'suggestion', field: 'uvp', text: 'Проект помо', complete: false },
    ]);
  });

  it('holds back a half-written tag at the end of a stream', () => {
    expect(parseMentorContent('Попробуйте так: <sugg', true)).toEqual([
      { kind: 'text', text: 'Попробуйте так:' },
    ]);
    expect(parseMentorContent('<suggestion field="uvp">Текст</sugg', true).at(-1)).toMatchObject({
      text: 'Текст',
      complete: false,
    });
  });

  it('splits list suggestions into items', () => {
    expect(suggestionValue('- Фото листа\n2. Карта очагов\n\n', 'list')).toEqual([
      'Фото листа',
      'Карта очагов',
    ]);
    expect(suggestionValue('one\ntwo', 'textarea')).toBe('one\ntwo');
  });
});

describe('light markdown', () => {
  it('parses paragraphs and lists', () => {
    expect(parseBlocks('Первый абзац\nпродолжение\n\n- раз\n- два\n1. один\nКонец')).toEqual([
      { kind: 'paragraph', text: 'Первый абзац\nпродолжение' },
      { kind: 'list', ordered: false, items: ['раз', 'два'] },
      { kind: 'list', ordered: true, items: ['один'] },
      { kind: 'paragraph', text: 'Конец' },
    ]);
  });

  it('finds bold spans and leaves markup-like text alone', () => {
    expect(inlineSpans('Это **важно**, а <b>это</b> — нет')).toEqual([
      { bold: false, text: 'Это ' },
      { bold: true, text: 'важно' },
      { bold: false, text: ', а <b>это</b> — нет' },
    ]);
  });
});

describe('numbers', () => {
  it('reads grouped and decimal input', () => {
    expect(parseNumberInput('1 500 000')).toBe(1_500_000);
    expect(parseNumberInput('1 500 000 ₸')).toBe(1_500_000);
    expect(parseNumberInput('2,5')).toBe(2.5);
    expect(parseNumberInput('')).toBeNull();
    expect(parseNumberInput('12abc')).toBeUndefined();
  });

  it('computes unit economics', () => {
    expect(unitEconomics({ price: 5000, variableCost: 2000, cac: 3000, purchases: 4 })).toEqual({
      margin: 3000,
      ltv: 12000,
      ratio: 4,
    });
    expect(unitEconomics({ price: 5000, variableCost: null, cac: 3000, purchases: 4 })).toEqual({
      margin: null,
      ltv: null,
      ratio: null,
    });
  });
});
