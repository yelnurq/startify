import { useMatches, useRouter } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { setRouteTitle } from '@/shared/hooks/use-document-meta';

import { focusMainHeading } from './route-focus';

/** The tab title from the deepest route that names its page. */
function useRouteTitle() {
  const { t } = useTranslation();
  const pageTitle = useMatches({
    select: (matches) => matches.findLast((m) => m.staticData.pageTitle)?.staticData.pageTitle,
  });
  const titleKey = useMatches({
    select: (matches) => matches.findLast((m) => m.staticData.titleKey)?.staticData.titleKey,
  });
  // Signed-in pages load the `app` catalogue in their route loader.
  const page = pageTitle
    ? t(`pageTitles.${pageTitle}`)
    : titleKey
      ? t(`titles.${titleKey}`, { ns: 'app' })
      : undefined;
  const title = page ? t('pageTitles.format', { page }) : undefined;
  useEffect(() => {
    setRouteTitle(title);
  }, [title]);
}

/**
 * Titles every page in the tab, and after moving to another page puts the
 * focus on its main heading or announces its title (WCAG 2.4.2, 2.4.3).
 */
export function RouteAnnouncer() {
  useRouteTitle();
  const router = useRouter();
  const [announcement, setAnnouncement] = useState('');
  useEffect(() => {
    let stop: () => void = () => undefined;
    const unsubscribe = router.subscribe('onResolved', (event) => {
      stop();
      // The first page load and changes of filters (search only) keep the focus.
      if (!event.fromLocation || !event.pathChanged) return;
      setAnnouncement('');
      stop = focusMainHeading(() => {
        setAnnouncement(document.title);
      });
    });
    return () => {
      stop();
      unsubscribe();
    };
  }, [router]);
  return (
    <div aria-live="polite" aria-atomic="true" className="sr-only">
      {announcement}
    </div>
  );
}
