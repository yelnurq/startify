import 'i18next';

import type admin from '@/locales/ru/admin.json';
import type ai from '@/locales/ru/ai.json';
import type app from '@/locales/ru/app.json';
import type auth from '@/locales/ru/auth.json';
import type common from '@/locales/ru/common.json';
import type design from '@/locales/ru/design.json';
import type landing from '@/locales/ru/landing.json';
import type onboarding from '@/locales/ru/onboarding.json';
import type team from '@/locales/ru/team.json';

// Translation keys are type-checked against the Russian catalogue; the
// i18n:check script keeps kk and en in sync with it.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'common';
    resources: {
      common: typeof common;
      design: typeof design;
      auth: typeof auth;
      landing: typeof landing;
      onboarding: typeof onboarding;
      app: typeof app;
      team: typeof team;
      admin: typeof admin;
      ai: typeof ai;
    };
    returnNull: false;
  }
}
