import { ListTree, PanelRightClose, PanelRightOpen } from 'lucide-react';
import { useState, type ReactNode } from 'react';
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
  const [navSheet, setNavSheet] = useState(false);
  const [asideSheet, setAsideSheet] = useState(false);

  return (
    <div className="flex min-h-[calc(100dvh-4rem)] flex-col">
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
          className="sticky top-16 hidden h-[calc(100dvh-4rem)] w-72 shrink-0 overflow-y-auto border-r border-border bg-surface p-3 lg:block"
        >
          {nav}
        </aside>

        <section className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-8">
          <div className="mx-auto max-w-3xl">{children}</div>
        </section>

        <aside
          aria-label={asideTitle}
          className={cn(
            'sticky top-16 hidden h-[calc(100dvh-4rem)] shrink-0 flex-col border-l border-border bg-surface transition-[width] duration-200 xl:flex',
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
