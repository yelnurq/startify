import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { ChevronDown, Lock, Mail, UserRound } from 'lucide-react';
import { useId, useMemo, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import { Button, Checkbox, EmptyState, Field, Input, Skeleton, toast } from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { applyFieldErrors } from '@/shared/forms/validation';
import { i18n, isLocale } from '@/shared/i18n';
import { useLocalized } from '@/shared/i18n/localized';

import { landingAfterSignIn, tenantQuery, useRegister, type Tenant } from './api';
import { AuthPage, FormAlert, PasswordInput, PasswordStrength } from './components';
import { SsoButtons } from './login-page';

function buildSchema(domains: string[]) {
  return z.object({
    full_name: z.string().trim().min(2).max(120),
    email: z
      .email()
      .max(254)
      .refine(
        (email) =>
          domains.length === 0 ||
          domains.some((d) => email.toLowerCase().endsWith(`@${d.toLowerCase()}`)),
        { error: () => i18n.t('common:errors.auth.email_domain_not_allowed') },
      ),
    password: z.string().min(10).max(128),
    consent: z
      .boolean()
      .refine((v) => v, { error: () => i18n.t('common:errors.auth.consent_required') }),
  });
}

type Values = z.infer<ReturnType<typeof buildSchema>>;

export function RegisterForm({ tenant }: { tenant: Tenant }) {
  const { t } = useTranslation(['auth', 'common']);
  const navigate = useNavigate();
  const register = useRegister();
  const text = useLocalized();
  const [formError, setFormError] = useState<string | null>(null);
  const [consentOpen, setConsentOpen] = useState(false);
  const consentId = useId();
  const strengthId = useId();
  const schema = useMemo(() => buildSchema(tenant.email_domains), [tenant.email_domains]);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { full_name: '', email: '', password: '', consent: false },
  });
  const password = useWatch({ control: form.control, name: 'password' });
  const consent = useWatch({ control: form.control, name: 'consent' });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      const session = await register.mutateAsync({
        email: values.email,
        password: values.password,
        full_name: values.full_name,
        locale: isLocale(i18n.language) ? i18n.language : 'kk',
        consent_version: tenant.consent.version,
      });
      toast.success(t('register.verifySent', { email: values.email }));
      await navigate({ href: landingAfterSignIn(session), replace: true });
    } catch (err) {
      if (applyFieldErrors(err, form.setError)) return;
      if (err instanceof ApiError) {
        if (err.code === 'auth.email_taken' || err.code === 'auth.email_domain_not_allowed') {
          form.setError('email', { message: errorMessage(err) });
          return;
        }
        if (err.code === 'auth.password_weak') {
          form.setError('password', { message: errorMessage(err) });
          return;
        }
        if (err.code === 'auth.consent_required') {
          form.setError('consent', { message: errorMessage(err) });
          return;
        }
      }
      setFormError(errorMessage(err));
    }
  });

  const errors = form.formState.errors;
  return (
    <>
      <FormAlert message={formError} />
      <form className="flex flex-col gap-5" onSubmit={(e) => void onSubmit(e)} noValidate>
        <Field
          label={t('fields.fullName')}
          hint={t('fields.fullNameHint')}
          error={errors.full_name?.message}
          required
        >
          <Input
            autoComplete="name"
            placeholder={t('fields.fullNamePlaceholder')}
            leading={<UserRound />}
            {...form.register('full_name')}
          />
        </Field>
        <Field
          label={t('fields.email')}
          hint={
            tenant.email_domains.length > 0
              ? t('register.domainHint', {
                  domains: tenant.email_domains.map((d) => `@${d}`).join(', '),
                })
              : undefined
          }
          error={errors.email?.message}
          required
        >
          <Input
            type="email"
            autoComplete="email"
            placeholder={t('fields.emailPlaceholder')}
            leading={<Mail />}
            {...form.register('email')}
          />
        </Field>
        <Field label={t('fields.password')} error={errors.password?.message} required>
          <PasswordInput
            autoComplete="new-password"
            leading={<Lock />}
            aria-describedby={strengthId}
            {...form.register('password')}
          />
        </Field>
        <PasswordStrength password={password} id={strengthId} />
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface-raised/60 p-4">
          <label className="flex items-start gap-3 text-sm text-fg">
            <Checkbox
              className="mt-0.5"
              aria-invalid={errors.consent ? true : undefined}
              aria-describedby={errors.consent ? `${consentId}-error` : undefined}
              checked={consent}
              onCheckedChange={(checked) => {
                form.setValue('consent', checked === true, { shouldValidate: true });
              }}
            />
            <span>{t('register.consentLabel')}</span>
          </label>
          <button
            type="button"
            aria-expanded={consentOpen}
            aria-controls={consentId}
            className="flex w-fit items-center gap-1 text-sm font-medium text-primary-text hover:underline"
            onClick={() => {
              setConsentOpen((v) => !v);
            }}
          >
            {t('register.consentShow')}
            <ChevronDown
              className={`size-4 transition-transform ${consentOpen ? 'rotate-180' : ''}`}
              aria-hidden
            />
          </button>
          {consentOpen && (
            <div
              id={consentId}
              className="max-h-56 overflow-y-auto rounded-lg bg-surface p-3 text-xs leading-relaxed whitespace-pre-line text-fg-muted"
            >
              {text(tenant.consent.text)}
              <p className="mt-2 text-fg-subtle">
                {t('register.consentVersion', { version: tenant.consent.version })}
              </p>
            </div>
          )}
          {errors.consent?.message && (
            <p id={`${consentId}-error`} className="text-sm text-danger" aria-live="polite">
              {errors.consent.message}
            </p>
          )}
        </div>
        <Button type="submit" size="lg" block loading={register.isPending}>
          {t('register.submit')}
        </Button>
      </form>
    </>
  );
}

export function RegisterPage() {
  const { t } = useTranslation('auth');
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();

  if (tenant.isPending) {
    return (
      <AuthPage title={t('register.title')}>
        <div className="flex flex-col gap-4" aria-busy>
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </div>
      </AuthPage>
    );
  }

  if (tenant.data && !tenant.data.self_signup) {
    return (
      <AuthPage title={t('register.closedTitle')}>
        <EmptyState
          size="sm"
          illustration="team"
          title={t('register.closedTitle')}
          description={t('register.closedText', { university: text(tenant.data.short_name) })}
          action={
            <Button asChild>
              <Link to="/login">{t('register.signIn')}</Link>
            </Button>
          }
        />
        <SsoButtons />
      </AuthPage>
    );
  }

  return (
    <AuthPage title={t('register.title')} subtitle={t('register.subtitle')} art="idea">
      {tenant.data && <RegisterForm tenant={tenant.data} />}
      {tenant.isError && <FormAlert message={t('register.tenantUnavailable')} />}
      <p className="text-center text-sm text-fg-muted">
        {t('register.hasAccount')}{' '}
        <Link to="/login" className="font-semibold text-primary-text hover:underline">
          {t('register.signIn')}
        </Link>
      </p>
    </AuthPage>
  );
}
