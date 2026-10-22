import type { ReactNode } from 'react';

import { EdgeIllustration } from '@/design-system/components/edge-illustration';
import { LanguageSwitcher, ThemeToggle } from '@/design-system/components/preferences';
import type { IllustrationName } from '@/design-system/illustrations';

interface AuthLayoutProps {
  /** Logo for the top bar on phones, where the brand panel is hidden. */
  logo: ReactNode;
  /** Link back to the site above the form on large screens. */
  back?: ReactNode;
  /** Brand panel next to the form on large screens. */
  aside: ReactNode;
  /** The drawing that peeks out of the corner of the form side. */
  art?: IllustrationName;
  children: ReactNode;
}

/** Sign-in, sign-up and recovery screens: a navy brand panel and the form on white. */
export function AuthLayout({ logo, back, aside, art = 'shield', children }: AuthLayoutProps) {
  return (
    <div className="grid min-h-dvh grid-cols-1 bg-surface lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
      <aside className="relative hidden overflow-hidden panel-ink lg:block">
        <div aria-hidden className="absolute inset-0 ornament opacity-[0.04] [--qd-brand:#fff]" />
        <div
          aria-hidden
          className="absolute -right-32 -bottom-40 size-[30rem] rounded-full bg-primary-500/20 blur-3xl"
        />
        <div className="relative flex h-full flex-col p-10 xl:p-14">{aside}</div>
      </aside>
      <div className="relative flex min-w-0 flex-col overflow-hidden">
        <EdgeIllustration
          name={art}
          placement="bottom-right"
          className="hidden w-64 -rotate-6 lg:block"
        />
        <div className="flex h-16 items-center justify-between gap-4 border-b border-border px-5 sm:px-10 lg:h-18">
          <div className="min-w-0 lg:hidden">{logo}</div>
          <div className="hidden lg:block">{back}</div>
          <div className="flex items-center gap-1">
            <LanguageSwitcher />
            <ThemeToggle />
          </div>
        </div>
        <main
          id="main"
          className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center px-5 py-12 sm:px-0"
        >
          {children}
        </main>
      </div>
    </div>
  );
}
