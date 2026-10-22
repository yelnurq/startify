import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import {
  ArrowDown,
  ArrowUp,
  BarChart3,
  CircleStop,
  Pencil,
  Play,
  Plus,
  RotateCcw,
  Save,
  Trash2,
} from 'lucide-react';
import { useId, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  cn,
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
  Spinner,
  Switch,
  toast,
} from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { fieldErrorMessage } from '@/shared/forms/validation';
import { useFormatters } from '@/shared/i18n/format';

import { AiNotice } from './ai-notice';
import {
  AI_FEATURES,
  AI_LIMITS,
  aiSettingsQuery,
  MENTOR_MODES,
  useSaveAiSettings,
  type AiFeature,
  type AiProvider,
  type AiRoute,
  type AiSettings,
} from './api';
import { useAiCheck, type CheckState } from './check';
import { providerLabel } from './labels';
import {
  addRoute,
  draftErrors,
  effectiveRoutes,
  moveRoute,
  removeRoute,
  replaceRoute,
  sameDraft,
  toDraft,
  toUpdate,
  usableRoutes,
  type SettingsDraft,
} from './routes';

/** AI providers and models per feature, data policy and daily limits of the university. */
export function AiSettingsPage() {
  const { t } = useTranslation(['ai', 'common']);
  const settings = useQuery(aiSettingsQuery);
  const check = useAiCheck();
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-3xl font-semibold text-fg">{t('settings.title')}</h1>
          <p className="max-w-3xl text-fg-muted">{t('settings.subtitle')}</p>
        </div>
        <Button asChild variant="secondary" className="self-start sm:self-auto">
          <Link to="/app/admin/ai/usage">
            <BarChart3 aria-hidden />
            {t('settings.usageLink')}
          </Link>
        </Button>
      </header>
      <AiNotice />
      {settings.isPending && (
        <div className="flex flex-col gap-4" aria-busy>
          <Skeleton className="h-96 w-full" />
          <Skeleton className="h-64 w-full" />
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
        <SettingsForm
          // A new version (saved here or reloaded) starts a fresh draft.
          key={settings.data.etag ?? 'initial'}
          settings={settings.data.data}
          etag={settings.data.etag}
          check={check}
          onReload={() => void settings.refetch()}
        />
      )}
    </div>
  );
}

type Check = ReturnType<typeof useAiCheck>;

function SettingsForm({
  settings,
  etag,
  check,
  onReload,
}: {
  settings: AiSettings;
  etag: string | undefined;
  check: Check;
  onReload: () => void;
}) {
  const { t, i18n } = useTranslation(['ai', 'common']);
  const format = useFormatters();
  const initial = useMemo(() => toDraft(settings), [settings]);
  const [draft, setDraft] = useState<SettingsDraft>(initial);
  const [showErrors, setShowErrors] = useState(false);
  const [serverErrors, setServerErrors] = useState<Partial<Record<string, string>>>({});
  const [stale, setStale] = useState(false);
  const save = useSaveAiSettings();
  const dirty = !sameDraft(draft, initial);
  const localErrors = draftErrors(draft, settings.providers);
  const errorOf = (field: string): string | undefined => {
    const code = showErrors ? localErrors[field] : undefined;
    if (code) {
      const params =
        code === 'maxItems'
          ? { count: AI_LIMITS.routes }
          : code === 'max'
            ? {
                value: format.number(
                  field === 'daily_user_tokens'
                    ? AI_LIMITS.dailyUserTokens
                    : AI_LIMITS.dailyTenantTokens,
                ),
              }
            : undefined;
      return fieldErrorMessage({ field, code, ...(params ? { params } : {}) });
    }
    return serverErrors[field];
  };

  const update = (patch: Partial<SettingsDraft>) => {
    setDraft((current) => ({ ...current, ...patch }));
    setServerErrors({});
  };
  const setRoutes = (feature: AiFeature, routes: AiRoute[] | undefined) => {
    setDraft((current) => {
      const { [feature]: _previous, ...others } = current.routes;
      return { ...current, routes: routes ? { ...others, [feature]: routes } : others };
    });
    setServerErrors({});
  };

  const submit = () => {
    if (Object.keys(localErrors).length > 0) {
      setShowErrors(true);
      return;
    }
    save.mutate(
      { body: toUpdate(draft), etag },
      {
        onSuccess: () => {
          toast.success(t('settings.saved'));
        },
        onError: (err) => {
          if (err instanceof ApiError && err.code === 'precondition.failed') {
            setStale(true);
            return;
          }
          if (err instanceof ApiError && err.fieldErrors.length > 0) {
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

  const locale = i18n.language === 'kk' || i18n.language === 'en' ? i18n.language : 'ru';

  return (
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
          title={t('settings.staleTitle')}
          action={
            <Button size="sm" variant="secondary" leftIcon={<RotateCcw />} onClick={onReload}>
              {t('settings.reload')}
            </Button>
          }
        >
          {t('settings.staleText')}
        </Callout>
      )}

      <Card>
        <CardHeader>
          <CardTitle>{t('routes.title')}</CardTitle>
          <CardDescription>{t('routes.subtitle')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col divide-y divide-border p-0">
          {AI_FEATURES.map((feature) => (
            <FeatureRoutes
              key={feature}
              feature={feature}
              draft={draft}
              settings={settings}
              error={errorOf(`routes.${feature}`)}
              check={check}
              locale={locale}
              onChange={(routes) => {
                setRoutes(feature, routes);
              }}
            />
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('policy.title')}</CardTitle>
          <CardDescription>{t('policy.subtitle')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-6">
          <LocalOnlySwitch
            checked={draft.local_only}
            onChange={(local_only) => {
              update({ local_only });
            }}
          />
          <fieldset className="flex flex-col gap-3">
            <legend className="mb-1 text-sm font-medium text-fg">{t('policy.mentorMode')}</legend>
            <RadioGroup
              value={draft.mentor_mode}
              onValueChange={(value) => {
                update({ mentor_mode: value === 'coauthor' ? 'coauthor' : 'coach' });
              }}
              className="grid grid-cols-1 gap-3 md:grid-cols-2"
            >
              {MENTOR_MODES.map((mode) => (
                <label
                  key={mode}
                  className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary-soft"
                >
                  <Radio value={mode} className="mt-0.5" />
                  <span className="flex flex-col gap-1">
                    <span className="font-semibold text-fg">{t(`policy.modes.${mode}`)}</span>
                    <span className="text-sm text-fg-muted">{t(`policy.modes.${mode}Text`)}</span>
                  </span>
                </label>
              ))}
            </RadioGroup>
          </fieldset>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <TokenLimit
              label={t('policy.dailyUserTokens')}
              value={draft.daily_user_tokens}
              error={errorOf('daily_user_tokens')}
              onChange={(daily_user_tokens) => {
                update({ daily_user_tokens });
              }}
            />
            <TokenLimit
              label={t('policy.dailyTenantTokens')}
              value={draft.daily_tenant_tokens}
              error={errorOf('daily_tenant_tokens')}
              onChange={(daily_tenant_tokens) => {
                update({ daily_tenant_tokens });
              }}
            />
          </div>
        </CardContent>
      </Card>

      <div
        className={cn(
          'flex flex-wrap items-center justify-end gap-3 rounded-xl border border-border bg-surface/90 p-3 backdrop-blur',
          dirty && 'sticky bottom-4 z-10 shadow-md',
        )}
      >
        {dirty && <p className="mr-auto text-sm text-fg-muted">{t('settings.unsaved')}</p>}
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
  );
}

function LocalOnlySwitch({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  const { t } = useTranslation('ai');
  const id = useId();
  return (
    <div className="flex items-start justify-between gap-4">
      <label htmlFor={id} className="flex flex-col gap-0.5">
        <span className="text-sm font-medium text-fg">{t('policy.localOnly')}</span>
        <span className="text-sm text-fg-subtle">{t('policy.localOnlyHint')}</span>
      </label>
      <Switch id={id} checked={checked} onCheckedChange={onChange} className="mt-0.5" />
    </div>
  );
}

function TokenLimit({
  label,
  value,
  error,
  onChange,
}: {
  label: string;
  value: number;
  error: string | undefined;
  onChange: (value: number) => void;
}) {
  const { t } = useTranslation('ai');
  const format = useFormatters();
  return (
    <Field
      label={label}
      error={error}
      hint={
        value === 0
          ? t('policy.unlimited')
          : t('policy.tokensHint', { value: format.number(value) })
      }
    >
      <Input
        inputMode="numeric"
        autoComplete="off"
        value={String(value)}
        trailing={<span className="text-xs">{t('policy.tokens')}</span>}
        onChange={(event) => {
          const digits = event.target.value.replace(/\D/g, '');
          onChange(digits === '' ? 0 : Number(digits));
        }}
      />
    </Field>
  );
}

function FeatureRoutes({
  feature,
  draft,
  settings,
  error,
  check,
  locale,
  onChange,
}: {
  feature: AiFeature;
  draft: SettingsDraft;
  settings: AiSettings;
  error: string | undefined;
  check: Check;
  locale: 'kk' | 'ru' | 'en';
  onChange: (routes: AiRoute[] | undefined) => void;
}) {
  const { t } = useTranslation(['ai', 'common']);
  const { routes, inherited } = effectiveRoutes(draft, settings.default_routes, feature);
  const usable = usableRoutes(routes, settings.providers, draft.local_only);
  const state =
    check.state.status !== 'idle' && check.state.feature === feature ? check.state : null;
  const running = state?.status === 'running';
  const titleId = useId();

  return (
    <section
      aria-labelledby={titleId}
      className="flex flex-col gap-4 p-5"
      data-testid={`ai-feature-${feature}`}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <h4 id={titleId} className="font-semibold text-fg">
              {t(`features.${feature}`)}
            </h4>
            <Badge size="sm" tone={inherited ? 'neutral' : 'primary'}>
              {inherited ? t('routes.inherited') : t('routes.custom')}
            </Badge>
          </div>
          <p className="text-sm text-fg-muted">{t(`features.${feature}Text`)}</p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          {inherited ? (
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<Pencil />}
              onClick={() => {
                onChange(
                  routes.length > 0 ? routes : addRoute([], settings.providers, draft.local_only),
                );
              }}
            >
              {t('routes.customize')}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              leftIcon={<RotateCcw />}
              onClick={() => {
                onChange(undefined);
              }}
            >
              {t('routes.useDefault')}
            </Button>
          )}
          {running ? (
            <Button size="sm" variant="secondary" leftIcon={<CircleStop />} onClick={check.stop}>
              {t('check.stop')}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              leftIcon={<Play />}
              onClick={() => {
                check.start(feature, locale);
              }}
            >
              {t('check.run')}
            </Button>
          )}
        </div>
      </div>

      <ol
        className="flex flex-col gap-2"
        aria-label={t('routes.chain', { feature: t(`features.${feature}`) })}
      >
        {routes.map((route, index) => (
          <RouteRow
            // Routes have no identity of their own; the position is what the admin edits.
            key={index}
            feature={feature}
            index={index}
            count={routes.length}
            route={route}
            providers={settings.providers}
            localOnly={draft.local_only}
            readOnly={inherited}
            onChange={(next) => {
              onChange(replaceRoute(routes, index, next));
            }}
            onMove={(delta) => {
              onChange(moveRoute(routes, index, delta));
            }}
            onRemove={() => {
              const next = removeRoute(routes, index);
              onChange(next.length > 0 ? next : undefined);
            }}
          />
        ))}
      </ol>

      {!inherited && routes.length < AI_LIMITS.routes && (
        <Button
          size="sm"
          variant="ghost"
          leftIcon={<Plus />}
          className="self-start"
          onClick={() => {
            onChange(addRoute(routes, settings.providers, draft.local_only));
          }}
        >
          {t('routes.addFallback')}
        </Button>
      )}
      {error && (
        <p className="text-sm text-danger" role="alert">
          {error}
        </p>
      )}
      {usable.length === 0 && <Callout tone="warning">{t('routes.noneUsable')}</Callout>}
      {state && <CheckPanel state={state} />}
    </section>
  );
}

function RouteRow({
  feature,
  index,
  count,
  route,
  providers,
  localOnly,
  readOnly,
  onChange,
  onMove,
  onRemove,
}: {
  feature: AiFeature;
  index: number;
  count: number;
  route: AiRoute;
  providers: AiProvider[];
  localOnly: boolean;
  readOnly: boolean;
  onChange: (route: AiRoute) => void;
  onMove: (delta: -1 | 1) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation('ai');
  const provider = providers.find((item) => item.name === route.provider);
  const defaultModel = provider?.models[feature] ?? '';
  const external = provider !== undefined && !provider.local;
  const blocked = localOnly && external;
  const position = index === 0 ? t('routes.primary') : t('routes.fallback', { n: index });

  if (readOnly) {
    return (
      <li className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-surface-raised px-3 py-2 text-sm">
        <span className="text-xs font-medium text-fg-subtle">{position}</span>
        <span className={cn('font-medium text-fg', blocked && 'line-through opacity-60')}>
          {providerLabel(t, route.provider)}
        </span>
        <span className="font-mono text-xs break-all text-fg-muted">
          {route.model ?? defaultModel}
        </span>
        <Badge size="sm" tone={external ? 'warning' : 'success'}>
          {external ? t('routes.external') : t('routes.local')}
        </Badge>
      </li>
    );
  }

  return (
    <li className="grid grid-cols-1 gap-2 rounded-lg border border-border p-3 md:grid-cols-[7rem_minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center">
      <span className="text-xs font-medium text-fg-subtle">{position}</span>
      <Select
        value={route.provider}
        onValueChange={(name) => {
          onChange({ provider: name });
        }}
      >
        <SelectTrigger
          aria-label={t('routes.provider', { position })}
          aria-invalid={blocked || provider === undefined || undefined}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {providers.map((item) => (
            <SelectItem key={item.name} value={item.name} disabled={localOnly && !item.local}>
              {t('routes.providerOption', {
                name: providerLabel(t, item.name),
                kind: item.local ? t('routes.local') : t('routes.external'),
              })}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        value={route.model ?? ''}
        maxLength={AI_LIMITS.model}
        spellCheck={false}
        autoCapitalize="none"
        className="font-mono"
        aria-label={t('routes.model', { position })}
        placeholder={defaultModel ? t('routes.modelDefault', { model: defaultModel }) : ''}
        onChange={(event) => {
          onChange({ provider: route.provider, model: event.target.value });
        }}
      />
      <div className="flex items-center justify-end gap-1">
        <IconButton
          size="sm"
          label={t('routes.moveUp')}
          icon={<ArrowUp />}
          disabled={index === 0}
          onClick={() => {
            onMove(-1);
          }}
        />
        <IconButton
          size="sm"
          label={t('routes.moveDown')}
          icon={<ArrowDown />}
          disabled={index === count - 1}
          onClick={() => {
            onMove(1);
          }}
        />
        <IconButton size="sm" label={t('routes.remove')} icon={<Trash2 />} onClick={onRemove} />
      </div>
    </li>
  );
}

function CheckPanel({ state }: { state: Exclude<CheckState, { status: 'idle' }> }) {
  const { t } = useTranslation('ai');
  const format = useFormatters();
  return (
    <div
      className="flex flex-col gap-3 rounded-xl border border-primary-border bg-primary-soft/40 p-4"
      data-testid="ai-check"
      aria-live="polite"
      aria-busy={state.status === 'running'}
    >
      <div className="flex items-center gap-2 text-sm font-medium text-fg">
        {state.status === 'running' && <Spinner className="size-4 text-brand" />}
        <span>{t(`check.status.${state.status}`)}</span>
      </div>
      {state.text !== '' && (
        <p
          className="text-sm leading-relaxed whitespace-pre-wrap text-fg"
          data-testid="ai-check-text"
        >
          {state.text}
        </p>
      )}
      {state.status === 'done' && (
        <dl className="grid grid-cols-1 gap-x-6 gap-y-1 text-xs text-fg-muted sm:grid-cols-2">
          <div className="flex gap-1.5">
            <dt>{t('check.provider')}</dt>
            <dd className="font-medium text-fg">
              {state.result.provider ? providerLabel(t, state.result.provider) : '—'}
            </dd>
          </div>
          <div className="flex min-w-0 gap-1.5">
            <dt>{t('check.model')}</dt>
            <dd className="font-mono break-all text-fg">{state.result.model}</dd>
          </div>
          {state.result.prompt_version !== '' && (
            <div className="flex gap-1.5">
              <dt>{t('check.prompt')}</dt>
              <dd className="font-mono text-fg">{state.result.prompt_version}</dd>
            </div>
          )}
          {state.result.input_tokens + state.result.output_tokens > 0 && (
            <div className="flex gap-1.5">
              <dt>{t('check.tokens')}</dt>
              <dd className="text-fg tabular-nums">
                {t('check.tokensValue', {
                  input: format.number(state.result.input_tokens),
                  output: format.number(state.result.output_tokens),
                })}
              </dd>
            </div>
          )}
        </dl>
      )}
      {state.status === 'failed' && (
        <p className="text-sm text-danger" role="alert">
          {state.message}
        </p>
      )}
    </div>
  );
}
