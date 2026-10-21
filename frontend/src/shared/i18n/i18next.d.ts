import 'i18next';

import type common from '@/locales/ru/common.json';
import type design from '@/locales/ru/design.json';

// Translation keys are type-checked against the Russian catalogue; the
// i18n:check script keeps kk and en in sync with it.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'common';
    resources: {
      common: typeof common;
      design: typeof design;
    };
    returnNull: false;
  }
}
