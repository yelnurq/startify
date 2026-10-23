import { useRouter, type ErrorComponentProps } from '@tanstack/react-router';
import { RefreshCw, TriangleAlert } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '@/design-system/components/button';
import { errorMessage } from '@/shared/api/errors';
import { useDocumentMeta } from '@/shared/hooks/use-document-meta';

import { isChunkLoadError } from './chunk-reload';

/**
 * What a screen shows when it cannot be drawn: its data failed to load (the
 * server answered 5xx, the network is gone) or its code is missing after a
 * deployment. Lives in the entry chunk, so it renders whatever else fails.
 */
export function RouteErrorPage({ error, reset }: ErrorComponentProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const outdated = isChunkLoadError(error);
  const title = outdated ? t('errorPage.outdatedTitle') : t('errorPage.title');
  useDocumentMeta(t('pageTitles.format', { page: title }));

  useEffect(() => {
    console.error(error);
  }, [error]);

  const retry = () => {
    // Missing code comes only with a fresh page; data is loaded again in place.
    if (outdated) {
      window.location.reload();
      return;
    }
    reset();
    void router.invalidate();
  };

  return (
    <div className="flex min-h-[60dvh] items-center justify-center px-4 py-12">
      <div className="flex max-w-md flex-col items-center gap-5 text-center">
        <span className="grid size-14 place-items-center rounded-full bg-danger-soft text-danger-soft-fg">
          <TriangleAlert className="size-7" aria-hidden />
        </span>
        <div role="alert" className="flex flex-col gap-2">
          <h1 className="font-display text-2xl font-bold text-balance text-ink">{title}</h1>
          <p className="text-fg-muted">
            {outdated ? t('errorPage.outdatedText') : errorMessage(error)}
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Button leftIcon={<RefreshCw />} onClick={retry}>
            {t('actions.retry')}
          </Button>
          <Button asChild variant="secondary">
            <a href="/">{t('actions.toHome')}</a>
          </Button>
        </div>
      </div>
    </div>
  );
}
