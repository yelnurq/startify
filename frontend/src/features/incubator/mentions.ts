import type { MemberBrief } from '@/features/projects/api';

/** Members who can be mentioned with their display names. */
export interface Mentionable {
  id: string;
  name: string;
  avatar: string | null;
}

/** The "@name" being typed at the end of the text before the caret, if any. */
export function mentionQuery(beforeCaret: string): string | null {
  const match = /(?:^|\s)@([\p{L}\p{N}.-]*)$/u.exec(beforeCaret);
  return match ? (match[1] ?? '') : null;
}

/** Mentioned people whose "@Name" is still in the text. */
export function mentionedIds(body: string, people: Mentionable[]): string[] {
  return people.filter((p) => body.includes(`@${p.name}`)).map((p) => p.id);
}

/** Project members (without the signed-in one) as people to mention. */
export function mentionablePeople(members: { user: MemberBrief }[], meId: string): Mentionable[] {
  return members
    .filter((m) => m.user.id !== meId)
    .map((m) => ({ id: m.user.id, name: m.user.full_name, avatar: m.user.avatar_url }));
}
