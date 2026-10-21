import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import { KeyRound, Mail } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

import { Button, Field, Input } from '@/design-system';
import { codeMessage, errorMessage } from '@/shared/api/errors';
import { applyFieldErrors } from '@/shared/forms/validation';
import { useLocalized } from '@/shared/i18n/localized';

import {
  landingAfterSignIn,
  safeRedirect,
  tenantQuery,
  useRequestLoginCode,
  useSignIn,
} from './api';
import { AuthPage, Divider, FormAlert, PasswordInput } from './components';

const schema = z.object({
  email: z.email().max(254),
  password: z.string().min(1).max(128),
});

type Values = z.infer<typeof schema>;

export function SsoButtons({ redirect }: { redirect?: string | undefined }) {
  const { t } = useTranslation('auth');
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  const providers = tenant.data?.sso_providers ?? [];
  if (providers.length === 0) return null;
  const target = encodeURIComponent(safeRedirect(redirect) ?? '/app');
  return (
    <div className="flex flex-col gap-3">
      {providers.map((provider) => (
        <Button key={provider.id} asChild variant="secondary" size="lg" block>
          <a href={`/api/v1/auth/oidc/${provider.id}/start?redirect=${target}`}>
            <KeyRound aria-hidden />
            {t('login.sso', { name: text(provider.name) })}
          </a>
        </Button>
      ))}
    </div>
  );
}

export function LoginPage({ redirect, error }: { redirect?: string; error?: string }) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const tenant = useQuery(tenantQuery);
  const signIn = useSignIn();
  const requestCode = useRequestLoginCode();
  const text = useLocalized();
  const [formError, setFormError] = useState<string | null>(error ? codeMessage(error) : null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      const session = await signIn.mutateAsync(values);
      await navigate({ href: landingAfterSignIn(session, redirect), replace: true });
    } catch (err) {
      if (!applyFieldErrors(err, form.setError)) setFormError(errorMessage(err));
    }
  });

  const signInWithCode = async () => {
    const email = form.getValues('email');
    const valid = await form.trigger('email');
    if (!valid) {
      await navigate({ to: '/login/code', search: { redirect } });
      return;
    }
    try {
      await requestCode.mutateAsync(email);
      await navigate({ to: '/login/code', search: { email, sent: true, redirect } });
    } catch (err) {
      setFormError(errorMessage(err));
    }
  };

  const errors = form.formState.errors;
  return (
    <AuthPage title={t('login.title')} subtitle={tenant.data ? text(tenant.data.name) : undefined}>
      <FormAlert message={formError} />
      <form className="flex flex-col gap-5" onSubmit={(e) => void onSubmit(e)} noValidate>
        <Field label={t('fields.email')} error={errors.email?.message}>
          <Input
            type="email"
            autoComplete="email"
            placeholder={t('fields.emailPlaceholder')}
            leading={<Mail />}
            {...form.register('email')}
          />
        </Field>
        <Field label={t('fields.password')} error={errors.password?.message}>
          <PasswordInput autoComplete="current-password" {...form.register('password')} />
        </Field>
        <div className="-mt-2 flex justify-end">
          <Button asChild variant="link" size="sm">
            <Link to="/forgot-password">{t('login.forgot')}</Link>
          </Button>
        </div>
        <Button type="submit" size="lg" block loading={signIn.isPending}>
          {t('login.submit')}
        </Button>
      </form>
      <Divider label={t('login.or')} />
      <div className="flex flex-col gap-3">
        <Button
          variant="secondary"
          size="lg"
          block
          leftIcon={<Mail />}
          loading={requestCode.isPending}
          onClick={() => void signInWithCode()}
        >
          {t('login.withCode')}
        </Button>
        <SsoButtons redirect={redirect} />
      </div>
      {tenant.data?.self_signup !== false && (
        <p className="text-center text-sm text-fg-muted">
          {t('login.noAccount')}{' '}
          <Link to="/register" className="font-semibold text-primary-text hover:underline">
            {t('login.register')}
          </Link>
        </p>
      )}
    </AuthPage>
  );
}

const loginRoute = getRouteApi('/login/');

/** `/login`: the search params become props. */
export function LoginRoute() {
  const { redirect, error } = loginRoute.useSearch();
  return <LoginPage redirect={redirect} error={error} />;
}
