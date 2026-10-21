import { Link } from '@tanstack/react-router';
import { Check, Eye, EyeOff, KeyRound, Mail, Sparkles } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { AuthLayout } from '@/app/layouts/auth-layout';
import { Button, Field, Input, Logo, Separator } from '@/design-system';

export function AuthDemo() {
  const { t } = useTranslation('design');
  const [visible, setVisible] = useState(false);
  return (
    <AuthLayout
      logo={
        <Link to="/design" className="rounded-lg">
          <Logo />
        </Link>
      }
      aside={
        <div className="flex max-w-lg flex-col gap-8">
          <div className="w-fit rounded-2xl border border-white/20 bg-white/10 p-4 shadow-lg backdrop-blur-md">
            <p className="flex items-center gap-2 text-xs font-semibold tracking-wide text-white/80 uppercase">
              <Sparkles className="size-3.5 text-gold-300" aria-hidden />
              {t('demo.auth.cardEyebrow')}
            </p>
            <p className="mt-1 font-display text-3xl font-semibold">{t('demo.auth.cardValue')}</p>
            <p className="text-sm text-white/75">{t('demo.auth.cardText')}</p>
          </div>
          <h2 className="font-display text-4xl leading-tight font-semibold">
            {t('demo.auth.headline')}
          </h2>
          <ul className="flex flex-col gap-3 text-white/85">
            {(['point1', 'point2', 'point3'] as const).map((key) => (
              <li key={key} className="flex items-start gap-3">
                <span className="mt-0.5 grid size-5 shrink-0 place-items-center rounded-full bg-white/20">
                  <Check className="size-3" aria-hidden />
                </span>
                {t(`demo.auth.${key}`)}
              </li>
            ))}
          </ul>
        </div>
      }
    >
      <div className="flex flex-col gap-8">
        <div className="flex flex-col gap-2">
          <h1 className="font-display text-3xl font-semibold text-fg">{t('demo.auth.title')}</h1>
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
