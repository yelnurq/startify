import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowRight, CircleCheck, Palette, ServerCrash } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import { Badge, Button, Card, Logo, Skeleton } from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import { metaQuery } from './api';

/** Home page until the landing ships: brand, platform status, design system. */
export function StatusPage() {
  const { t } = useTranslation();
  const format = useFormatters();
  const meta = useQuery(metaQuery);

  return (
    <PublicLayout
      logo={
        <Link to="/" className="rounded-lg">
          <Logo />
        </Link>
      }
    >
      <div className="relative overflow-hidden">
        <div aria-hidden className="absolute inset-0 ornament-fade opacity-[0.06]" />
        <div
          aria-hidden
          className="absolute -top-56 left-1/2 h-[30rem] w-[60rem] -translate-x-1/2 rounded-full bg-primary-300/30 blur-3xl dark:bg-primary-600/15"
        />
        <div className="relative mx-auto flex min-h-[calc(100dvh-4rem)] max-w-page flex-col items-center justify-center gap-10 px-4 py-16 text-center sm:px-6">
          <div className="flex flex-col items-center gap-5">
            <h1 className="font-display text-5xl font-semibold text-fg sm:text-6xl">
              {t('app.name')}
            </h1>
            <p className="max-w-2xl text-lg leading-relaxed text-fg-muted sm:text-xl">
              {t('app.tagline')}
            </p>
          </div>
          <Button asChild size="lg" rightIcon={<ArrowRight />}>
            <Link to="/design">
              <Palette aria-hidden />
              {t('nav.designSystem')}
            </Link>
          </Button>

          <Card className="w-full max-w-md gap-4 p-5 text-left" aria-labelledby="status-title">
            <div className="flex items-center justify-between gap-3">
              <h2 id="status-title" className="text-sm font-medium text-fg-muted">
                {t('status.title')}
              </h2>
              {meta.isSuccess && (
                <Badge tone="success" dot>
                  {t('status.apiOnline')}
                </Badge>
              )}
            </div>
            {meta.isPending && (
              <div className="flex flex-col gap-2" aria-busy>
                <span className="sr-only">{t('status.checking')}</span>
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            )}
            {meta.isError && (
              <div role="alert" className="flex flex-col items-start gap-3">
                <p className="flex items-center gap-2 font-medium text-danger">
                  <ServerCrash className="size-4" aria-hidden />
                  {t('status.apiOffline')}
                </p>
                <p className="text-sm text-fg-muted">{errorMessage(t, meta.error)}</p>
                <Button variant="secondary" size="sm" onClick={() => void meta.refetch()}>
                  {t('status.retry')}
                </Button>
              </div>
            )}
            {meta.isSuccess && (
              <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
                <dt className="text-fg-subtle">{t('status.version')}</dt>
                <dd
                  data-testid="api-version"
                  className="flex items-center gap-1.5 font-medium text-fg"
                >
                  <CircleCheck className="size-3.5 text-success" aria-hidden />
                  {meta.data.version}
                </dd>
                <dt className="text-fg-subtle">{t('status.commit')}</dt>
                <dd className="font-mono text-fg">{meta.data.commit}</dd>
                <dt className="text-fg-subtle">{t('status.serverTime')}</dt>
                <dd className="text-fg">{format.dateTime(meta.data.server_time)}</dd>
              </dl>
            )}
          </Card>
        </div>
      </div>
    </PublicLayout>
  );
}
