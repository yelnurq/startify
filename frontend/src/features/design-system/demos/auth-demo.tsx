import { Link } from '@tanstack/react-router';
import { ArrowLeft, Check, Eye, EyeOff, KeyRound, Mail } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { AuthLayout } from '@/app/layouts/auth-layout';
import { Button, Field, Input, Kicker, Logo, Separator } from '@/design-system';

export function AuthDemo() {
  const { t } = useTranslation(['design', 'common']);
  const [visible, setVisible] = useState(false);
  return (
    <AuthLayout
      logo={
        <Link to="/design" className="rounded-lg">
          <Logo />
        </Link>
      }
      back={
        <Link
          to="/design"
          className="inline-flex items-center gap-2 text-sm font-semibold text-ink hover:text-primary-text"
        >
          <ArrowLeft className="size-4" aria-hidden />
          {t('common:actions.toHome')}
        </Link>
      }
      aside={
        <div className="flex h-full flex-col gap-10 text-white">
          <Logo inverted caption={t('demo.auth.cardValue')} />
          <div className="my-auto flex max-w-lg flex-col gap-6">
            <Kicker tone="light">{t('demo.auth.cardEyebrow')}</Kicker>
            <h2 className="font-display text-5xl leading-tight font-extrabold">
              {t('demo.auth.headline')}
            </h2>
            <p className="text-white/75">{t('demo.auth.cardText')}</p>
          </div>
          <ul className="flex max-w-lg flex-col gap-4 border-t border-white/15 pt-8 text-sm text-white/85">
            {(['point1', 'point2', 'point3'] as const).map((key) => (
              <li key={key} className="flex items-start gap-3">
                <Check className="mt-0.5 size-4 shrink-0 text-primary-300" aria-hidden />
                {t(`demo.auth.${key}`)}
              </li>
            ))}
          </ul>
        </div>
      }
    >
      <div className="flex flex-col gap-8">
        <div className="flex flex-col gap-2">
          <h1 className="font-display text-4xl font-extrabold text-ink">{t('demo.auth.title')}</h1>
          <p className="text-fg-muted">{t('demo.auth.subtitle')}</p>
        </div>
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault();
          }}
        >
          <Field label={t('demo.auth.email')}>
            <Input
              type="email"
              autoComplete="email"
              placeholder={t('demo.auth.emailPlaceholder')}
              leading={<Mail />}
            />
          </Field>
          <Field label={t('demo.auth.password')}>
            <Input
              type={visible ? 'text' : 'password'}
              autoComplete="current-password"
              trailing={
                <button
                  type="button"
                  aria-label={visible ? t('demo.auth.hidePassword') : t('demo.auth.showPassword')}
                  className="grid size-8 place-items-center rounded-md text-fg-subtle hover:text-fg"
                  onClick={() => {
                    setVisible((v) => !v);
                  }}
                >
                  {visible ? <EyeOff /> : <Eye />}
                </button>
              }
            />
          </Field>
          <div className="-mt-2 flex justify-end">
            <Button variant="link" size="sm">
              {t('demo.auth.forgot')}
            </Button>
          </div>
          <Button type="submit" size="lg" block>
            {t('demo.auth.signIn')}
          </Button>
        </form>
        <div className="flex items-center gap-3 text-xs text-fg-subtle">
          <Separator className="flex-1" />
          {t('demo.auth.or')}
          <Separator className="flex-1" />
        </div>
        <div className="flex flex-col gap-3">
          <Button variant="secondary" size="lg" block leftIcon={<Mail />}>
            {t('demo.auth.otp')}
          </Button>
          <Button variant="secondary" size="lg" block leftIcon={<KeyRound />}>
            {t('demo.auth.sso')}
          </Button>
        </div>
        <p className="text-center text-sm text-fg-muted">
          {t('demo.auth.noAccount')}{' '}
          <Button variant="link" size="sm" className="font-semibold">
            {t('demo.auth.register')}
          </Button>
        </p>
      </div>
    </AuthLayout>
  );
}
