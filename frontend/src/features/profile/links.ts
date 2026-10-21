/** Only http(s) links are rendered; anything else (javascript:, data:) is dropped. */
export function safeUrl(value: string | null | undefined): string | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : null;
  } catch {
    return null;
  }
}

export const TELEGRAM = /^@?[A-Za-z0-9_]{5,32}$/;

export function telegramUrl(handle: string | null | undefined): string | null {
  if (!handle || !TELEGRAM.test(handle)) return null;
  return `https://t.me/${handle.replace(/^@/, '')}`;
}

/** The address of a public profile on this site. */
export function publicProfileUrl(slug: string): string {
  return `${window.location.origin}/u/${slug}`;
}
