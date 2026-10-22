import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import { Badge, Button, Logo } from '@/design-system';
import { sessionQuery, tenantQuery } from '@/features/auth/api';
import { metaQuery } from '@/features/system/api';
import { useLocalized } from '@/shared/i18n/localized';

const YEAR = new Date().getFullYear();

/** Look of a header navigation link: navy text, a teal rule under the current page. */
export const siteNavClass =
  'relative py-2 text-sm font-semibold text-ink transition-colors hover:text-primary-text after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-primary after:opacity-0 after:transition-opacity aria-[current=page]:after:opacity-100';

/** The university's full name for the caption under the logo. */
function useUniversityName() {
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  return tenant.data ? text(tenant.data.name) : undefined;
}

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
  const university = useUniversityName();
  return (
    <PublicLayout
      logo={
        <Link to="/" className="min-w-0 shrink-0">
          <Logo caption={university} />
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
          <Button asChild>
            <Link to="/app">{t('site.toApp')}</Link>
          </Button>
        ) : (
          <>
            <Button asChild variant="secondary">
              <Link to="/login">{t('site.signIn')}</Link>
            </Button>
            <Button asChild>
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

const footerLink = 'text-sm text-white/75 transition-colors hover:text-white';

function FooterColumn({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-4">
      <h2 className="border-b border-white/15 pb-4 text-sm font-bold tracking-label text-white uppercase">
        {title}
      </h2>
      <div className="flex flex-col items-start gap-3">{children}</div>
    </div>
  );
}

function SiteFooter() {
  const { t } = useTranslation(['showcase', 'common', 'landing']);
  const university = useUniversityName();
  return (
    <footer className="border-t-3 border-primary-500 panel-ink">
      <div className="mx-auto grid max-w-page grid-cols-1 gap-10 px-4 py-14 sm:px-6 md:grid-cols-[1.4fr_1fr_1fr]">
        <div className="flex flex-col gap-5">
          <Logo inverted caption={university} />
          <p className="max-w-sm text-sm leading-relaxed text-white/75">
            {t('common:app.tagline')}
          </p>
        </div>
        <nav aria-label={t('site.footerLabel')} className="contents">
          <FooterColumn title={t('site.footerPlatform')}>
            <Link to="/showcase" className={footerLink}>
              {t('site.showcase')}
            </Link>
            <a href="/#how" className={footerLink}>
              {t('landing:nav.how')}
            </a>
            <Link to="/register" className={footerLink}>
              {t('site.register')}
            </Link>
            <Link to="/login" className={footerLink}>
              {t('site.signIn')}
            </Link>
          </FooterColumn>
          <FooterColumn title={t('site.footerResources')}>
            <a href="/#universities" className={footerLink}>
              {t('landing:nav.universities')}
            </a>
            <a href="/#partners" className={footerLink}>
              {t('landing:nav.partners')}
            </a>
            <Link to="/design" className={footerLink}>
              {t('common:nav.designSystem')}
            </Link>
          </FooterColumn>
        </nav>
      </div>
      <div className="border-t border-white/10">
        <div className="mx-auto flex max-w-page flex-wrap items-center justify-between gap-4 px-4 py-6 sm:px-6">
          <p className="text-xs text-white/70">{t('site.rights', { year: YEAR })}</p>
          <ApiStatus />
        </div>
      </div>
    </footer>
  );
}
