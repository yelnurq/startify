import type { ReactNode } from 'react';

import { LanguageSwitcher, ThemeToggle } from '@/design-system/components/preferences';

interface AuthLayoutProps {
  logo: ReactNode;
  /** Brand panel shown next to the form on large screens. */
  aside: ReactNode;
  children: ReactNode;
}

/** Sign-in, sign-up and recovery screens: the form and a brand panel. */
export function AuthLayout({ logo, aside, children }: AuthLayoutProps) {
  return (
    <div className="grid min-h-dvh grid-cols-1 bg-bg lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)]">
      <div className="flex flex-col px-5 py-6 sm:px-10">
        <div className="flex items-center justify-between">
          {logo}
          <div className="flex items-center gap-1">
            <LanguageSwitcher />
            <ThemeToggle />
          </div>
        </div>
        <main
          id="main"
          className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center py-10"
        >
          {children}
        </main>
      </div>
      <aside className="relative hidden overflow-hidden bg-linear-to-br from-primary-600 via-primary-700 to-primary-950 lg:block">
        <div aria-hidden className="absolute inset-0 ornament opacity-10 [--qd-brand:#fff]" />
        <div
          aria-hidden
          className="absolute -top-32 -right-24 size-[28rem] rounded-full bg-primary-300/30 blur-3xl"
        />
        <div
          aria-hidden
          className="absolute -bottom-40 -left-20 size-[26rem] rounded-full bg-gold-400/20 blur-3xl"
        />
        <div className="relative flex h-full flex-col justify-end p-12 text-white">{aside}</div>
      </aside>
    </div>
  );
}
