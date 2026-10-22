import { zodResolver } from '@hookform/resolvers/zod';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import { Mail } from 'lucide-react';
import { useEffect, useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import { Button, Field, Input } from '@/design-system';
import { errorMessage } from '@/shared/api/errors';

import { landingAfterSignIn, useRequestLoginCode, useVerifyLoginCode } from './api';
import { AuthPage, CodeInput, FormAlert } from './components';

const RESEND_SECONDS = 60;

const emailSchema = z.object({ email: z.email().max(254) });

function useCountdown(seconds: number) {
  const [left, setLeft] = useState(seconds);
  useEffect(() => {
    if (left <= 0) return;
    const timer = window.setTimeout(() => {
      setLeft((v) => v - 1);
    }, 1000);
    return () => {
      window.clearTimeout(timer);
    };
  }, [left]);
  return [left, setLeft] as const;
}

export function LoginCodePage({
  email: initialEmail,
  sent,
  redirect,
}: {
  email?: string;
  sent?: boolean;
  redirect?: string;
}) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const requestCode = useRequestLoginCode();
  const verify = useVerifyLoginCode();
  const [email, setEmail] = useState(sent && initialEmail ? initialEmail : '');
  const [code, setCode] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [left, setLeft] = useCountdown(sent ? RESEND_SECONDS : 0);
  const labelId = useId();

  const form = useForm<z.infer<typeof emailSchema>>({
    resolver: zodResolver(emailSchema),
    defaultValues: { email: initialEmail ?? '' },
  });

  const send = form.handleSubmit(async ({ email: address }) => {
    setFormError(null);
    try {
      await requestCode.mutateAsync(address);
      setEmail(address);
      setCode('');
      setLeft(RESEND_SECONDS);
    } catch (err) {
      setFormError(errorMessage(err));
    }
  });

  const resend = async () => {
    setFormError(null);
    try {
      await requestCode.mutateAsync(email);
      setCode('');
      setLeft(RESEND_SECONDS);
    } catch (err) {
      setFormError(errorMessage(err));
    }
  };

  const submit = async (value: string) => {
    if (verify.isPending) return;
    setFormError(null);
    try {
      const session = await verify.mutateAsync({ email, code: value });
      await navigate({ href: landingAfterSignIn(session, redirect), replace: true });
    } catch (err) {
      setCode('');
      setFormError(errorMessage(err));
    }
  };

  if (!email) {
    return (
      <AuthPage title={t('code.title')} subtitle={t('code.emailStep')}>
        <FormAlert message={formError} />
        <form className="flex flex-col gap-5" onSubmit={(e) => void send(e)} noValidate>
          <Field label={t('fields.email')} error={form.formState.errors.email?.message}>
            <Input
              type="email"
              autoComplete="email"
              placeholder={t('fields.emailPlaceholder')}
              leading={<Mail />}
              {...form.register('email')}
            />
          </Field>
          <Button type="submit" size="lg" block loading={requestCode.isPending}>
            {t('code.send')}
          </Button>
        </form>
        <p className="text-center text-sm">
          <Link
            to="/login"
            search={{ redirect }}
            className="font-semibold text-primary-text hover:underline"
          >
            {t('code.backToPassword')}
          </Link>
        </p>
      </AuthPage>
    );
  }

  return (
    <AuthPage title={t('code.title')} subtitle={t('code.sentTo', { email })}>
      <FormAlert message={formError} />
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault();
          if (code.length === 6) void submit(code);
        }}
      >
        <div className="flex flex-col gap-2">
          <span id={labelId} className="text-sm font-medium text-fg">
            {t('code.label')}
          </span>
          <CodeInput
            value={code}
            onChange={setCode}
            onComplete={(value) => void submit(value)}
            disabled={verify.isPending}
            invalid={Boolean(formError)}
            labelledBy={labelId}
          />
        </div>
        <Button type="submit" size="lg" block loading={verify.isPending} disabled={code.length < 6}>
          {t('code.submit')}
        </Button>
      </form>
      <div className="flex flex-col items-center gap-2 text-sm">
        {left > 0 ? (
          <p className="text-fg-subtle tabular-nums" aria-live="polite">
            {t('code.resendIn', { seconds: left })}
          </p>
        ) : (
          <Button
            variant="link"
            size="sm"
            loading={requestCode.isPending}
            onClick={() => void resend()}
          >
            {t('code.resend')}
          </Button>
        )}
        <Button
          variant="link"
          size="sm"
          className="text-fg-muted"
          onClick={() => {
            setEmail('');
            setFormError(null);
          }}
        >
          {t('code.changeEmail')}
        </Button>
      </div>
    </AuthPage>
  );
}

const loginCodeRoute = getRouteApi('/login/code');

/** `/login/code`: the search params become props. */
export function LoginCodeRoute() {
  const { email, sent, redirect } = loginCodeRoute.useSearch();
  return <LoginCodePage email={email} sent={sent} redirect={redirect} />;
}
