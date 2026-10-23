import { CircleCheck, CircleMinus, CircleX, Search, TriangleAlert, UserCheck } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  cn,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Field,
  Input,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import {
  DIRECTORY_LIMITS,
  useTestDirectoryUser,
  type DirectorySettingsUpdate,
  type DirectoryTestStep,
  type DirectoryUserTest,
} from './api';

const statusIcon = {
  ok: <CircleCheck aria-hidden className="text-success" />,
  warning: <TriangleAlert aria-hidden className="text-warning" />,
  failed: <CircleX aria-hidden className="text-danger" />,
  skipped: <CircleMinus aria-hidden className="text-fg-faint" />,
} as const;

/** Every step of a check with its reason in plain words. */
export function StepList({
  steps,
  onUseBase,
}: {
  steps: DirectoryTestStep[];
  /** Offers a naming context of the server as the search base. */
  onUseBase?: (dn: string) => void;
}) {
  const { t } = useTranslation('directory');
  const format = useFormatters();
  return (
    <ol className="flex flex-col divide-y divide-border rounded-xl border border-border bg-surface">
      {steps.map((step, index) => (
        <li
          // Steps repeat (one connection attempt per address); the order is the identity.
          key={index}
          className="flex gap-3 p-3"
          data-testid={`directory-step-${step.step}`}
          data-status={step.status}
        >
          <span className="mt-0.5 shrink-0 [&_svg]:size-4.5">{statusIcon[step.status]}</span>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              <span className="text-sm font-semibold text-fg">{t(`steps.${step.step}`)}</span>
              <span
                className={cn(
                  'text-xs',
                  step.status === 'failed' ? 'text-danger' : 'text-fg-subtle',
                )}
              >
                {t(`statuses.${step.status}`)}
              </span>
              {step.server && (
                <span className="font-mono text-xs break-all text-fg-subtle">{step.server}</span>
              )}
            </div>
            {step.code && (
              <p className="text-sm text-fg-muted">
                {t(`reasons.${step.code}` as 'reasons.unknown', {
                  defaultValue: t('reasons.unknown'),
                })}
              </p>
            )}
            {step.certificate && (
              <p className="text-xs break-all text-fg-subtle">
                {t('check.certificate', {
                  subject: step.certificate.subject,
                  issuer: step.certificate.issuer,
                  date: format.dateTime(step.certificate.not_after, { dateStyle: 'long' }),
                })}
              </p>
            )}
            {step.step === 'search' && step.status === 'ok' && (
              <p className="text-xs text-fg-subtle">{t('check.found')}</p>
            )}
            {step.vendor && (
              <p className="text-xs text-fg-subtle">{t('check.vendor', { vendor: step.vendor })}</p>
            )}
            {step.naming_contexts && step.naming_contexts.length > 0 && (
              <div className="flex flex-col gap-1.5">
                <span className="text-xs text-fg-subtle">{t('check.namingContexts')}</span>
                <ul className="flex flex-wrap gap-2">
                  {step.naming_contexts.map((dn) => (
                    <li key={dn} className="flex items-center gap-2">
                      <code className="rounded-md bg-surface-raised px-1.5 py-0.5 text-xs break-all text-fg">
                        {dn}
                      </code>
                      {onUseBase && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => {
                            onUseBase(dn);
                          }}
                        >
                          {t('check.useAsBase')}
                        </Button>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {step.detail && (
              <p className="font-mono text-xs break-all text-fg-subtle">{step.detail}</p>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}

/** The dialog "Check a user": a login in, what Qadam would make of it out. */
export function UserCheckDialog({
  open,
  onOpenChange,
  settings,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The settings of the form, read when the check starts. */
  settings: () => DirectorySettingsUpdate;
}) {
  const { t } = useTranslation('directory');
  const [username, setUsername] = useState('');
  const [result, setResult] = useState<DirectoryUserTest | null>(null);
  const check = useTestDirectoryUser();

  const run = () => {
    if (username.trim() === '') return;
    check.mutate(
      { settings: settings(), username },
      {
        onSuccess: setResult,
        onError: (err) => {
          setResult(null);
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t('userCheck.title')}</DialogTitle>
          <DialogDescription>{t('userCheck.subtitle')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="flex flex-col gap-5">
          <form
            className="flex flex-col gap-3 sm:flex-row sm:items-end"
            onSubmit={(event) => {
              event.preventDefault();
              // React would pass the submit on to a form around the dialog.
              event.stopPropagation();
              run();
            }}
          >
            <Field label={t('userCheck.username')} className="flex-1">
              <Input
                value={username}
                maxLength={DIRECTORY_LIMITS.username}
                autoComplete="off"
                autoCapitalize="none"
                spellCheck={false}
                onChange={(event) => {
                  setUsername(event.target.value);
                }}
              />
            </Field>
            <Button
              type="submit"
              leftIcon={<Search />}
              loading={check.isPending}
              disabled={username.trim() === ''}
            >
              {t('userCheck.find')}
            </Button>
          </form>
          {result && <UserResult result={result} />}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

function UserResult({ result }: { result: DirectoryUserTest }) {
  const { t } = useTranslation(['directory', 'common']);
  if (!result.found || !result.user) {
    const code = result.code ?? 'not_found';
    return (
      <div className="flex flex-col gap-3" data-testid="directory-user-result">
        <Callout tone={code === 'connection' ? 'danger' : 'warning'}>
          {t(`userCheck.codes.${code}` as 'userCheck.codes.not_found')}
        </Callout>
        {code === 'connection' && <StepList steps={result.steps} />}
      </div>
    );
  }
  const user = result.user;
  const signIn = user.sign_in;
  const refusals = new Set([
    'required_group',
    'email_missing',
    'link_denied',
    'auto_provision_off',
    'suspended',
  ]);
  return (
    <div className="flex flex-col gap-4" data-testid="directory-user-result">
      <Callout
        tone={signIn.allowed ? 'success' : 'danger'}
        icon={signIn.allowed ? <UserCheck aria-hidden /> : undefined}
        title={signIn.allowed ? t('userCheck.allowed') : t('userCheck.denied')}
      >
        <ul className="flex flex-col gap-1">
          {signIn.outcome !== 'denied' && <li>{t(`userCheck.outcomes.${signIn.outcome}`)}</li>}
          {signIn.reasons.map((reason) => (
            <li key={reason} className={cn(refusals.has(reason) && 'font-medium')}>
              {t(`userCheck.reasons.${reason}`)}
            </li>
          ))}
        </ul>
      </Callout>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-[10rem_minmax(0,1fr)]">
        <dt className="text-fg-subtle">{t('userCheck.dn')}</dt>
        <dd className="font-mono text-xs break-all text-fg">{user.dn}</dd>
        <dt className="text-fg-subtle">{t('userCheck.login')}</dt>
        <dd className="font-mono text-xs break-all text-fg">{user.login}</dd>
        <dt className="text-fg-subtle">{t('userCheck.email')}</dt>
        <dd className={cn('break-all', user.email ? 'text-fg' : 'text-danger')}>
          {user.email ?? t('userCheck.noEmail')}
        </dd>
        <dt className="text-fg-subtle">{t('userCheck.name')}</dt>
        <dd className="text-fg">{user.full_name}</dd>
        <dt className="text-fg-subtle">{t('userCheck.subject')}</dt>
        <dd className="font-mono text-xs break-all text-fg">{user.subject}</dd>
        <dt className="text-fg-subtle">{t('userCheck.groups')}</dt>
        <dd>
          {user.groups.length === 0 ? (
            <span className="text-fg-muted">{t('userCheck.noGroups')}</span>
          ) : (
            <ul className="flex flex-col gap-1">
              {user.groups.map((group) => (
                <li key={group} className="font-mono text-xs break-all text-fg">
                  {group}
                </li>
              ))}
            </ul>
          )}
        </dd>
        <dt className="text-fg-subtle">{t('userCheck.roles')}</dt>
        <dd className="flex flex-wrap gap-1.5">
          {user.roles.map((role) => (
            <Badge key={role} tone="primary" size="sm">
              {t(`common:roles.${role}`)}
            </Badge>
          ))}
        </dd>
      </dl>
    </div>
  );
}
