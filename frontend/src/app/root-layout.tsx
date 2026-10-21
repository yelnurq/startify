import { Outlet, useNavigate } from '@tanstack/react-router';
import { Home, Languages, Monitor, Moon, Palette, Sun } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import {
  CommandPalette,
  Toaster,
  TooltipProvider,
  useRegisterCommands,
  useTheme,
  type PaletteCommand,
} from '@/design-system';
import { changeLocale, LOCALES } from '@/shared/i18n';

/** Global commands available on every screen of the palette. */
function useGlobalCommands(): PaletteCommand[] {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setTheme = useTheme((s) => s.setPreference);
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
      {
        id: 'nav.design',
        group: go,
        label: t('nav.designSystem'),
        icon: <Palette />,
        perform: () => void navigate({ to: '/design' }),
      },
      {
        id: 'theme.light',
        group: prefs,
        label: t('theme.switchToLight'),
        icon: <Sun />,
        perform: () => {
          setTheme('light');
        },
      },
      {
        id: 'theme.dark',
        group: prefs,
        label: t('theme.switchToDark'),
        icon: <Moon />,
        perform: () => {
          setTheme('dark');
        },
      },
      {
        id: 'theme.system',
        group: prefs,
        label: t('theme.switchToSystem'),
        icon: <Monitor />,
        perform: () => {
          setTheme('system');
        },
      },
      ...LOCALES.map((locale) => ({
        id: `locale.${locale}`,
        group: prefs,
        label: t(`language.${locale}`),
        hint: t('language.label'),
        icon: <Languages />,
        perform: () => void changeLocale(locale),
      })),
    ];
  }, [t, navigate, setTheme]);
}

export function RootLayout() {
  useRegisterCommands(useGlobalCommands());
  return (
    <TooltipProvider delayDuration={250} skipDelayDuration={150}>
      <Outlet />
      <CommandPalette />
      <Toaster />
    </TooltipProvider>
  );
}
