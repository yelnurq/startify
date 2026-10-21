import { Languages, Monitor, Moon, Sun } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { changeLocale, isLocale, LOCALES, type Locale } from '@/shared/i18n';

import { useTheme, type ThemePreference } from '../theme';

import { Button } from './button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from './dropdown-menu';
import { Tooltip } from './tooltip';

const themeIcons = { light: Sun, dark: Moon, system: Monitor } as const;

export function ThemeToggle() {
  const { t } = useTranslation();
  const preference = useTheme((s) => s.preference);
  const resolved = useTheme((s) => s.resolved);
  const setPreference = useTheme((s) => s.setPreference);
  const Current = resolved === 'dark' ? Moon : Sun;
  return (
    <DropdownMenu>
      <Tooltip content={t('theme.label')}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t('theme.label')}>
            <Current aria-hidden />
          </Button>
        </DropdownMenuTrigger>
      </Tooltip>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={preference}
          onValueChange={(value) => {
            setPreference(value as ThemePreference);
          }}
        >
          {(['light', 'dark', 'system'] as const).map((option) => {
            const Icon = themeIcons[option];
            return (
              <DropdownMenuRadioItem key={option} value={option} icon={<Icon aria-hidden />}>
                {t(`theme.${option}`)}
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const current: Locale = isLocale(i18n.language) ? i18n.language : 'kk';
  return (
    <DropdownMenu>
      <Tooltip content={t('language.label')}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-1.5 px-2.5">
            <Languages aria-hidden />
            <span className="sr-only">{t('language.label')}</span>
            <span className="uppercase">{current}</span>
          </Button>
        </DropdownMenuTrigger>
      </Tooltip>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={current}
          onValueChange={(value) => {
            if (isLocale(value)) void changeLocale(value);
          }}
        >
          {LOCALES.map((locale) => (
            <DropdownMenuRadioItem key={locale} value={locale} lang={locale}>
              {t(`language.${locale}`)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
