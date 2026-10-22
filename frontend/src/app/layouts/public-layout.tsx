import { Menu } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { IconButton } from '@/design-system/components/icon-button';
import { cn } from '@/design-system/lib/cn';
import { LanguageSwitcher } from '@/design-system/components/preferences';
import { Sheet, SheetContent, SheetTitle } from '@/design-system/components/sheet';

interface PublicLayoutProps {
  /** Logo link to the home page. */
  logo: ReactNode;
  /** Header navigation links. */
  nav?: ReactNode;
  /** Sign-in and sign-up buttons. */
  actions?: ReactNode;
  /** The one action that stays in the header on phones, e.g. «Войти». */
  mobileAction?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

/** Frame of public pages: landing, showcase, competitions. */
export function PublicLayout({
  logo,
  nav,
  actions,
  mobileAction,
  footer,
  children,
}: PublicLayoutProps) {
  const { t } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  const hasMenu = Boolean(nav ?? actions);
  return (
    <div className="flex min-h-dvh flex-col bg-bg">
      <a
        href="#main"
        className="sr-only z-50 rounded-lg bg-primary px-4 py-2 text-primary-fg focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        {t('actions.skipToContent')}
      </a>
      <header className="sticky top-0 z-40 border-b border-border bg-surface">
        <div className="mx-auto flex h-16 max-w-page items-center gap-3 px-4 sm:gap-8 sm:px-6 lg:h-20">
          {logo}
          {nav && (
            <nav aria-label={t('nav.main')} className="ml-auto hidden items-center gap-7 lg:flex">
              {nav}
            </nav>
          )}
          <div className={cn('flex items-center gap-1', nav ? 'ml-auto lg:ml-0' : 'ml-auto')}>
            <LanguageSwitcher />
            <div className="ml-2 hidden items-center gap-2 sm:flex">{actions}</div>
            {mobileAction && <div className="ml-1 flex items-center sm:hidden">{mobileAction}</div>}
            {hasMenu && (
              <IconButton
                label={t('actions.openMenu')}
                icon={<Menu />}
                className={nav ? 'lg:hidden' : 'sm:hidden'}
                onClick={() => {
                  setMenuOpen(true);
                }}
              />
            )}
          </div>
        </div>
      </header>

      <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
        <SheetContent side="right" size="sm">
          <SheetTitle className="px-5 pt-5 text-base font-semibold">{t('nav.main')}</SheetTitle>
          {nav && (
            <nav
              aria-label={t('nav.main')}
              className="flex flex-col gap-1 p-3 [&_a]:flex [&_a]:h-11 [&_a]:items-center [&_a]:rounded-lg [&_a]:px-3"
              onClick={(event) => {
                if ((event.target as HTMLElement).closest('a')) setMenuOpen(false);
              }}
            >
              {nav}
            </nav>
          )}
          <div
            className="mt-auto flex flex-col gap-2 border-t border-border p-4"
            onClick={(event) => {
              if ((event.target as HTMLElement).closest('a')) setMenuOpen(false);
            }}
          >
            {actions}
          </div>
        </SheetContent>
      </Sheet>

      <main id="main" className="flex-1">
        {children}
      </main>
      {footer}
    </div>
  );
}
