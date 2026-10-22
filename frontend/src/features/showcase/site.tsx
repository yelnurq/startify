import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import { Badge, Button, Logo } from '@/design-system';
import { sessionQuery } from '@/features/auth/api';
import { metaQuery } from '@/features/system/api';

const YEAR = new Date().getFullYear();

/** Look of a header navigation link. */
export const siteNavClass =
  'rounded-lg px-3 py-2 text-sm font-medium text-fg-muted transition-colors hover:bg-surface-raised hover:text-fg aria-[current=page]:bg-surface-raised aria-[current=page]:text-fg';

interface SiteFrameProps {
  /** Links of the page before the showcase link (landing anchors). */
  nav?: ReactNode;
  children: ReactNode;
}

/** The public site: the landing page and the showcase share its header and footer. */
export function SiteFrame({ nav, children }: SiteFrameProps) {
  const { t } = useTranslation('showcase');
  const session = useQuery(sessionQuery);
  const signedIn = Boolean(session.data);
  return (
    <PublicLayout
      logo={
        <Link to="/" className="shrink-0 rounded-lg">
          <Logo />
        </Link>
      }
      nav={
        <>
          {nav}
          <Link to="/showcase" className={siteNavClass}>
            {t('site.showcase')}
          </Link>
        </>
      }
      actions={
        signedIn ? (
          <Button asChild size="sm">
            <Link to="/app">{t('site.toApp')}</Link>
          </Button>
        ) : (
          <>
            <Button asChild variant="ghost" size="sm">
              <Link to="/login">{t('site.signIn')}</Link>
            </Button>
            <Button asChild size="sm">
              <Link to="/register">{t('site.start')}</Link>
            </Button>
          </>
        )
      }
      mobileAction={
        <Button asChild size="sm" variant={signedIn ? 'primary' : 'secondary'}>
          <Link to={signedIn ? '/app' : '/login'}>
            {signedIn ? t('site.toApp') : t('site.signIn')}
          </Link>
        </Button>
      }
      footer={<SiteFooter />}
    >
      {children}
    </PublicLayout>
  );
}

function ApiStatus() {
  const { t } = useTranslation('showcase');
  const meta = useQuery(metaQuery);
  if (meta.isPending) {
    return (
      <Badge tone="neutral" dot>
        {t('site.apiChecking')}
      </Badge>
    );
  }
  if (meta.isError) {
    return (
      <Badge tone="danger" dot>
        {t('site.apiOffline')}
      </Badge>
    );
  }
  return (
    <Badge tone="success" dot>
      {t('site.api')}
      <span data-testid="api-version" className="ml-1 font-mono tabular-nums">
        {meta.data.version}
      </span>
    </Badge>
  );
}

function SiteFooter() {
  const { t } = useTranslation(['showcase', 'common']);
  return (
    <footer className="border-t border-border bg-surface">
      <div className="mx-auto flex max-w-page flex-col gap-6 px-4 py-10 sm:px-6 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-col gap-2">
          <Logo />
          <p className="max-w-sm text-sm text-fg-subtle">{t('common:app.tagline')}</p>
        </div>
        <nav
          aria-label={t('site.footerLabel')}
          className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm"
        >
          <Link to="/showcase" className="text-fg-muted hover:text-fg">
            {t('site.showcase')}
          </Link>
          <Link to="/login" className="text-fg-muted hover:text-fg">
            {t('site.signIn')}
          </Link>
          <Link to="/register" className="text-fg-muted hover:text-fg">
            {t('site.register')}
          </Link>
          <Link to="/design" className="text-fg-muted hover:text-fg">
            {t('common:nav.designSystem')}
          </Link>
        </nav>
        <div className="flex flex-col items-start gap-2 md:items-end">
          <ApiStatus />
          <p className="text-xs text-fg-subtle">{t('site.rights', { year: YEAR })}</p>
        </div>
      </div>
    </footer>
  );
}
