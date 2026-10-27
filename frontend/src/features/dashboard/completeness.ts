import type { Me } from '@/features/profile/api';

export type ChecklistKey = 'email' | 'avatar' | 'headline' | 'bio' | 'skills' | 'links' | 'public';

/** What makes a profile useful to teammates and mentors. */
export function profileChecklist(me: Me): { key: ChecklistKey; done: boolean }[] {
  const { user, profile } = me;
  const links = Object.values(profile.links).some((link) => Boolean(link));
  return [
    { key: 'email', done: user.email_verified },
    { key: 'avatar', done: Boolean(user.avatar_url) },
    { key: 'headline', done: Boolean(profile.headline?.trim()) },
    { key: 'bio', done: Boolean(profile.bio?.trim()) },
    { key: 'skills', done: profile.skills.length >= 3 },
    { key: 'links', done: links },
    { key: 'public', done: profile.is_public },
  ];
}
