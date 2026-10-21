import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { errorMessage } from '@/shared/api/errors';
import { changeLocale, LOCALES } from '@/shared/i18n';
import { useFormatters } from '@/shared/i18n/format';

import { metaQuery } from './api';

/** Start page of the foundation phase: shows that web, API and database are wired. */
export function StatusPage() {
  const { t } = useTranslation();
  const format = useFormatters();
  const meta = useQuery(metaQuery);

  return (
    <main className="mx-auto flex min-h-dvh max-w-xl flex-col justify-center gap-8 px-6 py-16">
      <header className="flex flex-col gap-2">
        <h1 className="text-4xl font-semibold tracking-tight">{t('app.name')}</h1>
        <p className="text-lg text-slate-600">{t('app.tagline')}</p>
      </header>

      <section aria-labelledby="status-title" className="rounded-xl border border-slate-200 p-6">
        <h2 id="status-title" className="mb-4 text-sm font-medium text-slate-500">
          {t('status.title')}
        </h2>
        {meta.isPending && <p>{t('status.checking')}</p>}
        {meta.isError && (
          <div role="alert" className="flex flex-col items-start gap-3">
            <p className="font-medium text-red-700">{t('status.apiOffline')}</p>
            <p className="text-slate-600">{errorMessage(t, meta.error)}</p>
            <button
              type="button"
              onClick={() => void meta.refetch()}
              className="rounded-lg border border-slate-300 px-3 py-1.5"
            >
              {t('status.retry')}
            </button>
          </div>
        )}
        {meta.isSuccess && (
          <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
            <dt className="text-slate-500">{t('status.apiOnline')}</dt>
            <dd data-testid="api-version">{meta.data.version}</dd>
            <dt className="text-slate-500">{t('status.commit')}</dt>
            <dd className="font-mono">{meta.data.commit}</dd>
            <dt className="text-slate-500">{t('status.serverTime')}</dt>
            <dd>{format.dateTime(meta.data.server_time)}</dd>
          </dl>
        )}
      </section>

      <nav aria-label={t('language.label')} className="flex gap-2">
        {LOCALES.map((locale) => (
          <button
            key={locale}
            type="button"
            lang={locale}
            aria-pressed={format.locale === locale}
            onClick={() => void changeLocale(locale)}
            className="rounded-full border border-slate-300 px-3 py-1 text-sm aria-pressed:bg-slate-900 aria-pressed:text-white"
          >
            {t(`language.${locale}`)}
          </button>
        ))}
      </nav>
    </main>
  );
}
