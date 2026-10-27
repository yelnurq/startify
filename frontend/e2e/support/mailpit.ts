// Reads the letters the stack sends through Mailpit (docker compose, port 8025).
const MAILPIT_URL = process.env.MAILPIT_URL ?? 'http://localhost:8025';

interface Address {
  Name: string;
  Address: string;
}

interface MessageSummary {
  ID: string;
  To: Address[];
  Subject: string;
  Created: string;
}

export interface Message {
  ID: string;
  Subject: string;
  Text: string;
  HTML: string;
}

async function getJson<T>(path: string): Promise<T> {
  const response = await fetch(`${MAILPIT_URL}${path}`);
  if (!response.ok) throw new Error(`Mailpit ${path}: ${String(response.status)}`);
  return (await response.json()) as T;
}

/**
 * The letters already in the mailbox. Taken before an action, it lets
 * waitForMessage pick only what the action sent — independent of clock skew
 * between the test runner and the mail container.
 */
export async function mailboxMark(): Promise<Set<string>> {
  const { messages } = await getJson<{ messages: MessageSummary[] }>('/api/v1/messages?limit=200');
  return new Set(messages.map((m) => m.ID));
}

/**
 * Waits for the newest letter to `to` that arrived after the mark (and whose
 * subject matches, when given) and returns it with its text body.
 */
export async function waitForMessage(
  to: string,
  {
    since,
    subject,
    timeoutMs = 20_000,
  }: { since: Set<string>; subject?: RegExp; timeoutMs?: number },
): Promise<Message> {
  const deadline = Date.now() + timeoutMs;
  const address = to.toLowerCase();
  while (Date.now() < deadline) {
    const { messages } = await getJson<{ messages: MessageSummary[] }>(
      '/api/v1/messages?limit=200',
    );
    const match = messages
      .filter(
        (m) =>
          !since.has(m.ID) &&
          m.To.some((recipient) => recipient.Address.toLowerCase() === address) &&
          (!subject || subject.test(m.Subject)),
      )
      .sort((a, b) => new Date(b.Created).getTime() - new Date(a.Created).getTime())[0];
    if (match) return getJson<Message>(`/api/v1/message/${match.ID}`);
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`No letter to ${to} within ${String(timeoutMs)} ms`);
}

/** Path and query of the first link in the letter that leads to `pathname`. */
export function linkTo(message: Message, pathname: string): string {
  const body = `${message.Text}\n${message.HTML}`;
  const escaped = pathname.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = new RegExp(`https?://[^\\s"'<>]*${escaped}[^\\s"'<>]*`).exec(body);
  if (!match) throw new Error(`No link to ${pathname} in "${message.Subject}"`);
  const url = new URL(match[0].replaceAll('&amp;', '&'));
  return `${url.pathname}${url.search}`;
}

/** The six-digit sign-in code from the letter. */
export function codeFrom(message: Message): string {
  const match = /\b(\d{6})\b/.exec(message.Text);
  if (!match?.[1]) throw new Error(`No code in "${message.Subject}"`);
  return match[1];
}
