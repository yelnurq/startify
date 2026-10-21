import { useTranslation } from 'react-i18next';

import { LogoMark } from '@/design-system';

export function DesignFooter() {
  const { t } = useTranslation(['design', 'common']);
  return (
    <footer className="border-t border-border bg-surface">
      <div className="mx-auto flex max-w-page flex-col gap-4 px-4 py-8 text-sm text-fg-subtle sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div className="flex items-center gap-3">
          <LogoMark className="size-6" />
          <span>{t('design:footer.copyright', { year: 2026 })}</span>
        </div>
        <span>{t('design:footer.madeIn')}</span>
      </div>
    </footer>
  );
}
