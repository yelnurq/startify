/** A piece of a mentor answer: prose, or a wording proposed for a field. */
export type MentorSegment =
  | { kind: 'text'; text: string }
  | { kind: 'suggestion'; field: string; text: string; complete: boolean };

const OPEN = /<suggestion\s+field="([a-z0-9_]+)"\s*>/i;
const CLOSE = '</suggestion>';

/**
 * Splits a mentor answer into prose and suggestion blocks
 * (`<suggestion field="key">…</suggestion>`). While the answer is still
 * streaming, a block that is not closed yet is returned as incomplete, and a
 * half-written tag at the very end is held back instead of shown as text.
 */
export function parseMentorContent(content: string, streaming = false): MentorSegment[] {
  const out: MentorSegment[] = [];
  let rest = content;
  for (;;) {
    const open = OPEN.exec(rest);
    if (!open) break;
    const before = rest.slice(0, open.index);
    if (before.trim()) out.push({ kind: 'text', text: before.trim() });
    const field = (open[1] ?? '').toLowerCase();
    const body = rest.slice(open.index + open[0].length);
    const end = body.indexOf(CLOSE);
    if (end < 0) {
      const text = streaming ? holdBack(body, CLOSE) : body;
      out.push({ kind: 'suggestion', field, text: text.trim(), complete: !streaming });
      return out;
    }
    out.push({ kind: 'suggestion', field, text: body.slice(0, end).trim(), complete: true });
    rest = body.slice(end + CLOSE.length);
  }
  const tail = streaming ? holdBack(rest, '<suggestion') : rest;
  if (tail.trim()) out.push({ kind: 'text', text: tail.trim() });
  return out;
}

/** Drops an unfinished prefix of `tag` (or of an unfinished opening tag) at the end. */
function holdBack(text: string, tag: string): string {
  const lt = text.lastIndexOf('<');
  if (lt < 0) return text;
  const tail = text.slice(lt);
  if (tag.startsWith(tail) || tail.toLowerCase().startsWith('<suggestion'))
    return text.slice(0, lt);
  return text;
}

/** The value a suggestion puts into a field: list fields take one item per line. */
export function suggestionValue(text: string, type: string): string | string[] {
  if (type !== 'list') return text;
  return text
    .split('\n')
    .map((line) => line.replace(/^\s*(?:[-*•]|\d+[.)])\s*/, '').trim())
    .filter(Boolean);
}
