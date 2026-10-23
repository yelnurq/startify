import {
  AI_FEATURES,
  AI_LIMITS,
  type AiFeature,
  type AiProvider,
  type AiRoute,
  type AiSettings,
  type AiSettingsUpdate,
  type MentorMode,
} from './api';

/** Editable copy of the AI settings; a feature without own routes uses the defaults. */
export interface SettingsDraft {
  routes: Partial<Record<AiFeature, AiRoute[]>>;
  local_only: boolean;
  daily_user_tokens: number;
  daily_tenant_tokens: number;
  mentor_mode: MentorMode;
}

function cleanRoute(route: AiRoute): AiRoute {
  const model = route.model?.trim();
  return model ? { provider: route.provider, model } : { provider: route.provider };
}

export function toDraft(settings: AiSettings): SettingsDraft {
  const routes: SettingsDraft['routes'] = {};
  for (const feature of AI_FEATURES) {
    const own = settings.routes[feature];
    if (own && own.length > 0) routes[feature] = own.map(cleanRoute);
  }
  return {
    routes,
    local_only: settings.local_only,
    daily_user_tokens: settings.daily_user_tokens,
    daily_tenant_tokens: settings.daily_tenant_tokens,
    mentor_mode: settings.mentor_mode,
  };
}

export function toUpdate(draft: SettingsDraft): AiSettingsUpdate {
  const routes: AiSettingsUpdate['routes'] = {};
  for (const feature of AI_FEATURES) {
    const own = draft.routes[feature];
    if (own && own.length > 0) routes[feature] = own.map(cleanRoute);
  }
  return { ...draft, routes };
}

/** Same settings, ignoring blank models and empty route lists. */
export function sameDraft(a: SettingsDraft, b: SettingsDraft): boolean {
  return JSON.stringify(toUpdate(a)) === JSON.stringify(toUpdate(b));
}

/** The chain a feature uses: its own routes or the server defaults. */
export function effectiveRoutes(
  draft: SettingsDraft,
  defaults: AiSettings['default_routes'],
  feature: AiFeature,
): { routes: AiRoute[]; inherited: boolean } {
  const own = draft.routes[feature];
  if (own && own.length > 0) return { routes: own, inherited: false };
  return { routes: defaults[feature] ?? [], inherited: true };
}

export function isLocal(providers: AiProvider[], name: string): boolean {
  return providers.find((provider) => provider.name === name)?.local ?? false;
}

/** A provider serves embeddings when it has an embedding model (Anthropic has none). */
export function embeds(providers: AiProvider[], name: string): boolean {
  return Boolean(providers.find((provider) => provider.name === name)?.models.embedding);
}

/** Routes that will actually be tried: unknown and, when local-only, external providers drop out. */
export function usableRoutes(
  routes: AiRoute[],
  providers: AiProvider[],
  localOnly: boolean,
): AiRoute[] {
  return routes.filter((route) => {
    const provider = providers.find((item) => item.name === route.provider);
    return provider !== undefined && (!localOnly || provider.local);
  });
}

export function moveRoute(routes: AiRoute[], index: number, delta: -1 | 1): AiRoute[] {
  const target = index + delta;
  if (target < 0 || target >= routes.length) return routes;
  const next = [...routes];
  const [item] = next.splice(index, 1);
  if (item) next.splice(target, 0, item);
  return next;
}

export function removeRoute(routes: AiRoute[], index: number): AiRoute[] {
  return routes.filter((_, i) => i !== index);
}

export function replaceRoute(routes: AiRoute[], index: number, route: AiRoute): AiRoute[] {
  return routes.map((item, i) => (i === index ? route : item));
}

/** Appends a fallback: the first provider not in the chain yet (local ones first when required). */
export function addRoute(
  routes: AiRoute[],
  providers: AiProvider[],
  localOnly: boolean,
): AiRoute[] {
  if (routes.length >= AI_LIMITS.routes) return routes;
  const candidates = providers.filter((provider) => !localOnly || provider.local);
  const next =
    candidates.find((provider) => !routes.some((route) => route.provider === provider.name)) ??
    candidates[0];
  return next ? [...routes, { provider: next.name }] : routes;
}

/** Field errors of a draft, keyed like the server's (`routes.mentor`, `daily_user_tokens`). */
export function draftErrors(
  draft: SettingsDraft,
  providers: AiProvider[],
): Partial<Record<string, string>> {
  const errors: Partial<Record<string, string>> = {};
  for (const feature of AI_FEATURES) {
    const own = draft.routes[feature] ?? [];
    if (own.length > AI_LIMITS.routes) errors[`routes.${feature}`] = 'maxItems';
    else if (own.some((route) => !providers.some((item) => item.name === route.provider))) {
      errors[`routes.${feature}`] = 'not_found';
    } else if (draft.local_only && own.some((route) => !isLocal(providers, route.provider))) {
      errors[`routes.${feature}`] = 'external_not_allowed';
    } else if (feature === 'embedding' && own.some((route) => !embeds(providers, route.provider))) {
      errors[`routes.${feature}`] = 'not_supported';
    } else if (own.some((route) => (route.model?.trim().length ?? 0) > AI_LIMITS.model)) {
      errors[`routes.${feature}`] = 'too_long';
    }
  }
  const limit = (value: number, max: number) =>
    !Number.isInteger(value) || value < 0 ? 'invalid' : value > max ? 'max' : undefined;
  const user = limit(draft.daily_user_tokens, AI_LIMITS.dailyUserTokens);
  if (user) errors.daily_user_tokens = user;
  const tenant = limit(draft.daily_tenant_tokens, AI_LIMITS.dailyTenantTokens);
  if (tenant) errors.daily_tenant_tokens = tenant;
  return errors;
}
