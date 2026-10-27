import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import { CircleCheck, Mail } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import { Button, Callout, EmptyState, Field, Input, Spinner, toast } from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { i18n } from '@/shared/i18n';

import {
  landingAfterSignIn,
  sessionQuery,
  useForgotPassword,
  useResetPassword,
  useStoreSession,
  verifyEmail,
} from './api';
import {
  AuthPage,
  FormAlert,
  PasswordInput,
  PasswordStrength,
  ResendVerificationButton,
} from './components';

/**
 * Consumes the e-mail verification token once (queries are de-duplicated).
 * The link signs in, so the page continues where the member is expected:
 * onboarding after a new registration, otherwise the app.
 */
export function VerifyEmailPage({ token }: { token?: string }) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const storeSession = useStoreSession();
  const session = useQuery(sessionQuery);
  const verification = useQuery({
    queryKey: ['verify-email', token],
    queryFn: () => verifyEmail(token ?? ''),
    enabled: Boolean(token),
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: Number.POSITIVE_INFINITY,
  });

  const verified = verification.data;
  const continued = useRef(false);
  useEffect(() => {
    if (!verified || continued.current) return;
    continued.current = true;
    storeSession(verified);
    toast.success(t('verify.successTitle'));
    void navigate({ href: landingAfterSignIn(verified), replace: true });
  }, [verified, storeSession, navigate, t]);

  const signedInEmail = session.data?.user.email;
  const failed = !token || verification.isError;

  return (
    <AuthPage title={t('verify.title')}>
      {verification.isPending && token && (
        <p className="flex items-center gap-3 text-fg-muted" role="status">
          <Spinner className="text-brand" />
          {t('verify.checking')}
        </p>
      )}
      {verified && (
        <EmptyState
          size="sm"
          illustration="inbox"
          title={t('verify.successTitle')}
          description={t('verify.successText')}
          action={
            <Button asChild>
              <Link to={verified.membership.onboarded ? '/app' : '/onboarding'}>
                {t('verify.toApp')}
              </Link>
            </Button>
          }
        />
      )}
      {failed && (
        <div className="flex flex-col gap-4">
          <Callout tone="danger" title={t('verify.failedTitle')}>
            {verification.error ? errorMessage(verification.error) : t('verify.failedText')}
          </Callout>
          {signedInEmail ? (
            <ResendVerificationButton email={signedInEmail} />
          ) : (
            <Button asChild variant="secondary">
              <Link to="/login">{t('verify.toLogin')}</Link>
            </Button>
          )}
        </div>
      )}
    </AuthPage>
  );
}

const forgotSchema = z.object({ email: z.email().max(254) });

export function ForgotPasswordPage() {
  const { t } = useTranslation(['auth', 'common']);
  const forgot = useForgotPassword();
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<z.infer<typeof forgotSchema>>({
    resolver: zodResolver(forgotSchema),
    defaultValues: { email: '' },
  });

  const onSubmit = form.handleSubmit(async ({ email }) => {
    setFormError(null);
    try {
      await forgot.mutateAsync(email);
      setSentTo(email);
    } catch (err) {
      setFormError(errorMessage(err));
    }
  });

  if (sentTo) {
    return (
      <AuthPage title={t('forgot.sentTitle')}>
        <EmptyState
          size="sm"
          illustration="inbox"
          title={t('forgot.sentTitle')}
          description={t('forgot.sentText', { email: sentTo })}
          action={
            <Button asChild variant="secondary">
              <Link to="/login">{t('forgot.back')}</Link>
            </Button>
          }
        />
      </AuthPage>
    );
  }

  return (
    <AuthPage title={t('forgot.title')} subtitle={t('forgot.subtitle')} art="inbox">
      <FormAlert message={formError} />
      <form className="flex flex-col gap-5" onSubmit={(e) => void onSubmit(e)} noValidate>
        <Field label={t('fields.email')} error={form.formState.errors.email?.message}>
          <Input
            type="email"
            autoComplete="email"
            placeholder={t('fields.emailPlaceholder')}
            leading={<Mail />}
            {...form.register('email')}
          />
        </Field>
        <Button type="submit" size="lg" block loading={forgot.isPending}>
          {t('forgot.submit')}
        </Button>
      </form>
      <p className="text-center text-sm">
        <Link to="/login" className="font-semibold text-primary-text hover:underline">
          {t('forgot.back')}
        </Link>
      </p>
    </AuthPage>
  );
}

const resetSchema = z
  .object({
    password: z.string().min(10).max(128),
    confirm: z.string().min(1),
  })
  .refine((v) => v.password === v.confirm, {
    path: ['confirm'],
    error: () => i18n.t('common:validation.passwordsMismatch'),
  });

export function ResetPasswordPage({ token, invite }: { token?: string; invite?: boolean }) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const reset = useResetPassword();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<z.infer<typeof resetSchema>>({
    resolver: zodResolver(resetSchema),
    defaultValues: { password: '', confirm: '' },
  });
  const password = useWatch({ control: form.control, name: 'password' });

  const onSubmit = form.handleSubmit(async ({ password: value }) => {
    setFormError(null);
    try {
      await reset.mutateAsync({ token: token ?? '', password: value });
      toast.success(t('reset.doneTitle'), { description: t('reset.doneText') });
      await navigate({ to: '/login', replace: true });
    } catch (err) {
      setFormError(errorMessage(err));
    }
  });

  const title = invite ? t('reset.inviteTitle') : t('reset.title');
  if (!token) {
    return (
      <AuthPage title={title}>
        <Callout tone="danger">{t('reset.missingToken')}</Callout>
        <Button asChild variant="secondary">
          <Link to="/forgot-password">{t('verify.resend')}</Link>
        </Button>
      </AuthPage>
    );
  }

  return (
    <AuthPage title={title} subtitle={invite ? t('reset.inviteSubtitle') : t('reset.subtitle')}>
      <FormAlert message={formError} />
      <form className="flex flex-col gap-5" onSubmit={(e) => void onSubmit(e)} noValidate>
        <Field label={t('fields.newPassword')} error={form.formState.errors.password?.message}>
          <PasswordInput autoComplete="new-password" {...form.register('password')} />
        </Field>
        <PasswordStrength password={password} />
        <Field label={t('fields.confirmPassword')} error={form.formState.errors.confirm?.message}>
          <PasswordInput autoComplete="new-password" {...form.register('confirm')} />
        </Field>
        <Button type="submit" size="lg" block loading={reset.isPending} leftIcon={<CircleCheck />}>
          {t('reset.submit')}
        </Button>
      </form>
    </AuthPage>
  );
}

const verifyEmailRoute = getRouteApi('/verify-email');

export function VerifyEmailRoute() {
  const { token } = verifyEmailRoute.useSearch();
  return <VerifyEmailPage token={token} />;
}

const resetPasswordRoute = getRouteApi('/reset-password');

export function ResetPasswordRoute() {
  const { token, invite } = resetPasswordRoute.useSearch();
  return <ResetPasswordPage token={token} invite={invite} />;
}
