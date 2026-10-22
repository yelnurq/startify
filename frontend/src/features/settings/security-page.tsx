import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { KeyRound, Laptop, LogOut, Smartphone } from 'lucide-react';
import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  ConfirmDialog,
  Field,
  SkeletonText,
  toast,
} from '@/design-system';
import { FormAlert, PasswordInput, PasswordStrength } from '@/features/auth/components';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { i18n } from '@/shared/i18n';
import { useFormatters } from '@/shared/i18n/format';

import {
  describeAgent,
  sessionsQuery,
  useChangePassword,
  useRevokeOtherSessions,
  useRevokeSession,
  type SessionInfo,
} from './api';

/** Active sessions, signing out elsewhere and changing the password. */
export function SecurityPage() {
  const { t } = useTranslation('app');
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-semibold text-fg">{t('security.title')}</h1>
        <p className="text-fg-muted">{t('security.subtitle')}</p>
      </header>
      <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <SessionsCard />
        <PasswordCard />
      </div>
    </div>
  );
}

function SessionsCard() {
  const { t } = useTranslation(['app', 'common']);
  const sessions = useQuery(sessionsQuery);
  const revokeOthers = useRevokeOtherSessions();
  const others = sessions.data?.filter((session) => !session.current).length ?? 0;
  const [confirming, setConfirming] = useState(false);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('security.sessions.title')}</CardTitle>
        <CardDescription>{t('security.sessions.subtitle')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-5">
        {sessions.isPending && <SkeletonText lines={5} />}
        {sessions.isError && <Callout tone="danger">{errorMessage(sessions.error)}</Callout>}
        {sessions.isSuccess && (
          <ul className="flex flex-col divide-y divide-border" data-testid="sessions">
            {sessions.data.map((session) => (
              <SessionRow key={session.id} session={session} />
            ))}
          </ul>
        )}
        <div className="flex flex-col gap-3 rounded-xl border border-danger-border bg-danger-soft/50 p-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-fg-muted">{t('security.sessions.othersHint')}</p>
          <Button
            variant="danger"
            size="sm"
            leftIcon={<LogOut />}
            className="shrink-0"
            disabled={others === 0}
            onClick={() => {
              setConfirming(true);
            }}
          >
            {t('security.sessions.others')}
          </Button>
        </div>
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          tone="danger"
          title={t('security.sessions.confirmTitle')}
          description={t('security.sessions.confirmText', { n: others })}
          confirmLabel={t('security.sessions.others')}
          loading={revokeOthers.isPending}
          onConfirm={() => {
            revokeOthers.mutate(undefined, {
              onSuccess: () => {
                setConfirming(false);
                toast.success(t('security.sessions.revokedOthers'));
              },
              onError: (err) => {
                toast.error(errorMessage(err));
              },
            });
          }}
        />
      </CardContent>
    </Card>
  );
}

function SessionRow({ session }: { session: SessionInfo }) {
  const { t } = useTranslation(['app', 'common']);
  const format = useFormatters();
  const revoke = useRevokeSession();
  const agent = describeAgent(session.user_agent);
  const Icon = agent.mobile ? Smartphone : Laptop;
  const name =
    [agent.browser, agent.os].filter(Boolean).join(' · ') || t('security.sessions.unknownDevice');

  return (
    <li className="flex items-start gap-3 py-3 first:pt-0 last:pb-0">
      <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-surface-raised text-fg-muted">
        <Icon className="size-5" aria-hidden />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium text-fg">
          {name}
          {session.current && (
            <Badge tone="success" size="sm" dot>
              {t('security.sessions.current')}
            </Badge>
          )}
        </p>
        <p className="text-xs text-fg-subtle">
          {session.ip ? `${session.ip} · ` : ''}
          {t('security.sessions.signedIn', { date: format.dateTime(session.created_at) })}
        </p>
        <p className="text-xs text-fg-subtle">
          {t('security.sessions.lastUsed', { when: format.relative(session.last_used_at) })}
        </p>
      </div>
      {!session.current && (
        <Button
          size="sm"
          variant="ghost"
          loading={revoke.isPending}
          onClick={() => {
            revoke.mutate(session.id, {
              onSuccess: () => {
                toast.success(t('security.sessions.revoked'));
              },
              onError: (err) => {
                toast.error(errorMessage(err));
              },
            });
          }}
        >
          {t('security.sessions.revoke')}
        </Button>
      )}
    </li>
  );
}

const passwordSchema = z
  .object({
    current_password: z.string().min(1).max(128),
    new_password: z.string().min(10).max(128),
    confirm: z.string().min(1),
  })
  .refine((v) => v.new_password === v.confirm, {
    path: ['confirm'],
    error: () => i18n.t('common:validation.passwordsMismatch'),
  });

type PasswordValues = z.infer<typeof passwordSchema>;

function PasswordCard() {
  const { t } = useTranslation(['app', 'auth', 'common']);
  const change = useChangePassword();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<PasswordValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', new_password: '', confirm: '' },
  });
  const password = useWatch({ control: form.control, name: 'new_password' });
  const { errors, isSubmitting } = form.formState;

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await change.mutateAsync({
        current_password: values.current_password,
        new_password: values.new_password,
      });
      form.reset();
      toast.success(t('security.password.changed'), {
        description: t('security.password.changedText'),
      });
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth.password_incorrect') {
        form.setError('current_password', { message: errorMessage(err) });
      } else if (err instanceof ApiError && err.code === 'auth.password_weak') {
        form.setError('new_password', { message: errorMessage(err) });
      } else {
        setFormError(errorMessage(err));
      }
    }
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('security.password.title')}</CardTitle>
        <CardDescription>{t('security.password.subtitle')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-5" onSubmit={(e) => void onSubmit(e)} noValidate>
          <FormAlert message={formError} />
          <Field label={t('auth:fields.currentPassword')} error={errors.current_password?.message}>
            <PasswordInput autoComplete="current-password" {...form.register('current_password')} />
          </Field>
          <Field label={t('auth:fields.newPassword')} error={errors.new_password?.message}>
            <PasswordInput autoComplete="new-password" {...form.register('new_password')} />
          </Field>
          <PasswordStrength password={password} />
          <Field label={t('auth:fields.confirmPassword')} error={errors.confirm?.message}>
            <PasswordInput autoComplete="new-password" {...form.register('confirm')} />
          </Field>
          <Button
            type="submit"
            leftIcon={<KeyRound />}
            loading={isSubmitting}
            className="self-start"
          >
            {t('security.password.submit')}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
