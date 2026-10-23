import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import { Building2, KeyRound, Mail, UserRound } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import { Button, Callout, Field, Input, SegmentedControl, TruncatedText } from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { codeMessage, errorMessage } from '@/shared/api/errors';
import { applyFieldErrors } from '@/shared/forms/validation';
import { useLocalized } from '@/shared/i18n/localized';

import {
  landingAfterSignIn,
  safeRedirect,
  tenantQuery,
  useDirectorySignIn,
  useRequestLoginCode,
  useSignIn,
  type Tenant,
} from './api';
import {
  AuthPage,
  Divider,
  FormAlert,
  PasswordInput,
  ResendVerificationButton,
} from './components';

const schema = z.object({
  email: z.email().max(254),
  password: z.string().min(1).max(128),
});

type Values = z.infer<typeof schema>;

const directorySchema = z.object({
  username: z.string().trim().min(1).max(256),
  password: z.string().min(1).max(256),
});

type DirectoryValues = z.infer<typeof directorySchema>;

type Method = 'directory' | 'email';

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
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  const [formError, setFormError] = useState<string | null>(error ? codeMessage(error) : null);
  const [chosen, setChosen] = useState<Method | null>(null);
  const directory = tenant.data?.directory_login;
  // The university's own accounts come first when the university offers them.
  const method: Method = directory?.enabled ? (chosen ?? 'directory') : 'email';
  const directoryLabel = text(directory?.label) || t('login.methods.directory');

  return (
    <AuthPage title={t('login.title')} subtitle={tenant.data ? text(tenant.data.name) : undefined}>
      {directory?.enabled && (
        <SegmentedControl<Method>
          label={t('login.methodLabel')}
          value={method}
          onValueChange={(next) => {
            setChosen(next);
            setFormError(null);
          }}
          className="grid w-full grid-cols-[minmax(0,1fr)_auto]"
          options={[
            {
              value: 'directory',
              icon: <Building2 aria-hidden />,
              label: <TruncatedText className="min-w-0">{directoryLabel}</TruncatedText>,
            },
            { value: 'email', icon: <Mail aria-hidden />, label: t('login.methods.email') },
          ]}
        />
      )}
      <FormAlert message={formError} />
      {method === 'directory' && directory ? (
        <DirectoryForm directory={directory} redirect={redirect} onError={setFormError} />
      ) : (
        <EmailForm redirect={redirect} onError={setFormError} />
      )}
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

/** Login and password of the university directory (Active Directory, LDAP). */
function DirectoryForm({
  directory,
  redirect,
  onError,
}: {
  directory: Tenant['directory_login'];
  redirect: string | undefined;
  onError: (message: string | null) => void;
}) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const text = useLocalized();
  const signIn = useDirectorySignIn();
  const form = useForm<DirectoryValues>({
    resolver: zodResolver(directorySchema),
    defaultValues: { username: '', password: '' },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    onError(null);
    try {
      const session = await signIn.mutateAsync(values);
      await navigate({ href: landingAfterSignIn(session, redirect), replace: true });
    } catch (err) {
      // The same code as a wrong e-mail password, worded for a login.
      if (err instanceof ApiError && err.code === 'auth.invalid_credentials') {
        onError(t('login.invalidDirectoryCredentials'));
        return;
      }
      if (!applyFieldErrors(err, form.setError)) onError(errorMessage(err));
    }
  });

  const errors = form.formState.errors;
  return (
    <form
      className="flex flex-col gap-5"
      onSubmit={(e) => void onSubmit(e)}
      noValidate
      data-testid="directory-login"
    >
      <Field
        label={t('fields.username')}
        hint={text(directory.username_hint) || t('login.usernameHint')}
        error={errors.username?.message}
      >
        <Input
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          leading={<UserRound />}
          {...form.register('username')}
        />
      </Field>
      <Field label={t('fields.password')} error={errors.password?.message}>
        <PasswordInput autoComplete="current-password" {...form.register('password')} />
      </Field>
      <Button type="submit" size="lg" block loading={signIn.isPending}>
        {t('login.submit')}
      </Button>
    </form>
  );
}

/** E-mail and password, a sign-in code, single sign-on. */
function EmailForm({
  redirect,
  onError,
}: {
  redirect: string | undefined;
  onError: (message: string | null) => void;
}) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const signIn = useSignIn();
  const requestCode = useRequestLoginCode();
  // The password was right, but the address waits for its verification link.
  const [unverified, setUnverified] = useState<string | null>(null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    onError(null);
    setUnverified(null);
    try {
      const session = await signIn.mutateAsync(values);
      await navigate({ href: landingAfterSignIn(session, redirect), replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth.email_unverified') {
        setUnverified(values.email);
        return;
      }
      if (!applyFieldErrors(err, form.setError)) onError(errorMessage(err));
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
      onError(errorMessage(err));
    }
  };

  const errors = form.formState.errors;
  return (
    <>
      {unverified && (
        <Callout
          tone="warning"
          title={t('login.unverifiedTitle')}
          action={<ResendVerificationButton key={unverified} email={unverified} />}
        >
          {t('login.unverifiedText', { email: unverified })}
        </Callout>
      )}
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
    </>
  );
}

const loginRoute = getRouteApi('/login/');

/** `/login`: the search params become props. */
export function LoginRoute() {
  const { redirect, error } = loginRoute.useSearch();
  return <LoginPage redirect={redirect} error={error} />;
}
