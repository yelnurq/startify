export type DiffKind = 'same' | 'added' | 'removed';

export interface DiffSegment {
  kind: DiffKind;
  text: string;
}

/** Words together with the whitespace that follows them, so the text rebuilds exactly. */
export function tokenize(text: string): string[] {
  return text.match(/\s+|[^\s]+\s*/g) ?? [];
}

// Beyond this many tokens a side the quadratic table is too large: show a replacement.
const MAX_TOKENS = 2000;

function push(out: DiffSegment[], kind: DiffKind, text: string) {
  const last = out[out.length - 1];
  if (last?.kind === kind) last.text += text;
  else out.push({ kind, text });
}

/** Word-level difference between two texts (longest common subsequence). */
export function wordDiff(before: string, after: string): DiffSegment[] {
  if (before === after) return before ? [{ kind: 'same', text: before }] : [];
  const a = tokenize(before);
  const b = tokenize(after);
  const out: DiffSegment[] = [];
  if (a.length > MAX_TOKENS || b.length > MAX_TOKENS) {
    if (before) out.push({ kind: 'removed', text: before });
    if (after) out.push({ kind: 'added', text: after });
    return out;
  }
  // Compare words without their trailing spaces, so "word" and "word " match.
  const key = (token: string) => token.trimEnd();
  const rows = a.length + 1;
  const cols = b.length + 1;
  const lcs = new Uint32Array(rows * cols);
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i * cols + j] =
        key(a[i] ?? '') === key(b[j] ?? '')
          ? (lcs[(i + 1) * cols + j + 1] ?? 0) + 1
          : Math.max(lcs[(i + 1) * cols + j] ?? 0, lcs[i * cols + j + 1] ?? 0);
    }
  }
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    const ta = a[i] ?? '';
    const tb = b[j] ?? '';
    if (key(ta) === key(tb)) {
      push(out, 'same', tb);
      i++;
      j++;
    } else if ((lcs[(i + 1) * cols + j] ?? 0) >= (lcs[i * cols + j + 1] ?? 0)) {
      push(out, 'removed', ta);
      i++;
    } else {
      push(out, 'added', tb);
      j++;
    }
  }
  for (; i < a.length; i++) push(out, 'removed', a[i] ?? '');
  for (; j < b.length; j++) push(out, 'added', b[j] ?? '');
  return out;
}

/** Item-level difference of two lists (lines of a list or table). */
export function itemDiff(before: string[], after: string[]): DiffSegment[] {
  const out: DiffSegment[] = [];
  const kept = new Set(before.filter((item) => after.includes(item)));
  for (const item of before) if (!kept.has(item)) out.push({ kind: 'removed', text: item });
  for (const item of after) out.push({ kind: kept.has(item) ? 'same' : 'added', text: item });
  return out;
}
