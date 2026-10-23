import { useQuery } from '@tanstack/react-query';
import {
  Circle,
  CircleCheck,
  PlugZap,
  Plus,
  RotateCcw,
  Save,
  Trash2,
  TriangleAlert,
  UserSearch,
} from 'lucide-react';
import { useId, useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  cn,
  ConfirmDialog,
  Field,
  IconButton,
  Input,
  Radio,
  RadioGroup,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Skeleton,
  Switch,
  Textarea,
  toast,
} from '@/design-system';
import type { Role } from '@/features/auth/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { fieldErrorMessage } from '@/shared/forms/validation';
import { useLocalized } from '@/shared/i18n/localized';

import {
  DIRECTORY_LIMITS,
  DIRECTORY_PRESETS,
  directorySettingsQuery,
  useSaveDirectorySettings,
  useTestDirectoryConnection,
  type DirectoryConnectionTest,
  type DirectoryPreset,
  type DirectorySettings,
} from './api';
import { StepList, UserCheckDialog } from './checks';
import {
  applyPreset,
  connectionKey,
  draftErrors,
  isPlain,
  LANGS,
  mappingKey,
  presetDefaults,
  presetEdited,
  sameDraft,
  toDraft,
  toUpdate,
  type DirectoryDraft,
  type Lang,
} from './form';

const ROLES = [
  'student',
  'supervisor',
  'mentor',
  'jury',
  'innovation_manager',
  'tenant_admin',
  'leadership',
  'partner',
] as const satisfies readonly Role[];

/** Sign-in through the university directory: settings, checks and the switch. */
export function DirectoryPage() {
  const { t } = useTranslation(['directory', 'common']);
  const settings = useQuery(directorySettingsQuery);
  const enabled = settings.data?.data.enabled ?? false;
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-3xl font-bold text-ink">{t('title')}</h1>
          <p className="max-w-3xl text-fg-muted">{t('subtitle')}</p>
        </div>
        {settings.data && (
          <Badge
            tone={enabled ? 'success' : 'neutral'}
            size="lg"
            dot
            className="self-start sm:self-auto"
            data-testid="directory-status"
          >
            {enabled ? t('status.on') : t('status.off')}
          </Badge>
        )}
      </header>
      {settings.isPending && (
        <div className="flex flex-col gap-4" aria-busy>
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-72 w-full" />
          <Skeleton className="h-72 w-full" />
        </div>
      )}
      {settings.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void settings.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(settings.error)}
        </Callout>
      )}
      {settings.data && (
        <DirectoryForm
          // A new version (saved here or reloaded) starts a fresh draft.
          key={settings.data.etag ?? 'initial'}
          settings={settings.data.data}
          etag={settings.data.etag}
          onReload={() => void settings.refetch()}
        />
      )}
    </div>
  );
}

interface LastCheck {
  result: DirectoryConnectionTest;
  key: string;
}

function DirectoryForm({
  settings,
  etag,
  onReload,
}: {
  settings: DirectorySettings;
  etag: string | undefined;
  onReload: () => void;
}) {
  const { t } = useTranslation(['directory', 'common', 'auth']);
  const text = useLocalized();
  const initial = useMemo(() => toDraft(settings), [settings]);
  const [draft, setDraft] = useState<DirectoryDraft>(initial);
  const [showErrors, setShowErrors] = useState(false);
  const [serverErrors, setServerErrors] = useState<Partial<Record<string, string>>>({});
  const [stale, setStale] = useState(false);
  const [lastCheck, setLastCheck] = useState<LastCheck | null>(null);
  const [pendingPreset, setPendingPreset] = useState<DirectoryPreset | null>(null);
  const [userCheckOpen, setUserCheckOpen] = useState(false);
  const save = useSaveDirectorySettings();
  const test = useTestDirectoryConnection();

  const dirty = !sameDraft(draft, initial);
  const localErrors = draftErrors(draft, settings.plain_ldap_allowed);
  const key = connectionKey(draft);
  const tested = lastCheck !== null && lastCheck.result.ok && lastCheck.key === key;
  // The server's rule: sign-in turns on, or stays on after a change of the
  // connection, only with a successful check of the same connection.
  const connectionChanged = key !== connectionKey(initial);
  const needsProof = draft.enabled && (!settings.enabled || connectionChanged);
  const errorOf = (field: string): string | undefined => {
    const code = showErrors ? localErrors[field] : undefined;
    if (code) {
      const params =
        code === 'min'
          ? { value: 1 }
          : code === 'max'
            ? { value: 60 }
            : code === 'maxItems'
              ? { count: DIRECTORY_LIMITS.mappings }
              : undefined;
      return fieldErrorMessage({ field, code, ...(params ? { params } : {}) });
    }
    return serverErrors[field];
  };

  const update = (patch: Partial<DirectoryDraft>) => {
    setDraft((current) => ({ ...current, ...patch }));
    setServerErrors({});
  };

  const choosePreset = (preset: DirectoryPreset) => {
    if (preset === draft.preset) return;
    if (presetEdited(draft, settings.presets)) {
      setPendingPreset(preset);
      return;
    }
    const defaults = presetDefaults(settings.presets, preset);
    if (defaults) update(applyPreset(draft, defaults));
  };

  const runCheck = () => {
    if (Object.keys(localErrors).length > 0) {
      setShowErrors(true);
      return;
    }
    const checkedKey = key;
    test.mutate(toUpdate(draft), {
      onSuccess: (result) => {
        setLastCheck({ result, key: checkedKey });
      },
      onError: (err) => {
        if (err instanceof ApiError && err.fieldErrors.length > 0) {
          setShowErrors(true);
          setServerErrors(
            Object.fromEntries(err.fieldErrors.map((fe) => [fe.field, fieldErrorMessage(fe)])),
          );
          return;
        }
        toast.error(errorMessage(err));
      },
    });
  };

  const submit = () => {
    if (Object.keys(localErrors).length > 0) {
      setShowErrors(true);
      return;
    }
    save.mutate(
      { body: toUpdate(draft, tested ? lastCheck.result.proof : null), etag },
      {
        onSuccess: () => {
          toast.success(t('save.saved'));
        },
        onError: (err) => {
          if (err instanceof ApiError && err.code === 'precondition.failed') {
            setStale(true);
            return;
          }
          if (err instanceof ApiError && err.fieldErrors.length > 0) {
            setShowErrors(true);
            setServerErrors(
              Object.fromEntries(err.fieldErrors.map((fe) => [fe.field, fieldErrorMessage(fe)])),
            );
            return;
          }
          toast.error(errorMessage(err));
        },
      },
    );
  };

  const presetName = (preset: DirectoryPreset) => t(`preset.names.${preset}`);
  const baseExample = presetDefaults(settings.presets, draft.preset)?.base_dn_example ?? '';
  const label = text(draft.login_label) || t('auth:login.methods.directory');
  const steps = [
    { id: 'ask', done: settings.configured },
    { id: 'preset', done: settings.configured || draft.preset !== initial.preset },
    {
      id: 'connection',
      done:
        draft.url.trim() !== '' &&
        draft.bind_dn.trim() !== '' &&
        draft.user_base_dn.trim() !== '' &&
        (settings.has_bind_password || draft.bind_password !== ''),
    },
    { id: 'test', done: tested || (settings.enabled && !connectionChanged) },
    { id: 'enable', done: settings.enabled },
  ] as const;

  return (
    <>
      <form
        className="flex flex-col gap-6"
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        {stale && (
          <Callout
            tone="warning"
            title={t('save.staleTitle')}
            action={
              <Button size="sm" variant="secondary" leftIcon={<RotateCcw />} onClick={onReload}>
                {t('save.reload')}
              </Button>
            }
          >
            {t('save.staleText')}
          </Callout>
        )}

        <Card>
          <CardHeader>
            <CardTitle>{t('guide.title')}</CardTitle>
            <CardDescription>{t('guide.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent>
            <ol className="grid grid-cols-1 gap-3 lg:grid-cols-5" data-testid="directory-guide">
              {steps.map((step, index) => (
                <li
                  key={step.id}
                  className={cn(
                    'flex items-start gap-3 rounded-xl border p-3 text-sm lg:flex-col',
                    step.done
                      ? 'border-success-border bg-success-soft'
                      : 'border-border bg-surface',
                  )}
                >
                  <span className="flex shrink-0 items-center gap-2 font-semibold text-ink [&_svg]:size-4.5">
                    {step.done ? (
                      <CircleCheck aria-hidden className="text-success" />
                    ) : (
                      <Circle aria-hidden className="text-fg-faint" />
                    )}
                    {index + 1}
                  </span>
                  <span className="text-fg-muted">{t(`guide.steps.${step.id}`)}</span>
                </li>
              ))}
            </ol>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('preset.title')}</CardTitle>
            <CardDescription>{t('preset.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent>
            <RadioGroup
              value={draft.preset}
              onValueChange={(value) => {
                const preset = DIRECTORY_PRESETS.find((p) => p === value);
                if (preset) choosePreset(preset);
              }}
              className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4"
              aria-label={t('preset.title')}
            >
              {DIRECTORY_PRESETS.map((preset) => (
                <label
                  key={preset}
                  className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary-soft"
                >
                  <Radio value={preset} className="mt-0.5" />
                  <span className="flex flex-col gap-1">
                    <span className="font-semibold text-fg">{presetName(preset)}</span>
                    <span className="text-sm text-fg-muted">{t(`preset.texts.${preset}`)}</span>
                  </span>
                </label>
              ))}
            </RadioGroup>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('connection.title')}</CardTitle>
            <CardDescription>{t('connection.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <Field
              label={t('connection.url')}
              hint={t('connection.urlHint')}
              error={errorOf('url')}
              required
            >
              <Input
                value={draft.url}
                maxLength={DIRECTORY_LIMITS.url}
                spellCheck={false}
                autoCapitalize="none"
                autoComplete="off"
                className="font-mono"
                onChange={(event) => {
                  update({ url: event.target.value });
                }}
              />
            </Field>
            {isPlain(draft) && settings.plain_ldap_allowed && (
              <Callout tone="warning" title={t('connection.plainTitle')}>
                {t('connection.plainText')}
              </Callout>
            )}
            <SwitchRow
              label={t('connection.startTls')}
              hint={t('connection.startTlsHint')}
              checked={draft.start_tls}
              onChange={(start_tls) => {
                update({ start_tls });
              }}
            />
            <div className="grid grid-cols-1 gap-5 md:grid-cols-[minmax(0,1fr)_10rem]">
              <Field
                label={t('connection.ca')}
                hint={t('connection.caHint')}
                error={errorOf('ca_cert_pem')}
                optional
              >
                <Textarea
                  value={draft.ca_cert_pem}
                  maxLength={DIRECTORY_LIMITS.ca}
                  spellCheck={false}
                  autoGrow={false}
                  rows={4}
                  className="font-mono text-xs"
                  onChange={(event) => {
                    update({ ca_cert_pem: event.target.value });
                  }}
                />
              </Field>
              <Field
                label={t('connection.timeout')}
                hint={t('connection.timeoutHint')}
                error={errorOf('timeout_seconds')}
              >
                <Input
                  inputMode="numeric"
                  value={draft.timeout_seconds}
                  trailing={<span className="text-xs">{t('connection.seconds')}</span>}
                  onChange={(event) => {
                    update({ timeout_seconds: event.target.value.replace(/\D/g, '').slice(0, 2) });
                  }}
                />
              </Field>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('account.title')}</CardTitle>
            <CardDescription>{t('account.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-5 md:grid-cols-2">
            <Field
              label={t('account.bindDn')}
              hint={t('account.bindDnHint')}
              error={errorOf('bind_dn')}
              required
            >
              <Input
                value={draft.bind_dn}
                maxLength={DIRECTORY_LIMITS.dn}
                spellCheck={false}
                autoCapitalize="none"
                autoComplete="off"
                className="font-mono"
                onChange={(event) => {
                  update({ bind_dn: event.target.value });
                }}
              />
            </Field>
            <Field
              label={t('account.password')}
              hint={t('account.passwordHint')}
              error={errorOf('bind_password')}
              required={!settings.has_bind_password}
            >
              <Input
                type="password"
                value={draft.bind_password}
                maxLength={DIRECTORY_LIMITS.password}
                autoComplete="new-password"
                placeholder={settings.has_bind_password ? t('account.passwordSet') : ''}
                onChange={(event) => {
                  update({ bind_password: event.target.value });
                }}
              />
            </Field>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('search.title')}</CardTitle>
            <CardDescription>{t('search.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
              <Field
                label={t('search.baseDn')}
                hint={t('search.baseDnHint', { example: baseExample })}
                error={errorOf('user_base_dn')}
                required
              >
                <Input
                  value={draft.user_base_dn}
                  maxLength={DIRECTORY_LIMITS.dn}
                  spellCheck={false}
                  autoCapitalize="none"
                  autoComplete="off"
                  className="font-mono"
                  onChange={(event) => {
                    update({ user_base_dn: event.target.value });
                  }}
                />
              </Field>
              <Field
                label={t('search.filter')}
                hint={t('search.filterHint')}
                error={errorOf('user_filter')}
                required
              >
                <Input
                  value={draft.user_filter}
                  maxLength={DIRECTORY_LIMITS.filter}
                  spellCheck={false}
                  autoCapitalize="none"
                  autoComplete="off"
                  className="font-mono"
                  onChange={(event) => {
                    update({ user_filter: event.target.value });
                  }}
                />
              </Field>
            </div>
            <Accordion
              type="single"
              collapsible
              defaultValue={showErrors ? 'attributes' : undefined}
            >
              <AccordionItem value="attributes" className="border-t border-border">
                <AccordionTrigger>{t('search.attributes')}</AccordionTrigger>
                <AccordionContent className="flex flex-col gap-5 pt-1">
                  <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-4">
                    {(
                      [
                        ['login_attr', 'loginAttr', true],
                        ['email_attr', 'emailAttr', true],
                        ['email_fallback_attr', 'emailFallbackAttr', false],
                        ['name_attr', 'nameAttr', true],
                        ['given_name_attr', 'givenNameAttr', false],
                        ['surname_attr', 'surnameAttr', false],
                        ['id_attr', 'idAttr', true],
                        ['groups_attr', 'groupsAttr', true],
                      ] as const
                    ).map(([field, labelKey, needed]) => (
                      <Field
                        key={field}
                        label={t(`search.${labelKey}`)}
                        hint={field === 'id_attr' ? t('search.idAttrHint') : undefined}
                        error={errorOf(field)}
                        required={needed}
                        optional={!needed}
                      >
                        <Input
                          value={draft[field]}
                          maxLength={DIRECTORY_LIMITS.attribute}
                          spellCheck={false}
                          autoCapitalize="none"
                          autoComplete="off"
                          className="font-mono"
                          onChange={(event) => {
                            update({ [field]: event.target.value });
                          }}
                        />
                      </Field>
                    ))}
                  </div>
                  <SwitchRow
                    label={t('search.nestedGroups')}
                    hint={t('search.nestedGroupsHint')}
                    checked={draft.nested_groups}
                    onChange={(nested_groups) => {
                      update({ nested_groups });
                    }}
                  />
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('roles.title')}</CardTitle>
            <CardDescription>{t('roles.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            {draft.role_mappings.length === 0 ? (
              <p className="text-sm text-fg-muted">{t('roles.empty')}</p>
            ) : (
              <ul className="flex flex-col gap-3" aria-label={t('roles.title')}>
                {draft.role_mappings.map((mapping, index) => (
                  <li
                    key={mapping.key}
                    className="grid grid-cols-1 gap-3 rounded-xl border border-border p-3 md:grid-cols-[minmax(0,1fr)_14rem_auto] md:items-start"
                  >
                    <Field
                      label={t('roles.groupDn')}
                      error={errorOf(`role_mappings.${String(index)}.group_dn`)}
                    >
                      <Input
                        value={mapping.group_dn}
                        maxLength={DIRECTORY_LIMITS.dn}
                        spellCheck={false}
                        autoCapitalize="none"
                        autoComplete="off"
                        className="font-mono"
                        onChange={(event) => {
                          update({
                            role_mappings: draft.role_mappings.map((m) =>
                              m.key === mapping.key ? { ...m, group_dn: event.target.value } : m,
                            ),
                          });
                        }}
                      />
                    </Field>
                    <Field label={t('roles.role')}>
                      <RoleSelect
                        value={mapping.role}
                        onChange={(role) => {
                          update({
                            role_mappings: draft.role_mappings.map((m) =>
                              m.key === mapping.key ? { ...m, role } : m,
                            ),
                          });
                        }}
                      />
                    </Field>
                    <IconButton
                      label={t('roles.remove')}
                      icon={<Trash2 />}
                      className="justify-self-end md:mt-6 md:justify-self-auto"
                      onClick={() => {
                        update({
                          role_mappings: draft.role_mappings.filter((m) => m.key !== mapping.key),
                        });
                      }}
                    />
                  </li>
                ))}
              </ul>
            )}
            {draft.role_mappings.length < DIRECTORY_LIMITS.mappings && (
              <Button
                size="sm"
                variant="ghost"
                leftIcon={<Plus />}
                className="self-start"
                onClick={() => {
                  update({
                    role_mappings: [
                      ...draft.role_mappings,
                      { key: mappingKey(), group_dn: '', role: 'student' },
                    ],
                  });
                }}
              >
                {t('roles.add')}
              </Button>
            )}
            <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
              <Field label={t('roles.defaultRole')} hint={t('roles.defaultRoleHint')}>
                <RoleSelect
                  value={draft.default_role}
                  onChange={(default_role) => {
                    update({ default_role });
                  }}
                />
              </Field>
              <Field
                label={t('roles.requiredGroup')}
                hint={t('roles.requiredGroupHint')}
                error={errorOf('required_group_dn')}
                optional
              >
                <Input
                  value={draft.required_group_dn}
                  maxLength={DIRECTORY_LIMITS.dn}
                  spellCheck={false}
                  autoCapitalize="none"
                  autoComplete="off"
                  className="font-mono"
                  onChange={(event) => {
                    update({ required_group_dn: event.target.value });
                  }}
                />
              </Field>
            </div>
            <SwitchRow
              label={t('roles.autoProvision')}
              hint={t('roles.autoProvisionHint')}
              checked={draft.auto_provision}
              onChange={(auto_provision) => {
                update({ auto_provision });
              }}
            />
            <SwitchRow
              label={t('roles.syncRoles')}
              hint={t('roles.syncRolesHint')}
              checked={draft.sync_roles}
              onChange={(sync_roles) => {
                update({ sync_roles });
              }}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('page.title')}</CardTitle>
            <CardDescription>{t('page.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <LocalizedInputs
              label={t('page.label')}
              value={draft.login_label}
              maxLength={DIRECTORY_LIMITS.label}
              placeholder={t('auth:login.methods.directory')}
              errorOf={(lang) => errorOf(`login_label.${lang}`)}
              onChange={(login_label) => {
                update({ login_label });
              }}
            />
            <LocalizedInputs
              label={t('page.hint')}
              value={draft.username_hint}
              maxLength={DIRECTORY_LIMITS.hint}
              placeholder={t('auth:login.usernameHint')}
              errorOf={(lang) => errorOf(`username_hint.${lang}`)}
              onChange={(username_hint) => {
                update({ username_hint });
              }}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('check.title')}</CardTitle>
            <CardDescription>{t('check.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <div className="flex flex-wrap gap-3">
              <Button
                variant="secondary"
                leftIcon={<PlugZap />}
                loading={test.isPending}
                onClick={runCheck}
              >
                {t('check.connection')}
              </Button>
              <Button
                variant="secondary"
                leftIcon={<UserSearch />}
                onClick={() => {
                  if (Object.keys(localErrors).length > 0) {
                    setShowErrors(true);
                    return;
                  }
                  setUserCheckOpen(true);
                }}
              >
                {t('check.user')}
              </Button>
            </div>
            {lastCheck && (
              <div className="flex flex-col gap-3" aria-live="polite" data-testid="directory-check">
                <p
                  className={cn(
                    'flex items-center gap-2 text-sm font-semibold [&_svg]:size-4.5',
                    lastCheck.result.ok ? 'text-success' : 'text-danger',
                  )}
                >
                  {lastCheck.result.ok ? (
                    <CircleCheck aria-hidden />
                  ) : (
                    <TriangleAlert aria-hidden />
                  )}
                  {lastCheck.result.ok ? t('check.passed') : t('check.failed')}
                </p>
                {lastCheck.key !== key && <Callout tone="warning">{t('check.stale')}</Callout>}
                <StepList
                  steps={lastCheck.result.steps}
                  onUseBase={(dn) => {
                    update({ user_base_dn: dn });
                  }}
                />
              </div>
            )}
            <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface-raised p-4">
              <SwitchRow
                label={t('check.enable')}
                hint={t('check.enableHint', { label })}
                checked={draft.enabled}
                // Turning on needs a check; turning off never does.
                disabled={!draft.enabled && !tested}
                onChange={(enabled) => {
                  update({ enabled });
                }}
              />
              {!draft.enabled && !tested && (
                <p className="text-sm text-fg-muted">{t('check.enableNeedsTest')}</p>
              )}
              {needsProof && !tested && draft.enabled && settings.enabled && (
                <p className="text-sm text-warning-soft-fg">{t('check.changedNeedsTest')}</p>
              )}
            </div>
          </CardContent>
        </Card>

        <div
          className={cn(
            'flex flex-wrap items-center justify-end gap-3 rounded-xl border border-border bg-surface/90 p-3 backdrop-blur',
            dirty && 'sticky bottom-4 z-10 shadow-md',
          )}
        >
          {dirty && <p className="mr-auto text-sm text-fg-muted">{t('save.unsaved')}</p>}
          <Button
            type="button"
            variant="ghost"
            disabled={!dirty || save.isPending}
            onClick={() => {
              setDraft(initial);
              setShowErrors(false);
              setServerErrors({});
            }}
          >
            {t('common:actions.reset')}
          </Button>
          <Button type="submit" leftIcon={<Save />} loading={save.isPending} disabled={!dirty}>
            {t('common:actions.save')}
          </Button>
        </div>
      </form>
      {/* Outside the form: React passes events up through portals, and a
          submit inside a dialog must not save the settings. */}
      <ConfirmDialog
        open={pendingPreset !== null}
        onOpenChange={(open) => {
          if (!open) setPendingPreset(null);
        }}
        title={t('preset.confirmTitle')}
        description={
          pendingPreset ? t('preset.confirmText', { name: presetName(pendingPreset) }) : undefined
        }
        confirmLabel={t('preset.confirm')}
        onConfirm={() => {
          const defaults = pendingPreset
            ? presetDefaults(settings.presets, pendingPreset)
            : undefined;
          if (defaults) update(applyPreset(draft, defaults));
          setPendingPreset(null);
        }}
      />
      <UserCheckDialog
        open={userCheckOpen}
        onOpenChange={setUserCheckOpen}
        settings={() => toUpdate(draft)}
      />
    </>
  );
}

function SwitchRow({
  label,
  hint,
  checked,
  disabled = false,
  onChange,
}: {
  label: string;
  hint: ReactNode;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  const id = useId();
  return (
    <div className="flex items-start justify-between gap-4">
      <label htmlFor={id} className="flex flex-col gap-0.5">
        <span className="text-sm font-medium text-fg">{label}</span>
        <span className="text-sm text-fg-subtle">{hint}</span>
      </label>
      <Switch
        id={id}
        checked={checked}
        disabled={disabled}
        onCheckedChange={onChange}
        className="mt-0.5"
      />
    </div>
  );
}

function RoleSelect({ value, onChange }: { value: Role; onChange: (role: Role) => void }) {
  const { t } = useTranslation('common');
  return (
    <Select
      value={value}
      onValueChange={(next) => {
        const role = ROLES.find((r) => r === next);
        if (role) onChange(role);
      }}
    >
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {ROLES.map((role) => (
          <SelectItem key={role} value={role}>
            {t(`roles.${role}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function LocalizedInputs({
  label,
  value,
  maxLength,
  placeholder,
  errorOf,
  onChange,
}: {
  label: string;
  value: Record<Lang, string>;
  maxLength: number;
  placeholder: string;
  errorOf: (lang: Lang) => string | undefined;
  onChange: (value: Record<Lang, string>) => void;
}) {
  const { t } = useTranslation('directory');
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 text-sm font-medium text-fg">{label}</legend>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {LANGS.map((lang) => (
          <Field key={lang} label={t(`page.languages.${lang}`)} error={errorOf(lang)} optional>
            <Input
              value={value[lang]}
              maxLength={maxLength}
              placeholder={placeholder}
              lang={lang}
              onChange={(event) => {
                onChange({ ...value, [lang]: event.target.value });
              }}
            />
          </Field>
        ))}
      </div>
    </fieldset>
  );
}
