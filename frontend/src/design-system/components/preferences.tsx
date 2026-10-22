import { Globe } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { changeLocale, isLocale, LOCALES, type Locale } from '@/shared/i18n';

import { Button } from './button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from './dropdown-menu';
import { Tooltip } from './tooltip';

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const current: Locale = isLocale(i18n.language) ? i18n.language : 'kk';
  return (
    <DropdownMenu>
      <Tooltip content={t('language.label')}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-1.5 px-2.5">
            <Globe aria-hidden />
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
