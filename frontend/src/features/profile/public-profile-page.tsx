import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { BriefcaseBusiness, Building2, CodeXml, Globe, Send, UsersRound } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import {
  Avatar,
  Badge,
  Button,
  Card,
  EmptyState,
  Logo,
  Skeleton,
  SkeletonText,
  Tag,
} from '@/design-system';
import { sessionQuery } from '@/features/auth/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import { publicProfileQuery, type PublicProfile } from './api';
import { safeUrl, telegramUrl } from './links';

/** A member's profile as anyone on the internet sees it (`/u/:slug`). */
export function PublicProfilePage({ slug }: { slug: string }) {
  const { t } = useTranslation(['app', 'common']);
  const profile = useQuery(publicProfileQuery(slug));
  const session = useQuery(sessionQuery);
  const notFound = profile.error instanceof ApiError && profile.error.status === 404;

  return (
    <PublicLayout
      logo={
        <Link to="/" className="rounded-lg">
          <Logo />
        </Link>
      }
      actions={
        session.data ? (
          <Button asChild size="sm">
            <Link to="/app">{t('publicProfile.toApp')}</Link>
          </Button>
        ) : (
          <Button asChild size="sm" variant="secondary">
            <Link to="/login">{t('common:actions.signIn')}</Link>
          </Button>
        )
      }
    >
      <div className="mx-auto flex max-w-3xl flex-col gap-6 px-4 py-10 sm:px-6 sm:py-16">
        {profile.isPending && (
          <Card className="gap-6 p-6 sm:p-8">
            <div className="flex items-center gap-4">
              <Skeleton className="size-20 rounded-full" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-6 w-1/2" />
                <Skeleton className="h-4 w-1/3" />
              </div>
            </div>
            <SkeletonText lines={4} />
          </Card>
        )}
        {profile.isError && (
          <EmptyState
            illustration={notFound ? 'search' : 'error'}
            title={notFound ? t('publicProfile.notFound') : t('publicProfile.failed')}
            description={
              notFound ? t('common:errors.profile.not_found') : errorMessage(profile.error)
            }
            action={
              <Button asChild variant="secondary">
                <Link to="/">{t('common:notFound.back')}</Link>
              </Button>
            }
          />
        )}
        {profile.isSuccess && <ProfileView profile={profile.data} />}
      </div>
    </PublicLayout>
  );
}

function ProfileView({ profile }: { profile: PublicProfile }) {
  const { t } = useTranslation('app');
  const text = useLocalized();
  const links: { key: string; href: string | null; icon: ReactNode; label: string }[] = [
    {
      key: 'website',
      href: safeUrl(profile.links.website),
      icon: <Globe />,
      label: t('profile.fields.website'),
    },
    {
      key: 'github',
      href: safeUrl(profile.links.github),
      icon: <CodeXml />,
      label: t('profile.fields.github'),
    },
    {
      key: 'linkedin',
      href: safeUrl(profile.links.linkedin),
      icon: <BriefcaseBusiness />,
      label: t('profile.fields.linkedin'),
    },
    {
      key: 'telegram',
      href: telegramUrl(profile.links.telegram),
      icon: <Send />,
      label: t('profile.fields.telegram'),
    },
  ];
  const visibleLinks = links.filter((link) => link.href);

  return (
    <Card className="overflow-hidden">
      <div
        aria-hidden
        className="h-28 bg-linear-to-br from-primary-400 via-primary-600 to-primary-900"
      />
      <div className="flex flex-col gap-6 px-6 pb-8 sm:px-8">
        <div className="-mt-10 flex items-end justify-between gap-4">
          <Avatar
            name={profile.full_name}
            src={profile.avatar_url}
            size="xl"
            className="rounded-full ring-4 ring-surface"
          />
          {profile.looking_for_team && (
            <Badge tone="success" size="lg" icon={<UsersRound />}>
              {t('publicProfile.lookingForTeam')}
            </Badge>
          )}
        </div>
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-2xl font-bold text-ink sm:text-3xl">
            {profile.full_name}
          </h1>
          {profile.headline && <p className="text-fg-muted">{profile.headline}</p>}
        </div>

        <div className="flex items-start gap-2 text-sm text-fg-subtle">
          <Building2 className="mt-0.5 size-4 shrink-0" aria-hidden />
          <div className="flex flex-col gap-0.5">
            <span>{text(profile.tenant.name)}</span>
            {profile.faculty && <span>{text(profile.faculty.name)}</span>}
          </div>
        </div>

        {profile.bio && (
          <section className="flex flex-col gap-2" aria-labelledby="bio-title">
            <h2 id="bio-title" className="text-sm font-semibold text-ink">
              {t('publicProfile.about')}
            </h2>
            <p className="leading-relaxed whitespace-pre-line text-fg-muted">{profile.bio}</p>
          </section>
        )}

        {profile.skills.length > 0 && (
          <TagSection id="skills" title={t('profile.fields.skills')} tags={profile.skills} />
        )}
        {profile.interests.length > 0 && (
          <TagSection
            id="interests"
            title={t('profile.fields.interests')}
            tags={profile.interests}
          />
        )}

        {visibleLinks.length > 0 && (
          <section className="flex flex-col gap-2" aria-labelledby="links-title">
            <h2 id="links-title" className="text-sm font-semibold text-ink">
              {t('publicProfile.links')}
            </h2>
            <ul className="flex flex-wrap gap-2">
              {visibleLinks.map((link) => (
                <li key={link.key}>
                  <Button asChild size="sm" variant="secondary">
                    <a
                      href={link.href ?? undefined}
                      target="_blank"
                      rel="noopener noreferrer nofollow"
                    >
                      {link.icon}
                      {link.label}
                    </a>
                  </Button>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </Card>
  );
}

function TagSection({ id, title, tags }: { id: string; title: string; tags: string[] }) {
  return (
    <section className="flex flex-col gap-2" aria-labelledby={`${id}-title`}>
      <h2 id={`${id}-title`} className="text-sm font-semibold text-ink">
        {title}
      </h2>
      <ul className="flex flex-wrap gap-1.5">
        {tags.map((tag) => (
          <li key={tag}>
            <Tag>{tag}</Tag>
          </li>
        ))}
      </ul>
    </section>
  );
}

const publicProfileRoute = getRouteApi('/u/$slug');

export function PublicProfileRoute() {
  const { slug } = publicProfileRoute.useParams();
  return <PublicProfilePage key={slug} slug={slug} />;
}
