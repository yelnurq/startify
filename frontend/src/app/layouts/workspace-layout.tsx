import { useLocation } from '@tanstack/react-router';
import { ListTree, PanelRightClose, PanelRightOpen } from 'lucide-react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '@/design-system/components/button';
import { IconButton } from '@/design-system/components/icon-button';
import { Sheet, SheetContent, SheetTitle } from '@/design-system/components/sheet';
import { cn } from '@/design-system/lib/cn';

interface WorkspaceLayoutProps {
  /** Breadcrumbs, team presence, save status. */
  header: ReactNode;
  /** Stage and step navigation (left column). */
  nav: ReactNode;
  navTitle: string;
  /** Collapsible assistant panel (right column), e.g. the AI mentor. */
  aside: ReactNode;
  asideTitle: string;
  asideIcon: ReactNode;
  children: ReactNode;
}

/**
 * Three-column project workspace: steps on the left, the step form in the
 * centre, the AI mentor on the right. On phones both side columns open as sheets.
 */
export function WorkspaceLayout({
  header,
  nav,
  navTitle,
  aside,
  asideTitle,
  asideIcon,
  children,
}: WorkspaceLayoutProps) {
  const { t } = useTranslation();
  const [asideOpen, setAsideOpen] = useState(true);
  // Sheets remember the address they were opened at: navigating closes them.
  const { pathname } = useLocation();
  const [navSheetAt, setNavSheetAt] = useState<string | null>(null);
  const [asideSheetAt, setAsideSheetAt] = useState<string | null>(null);
  const navSheet = navSheetAt === pathname;
  const asideSheet = asideSheetAt === pathname;
  const setNavSheet = (open: boolean) => {
    setNavSheetAt(open ? pathname : null);
  };
  const setAsideSheet = (open: boolean) => {
    setAsideSheetAt(open ? pathname : null);
  };
  // A new page (another step) starts at the top of the centre column.
  const main = useRef<HTMLElement>(null);
  useEffect(() => {
    main.current?.scrollTo({ top: 0 });
  }, [pathname]);

  return (
    // The columns scroll on their own under the app header, like a desktop app.
    <div className="flex h-[calc(100dvh-4rem)] flex-col">
      <div className="flex items-center gap-2 border-b border-border bg-surface px-4 py-3 lg:px-6">
        <IconButton
          label={navTitle}
          icon={<ListTree />}
          className="lg:hidden"
          onClick={() => {
            setNavSheet(true);
          }}
        />
        <div className="min-w-0 flex-1">{header}</div>
        <Button
          variant="soft"
          size="sm"
          className="xl:hidden"
          leftIcon={asideIcon}
          onClick={() => {
            setAsideSheet(true);
          }}
        >
          {asideTitle}
        </Button>
      </div>

      <div className="flex min-h-0 flex-1">
        <aside
          aria-label={navTitle}
          className="hidden w-72 shrink-0 overflow-y-auto border-r border-border bg-surface p-3 lg:block"
        >
          {nav}
        </aside>

        <section
          ref={main}
          className="min-w-0 flex-1 overflow-y-auto px-4 pt-6 sm:px-6 lg:px-10 lg:pt-8"
        >
          <div className="mx-auto max-w-3xl pb-6 lg:pb-8">{children}</div>
        </section>

        <aside
          aria-label={asideTitle}
          className={cn(
            'hidden shrink-0 flex-col border-l border-border bg-surface transition-[width] duration-200 xl:flex',
            asideOpen ? 'w-96' : 'w-14',
          )}
        >
          <div
            className={cn(
              'flex h-12 shrink-0 items-center gap-2 border-b border-border px-3',
              !asideOpen && 'justify-center px-0',
            )}
          >
            {asideOpen && (
              <span className="flex flex-1 items-center gap-2 text-sm font-semibold text-fg [&_svg]:size-4 [&_svg]:text-brand">
                {asideIcon}
                {asideTitle}
              </span>
            )}
            <IconButton
              size="sm"
              label={asideOpen ? t('actions.collapsePanel') : t('actions.expandPanel')}
              icon={asideOpen ? <PanelRightClose /> : <PanelRightOpen />}
              tooltipSide="left"
              onClick={() => {
                setAsideOpen((v) => !v);
              }}
            />
          </div>
          {asideOpen && <div className="min-h-0 flex-1 animate-fade-in">{aside}</div>}
        </aside>
      </div>

      <Sheet open={navSheet} onOpenChange={setNavSheet}>
        <SheetContent side="left" size="sm">
          <SheetTitle className="px-5 pt-5 text-base font-semibold">{navTitle}</SheetTitle>
          <div className="min-h-0 flex-1 overflow-y-auto p-3">{nav}</div>
        </SheetContent>
      </Sheet>
      <Sheet open={asideSheet} onOpenChange={setAsideSheet}>
        <SheetContent side="right" size="md">
          <SheetTitle className="flex items-center gap-2 px-5 pt-5 text-base font-semibold [&_svg]:size-4 [&_svg]:text-brand">
            {asideIcon}
            {asideTitle}
          </SheetTitle>
          <div className="min-h-0 flex-1">{aside}</div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
