import { useQuery } from '@tanstack/react-query';
import { Outlet, useNavigate } from '@tanstack/react-router';
import { Globe, Home } from 'lucide-react';
import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import {
  CommandPalette,
  Toaster,
  TooltipProvider,
  useRegisterCommands,
  type PaletteCommand,
} from '@/design-system';
import { tenantQuery } from '@/features/auth/api';
import { changeLocale, LOCALES } from '@/shared/i18n';

import { RouteAnnouncer } from './route-announcer';

/** Global commands available on every screen of the palette. */
function useGlobalCommands(): PaletteCommand[] {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return useMemo(() => {
    const go = t('commandPalette.groups.navigation');
    const prefs = t('commandPalette.groups.preferences');
    return [
      {
        id: 'nav.home',
        group: go,
        label: t('nav.home'),
        icon: <Home />,
        perform: () => void navigate({ to: '/' }),
      },
      ...LOCALES.map((locale) => ({
        id: `locale.${locale}`,
        group: prefs,
        label: t(`language.${locale}`),
        hint: t('language.label'),
        icon: <Globe />,
        perform: () => void changeLocale(locale),
      })),
    ];
  }, [t, navigate]);
}

/** The browser tab shows the university's seal when it has one. */
function useUniversityIcon() {
  const tenant = useQuery(tenantQuery);
  const logo = tenant.data?.logo_url;
  useEffect(() => {
    if (!logo) return;
    for (const rel of ['icon', 'apple-touch-icon']) {
      let link = document.head.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`);
      if (!link) {
        link = document.createElement('link');
        link.rel = rel;
        document.head.append(link);
      }
      link.removeAttribute('type');
      link.href = logo;
    }
  }, [logo]);
}

export function RootLayout() {
  useRegisterCommands(useGlobalCommands());
  useUniversityIcon();
  return (
    <TooltipProvider delayDuration={250} skipDelayDuration={150}>
      <Outlet />
      <CommandPalette />
      <Toaster />
      <RouteAnnouncer />
    </TooltipProvider>
  );
}
