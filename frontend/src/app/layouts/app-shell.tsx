import { Menu, PanelLeftClose, PanelLeftOpen, Search } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '@/design-system/components/button';
import { IconButton } from '@/design-system/components/icon-button';
import { Kbd, modKey } from '@/design-system/components/kbd';
import { LogoMark } from '@/design-system/components/logo';
import { useCommandPalette } from '@/design-system/components/command-palette';
import { Sheet, SheetContent, SheetTitle } from '@/design-system/components/sheet';
import { cn } from '@/design-system/lib/cn';

import { SidebarCollapsed } from './sidebar-context';

const COLLAPSE_KEY = 'qadam.sidebar.collapsed';

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === '1';
  } catch {
    return false;
  }
}

interface AppShellProps {
  /** NavSection / NavLink elements. */
  nav: ReactNode;
  /** Product and tenant name next to the logo. */
  brand: ReactNode;
  /** The mark next to the brand (the university's seal); the Qadam mark by default. */
  mark?: ReactNode;
  /** Bottom of the sidebar: the signed-in user. */
  user?: ReactNode;
  /** Left side of the top bar: breadcrumbs or the page title. */
  topbar?: ReactNode;
  /** Right side of the top bar: notifications, preferences, avatar. */
  actions?: ReactNode;
  children: ReactNode;
  /** Wider content for dashboards (1440px instead of 1280px). */
  wide?: boolean;
  /** Full-width content without padding, for layouts with their own columns. */
  bleed?: boolean;
}

/** Signed-in application frame: collapsible sidebar, glass top bar, content. */
export function AppShell({
  nav,
  brand,
  mark,
  user,
  topbar,
  actions,
  children,
  wide = false,
  bleed = false,
}: AppShellProps) {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(readCollapsed);
  const [mobileOpen, setMobileOpen] = useState(false);
  const openPalette = useCommandPalette((s) => s.setOpen);

  const toggleCollapsed = () => {
    setCollapsed((value) => {
      try {
        localStorage.setItem(COLLAPSE_KEY, value ? '0' : '1');
      } catch {
        // Not persisted when storage is unavailable.
      }
      return !value;
    });
  };

  const sidebar = (isCollapsed: boolean) => (
    <SidebarCollapsed value={isCollapsed}>
      <div className="flex h-full flex-col">
        <div
          className={cn(
            'flex h-16 shrink-0 items-center gap-2.5 px-4',
            isCollapsed && 'justify-center px-0',
          )}
        >
          {mark ?? <LogoMark />}
          {!isCollapsed && <div className="min-w-0 flex-1">{brand}</div>}
        </div>
        <nav
          aria-label={t('nav.main')}
          className={cn('flex-1 overflow-y-auto px-3 pb-4', isCollapsed && 'px-2')}
        >
          {nav}
        </nav>
        {user && (
          <div className={cn('border-t border-border p-3', isCollapsed && 'px-2')}>{user}</div>
        )}
      </div>
    </SidebarCollapsed>
  );

  return (
    <div className="flex min-h-dvh bg-bg">
      <aside
        className={cn(
          'sticky top-0 hidden h-dvh shrink-0 border-r border-border bg-surface transition-[width] duration-200 lg:block',
          collapsed ? 'w-18' : 'w-64',
        )}
      >
        {sidebar(collapsed)}
      </aside>

      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" size="sm" hideClose>
          <SheetTitle className="sr-only">{t('nav.main')}</SheetTitle>
          <div
            className="h-full"
            onClick={(event) => {
              if ((event.target as HTMLElement).closest('a')) setMobileOpen(false);
            }}
          >
            {sidebar(false)}
          </div>
        </SheetContent>
      </Sheet>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-2 border-b border-border surface-glass px-4 lg:px-6">
          <IconButton
            label={t('actions.openMenu')}
            icon={<Menu />}
            className="lg:hidden"
            onClick={() => {
              setMobileOpen(true);
            }}
          />
          <IconButton
            label={collapsed ? t('actions.expandSidebar') : t('actions.collapseSidebar')}
            icon={collapsed ? <PanelLeftOpen /> : <PanelLeftClose />}
            className="hidden lg:inline-flex"
            onClick={toggleCollapsed}
          />
          <div className="min-w-0 flex-1">{topbar}</div>
          <Button
            variant="secondary"
            size="sm"
            className="hidden w-56 justify-start gap-2 text-sm font-normal tracking-normal text-fg-subtle normal-case md:inline-flex"
            onClick={() => {
              openPalette(true);
            }}
          >
            <Search aria-hidden />
            <span className="flex-1 text-left font-normal">{t('actions.search')}</span>
            <Kbd>{modKey()}</Kbd>
            <Kbd>K</Kbd>
          </Button>
          <IconButton
            label={t('actions.search')}
            icon={<Search />}
            className="md:hidden"
            onClick={() => {
              openPalette(true);
            }}
          />
          {actions}
        </header>
        <main
          id="main"
          className={cn(
            'mx-auto w-full min-w-0 flex-1',
            !bleed && 'px-4 py-6 sm:px-6 lg:px-8 lg:py-8',
            !bleed && (wide ? 'max-w-wide' : 'max-w-page'),
          )}
        >
          {children}
        </main>
      </div>
    </div>
  );
}
