import { Menu } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { IconButton } from '@/design-system/components/icon-button';
import { LanguageSwitcher, ThemeToggle } from '@/design-system/components/preferences';
import { Sheet, SheetContent, SheetTitle } from '@/design-system/components/sheet';

interface PublicLayoutProps {
  /** Logo link to the home page. */
  logo: ReactNode;
  /** Header navigation links. */
  nav?: ReactNode;
  /** Sign-in and sign-up buttons. */
  actions?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

/** Frame of public pages: landing, showcase, competitions. */
export function PublicLayout({ logo, nav, actions, footer, children }: PublicLayoutProps) {
  const { t } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="flex min-h-dvh flex-col bg-bg">
      <a
        href="#main"
        className="sr-only z-50 rounded-lg bg-primary px-4 py-2 text-primary-fg focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        {t('actions.skipToContent')}
      </a>
      <header className="sticky top-0 z-40 border-b border-border/70 surface-glass">
        <div className="mx-auto flex h-16 max-w-page items-center gap-6 px-4 sm:px-6">
          {logo}
          {nav && (
            <nav aria-label={t('nav.main')} className="hidden items-center gap-1 md:flex">
              {nav}
            </nav>
          )}
          <div className="ml-auto flex items-center gap-1">
            <LanguageSwitcher />
            <ThemeToggle />
            <div className="ml-2 hidden items-center gap-2 sm:flex">{actions}</div>
            {nav && (
              <IconButton
                label={t('actions.openMenu')}
                icon={<Menu />}
                className="md:hidden"
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
          <nav
            aria-label={t('nav.main')}
            className="flex flex-col gap-1 p-3 [&_a]:flex [&_a]:h-11 [&_a]:items-center [&_a]:rounded-lg [&_a]:px-3"
            onClick={(event) => {
              if ((event.target as HTMLElement).closest('a')) setMenuOpen(false);
            }}
          >
            {nav}
          </nav>
          <div className="mt-auto flex flex-col gap-2 border-t border-border p-4">{actions}</div>
        </SheetContent>
      </Sheet>

      <main id="main" className="flex-1">
        {children}
      </main>
      {footer}
    </div>
  );
}
