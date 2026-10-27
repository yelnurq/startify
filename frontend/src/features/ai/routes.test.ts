import { describe, expect, it } from 'vitest';

import type { AiProvider, AiSettings } from './api';
import {
  addRoute,
  draftErrors,
  effectiveRoutes,
  moveRoute,
  removeRoute,
  sameDraft,
  toDraft,
  toUpdate,
  usableRoutes,
} from './routes';

const providers: AiProvider[] = [
  { name: 'mock', local: true, models: { mentor: 'mock-mentor' } },
  { name: 'anthropic', local: false, models: { mentor: 'claude-sonnet-5-5' } },
  { name: 'openai', local: true, models: { mentor: 'qwen3' } },
];

const settings: AiSettings = {
  routes: { mentor: [{ provider: 'anthropic', model: 'claude-sonnet-5-5' }], summary: [] },
  default_routes: {
    mentor: [{ provider: 'mock' }],
    evaluation: [{ provider: 'anthropic' }, { provider: 'mock' }],
  },
  providers,
  local_only: false,
  daily_user_tokens: 200_000,
  daily_tenant_tokens: 20_000_000,
  mentor_mode: 'coach',
};

describe('settings draft', () => {
  it('keeps only own, non-empty routes and round-trips to an update', () => {
    const draft = toDraft(settings);
    expect(draft.routes).toEqual({
      mentor: [{ provider: 'anthropic', model: 'claude-sonnet-5-5' }],
    });
    expect(toUpdate(draft)).toEqual({
      routes: { mentor: [{ provider: 'anthropic', model: 'claude-sonnet-5-5' }] },
      local_only: false,
      daily_user_tokens: 200_000,
      daily_tenant_tokens: 20_000_000,
      mentor_mode: 'coach',
    });
  });

  it('ignores blank models and empty lists when comparing', () => {
    const draft = toDraft(settings);
    const edited = {
      ...draft,
      routes: { mentor: [{ provider: 'anthropic', model: ' claude-sonnet-5-5 ' }], generation: [] },
    };
    expect(sameDraft(draft, edited)).toBe(true);
    expect(sameDraft(draft, { ...draft, mentor_mode: 'coauthor' })).toBe(false);
  });

  it('falls back to the default chain', () => {
    const draft = toDraft(settings);
    expect(effectiveRoutes(draft, settings.default_routes, 'mentor').inherited).toBe(false);
    expect(effectiveRoutes(draft, settings.default_routes, 'evaluation')).toEqual({
      routes: [{ provider: 'anthropic' }, { provider: 'mock' }],
      inherited: true,
    });
    expect(effectiveRoutes(draft, settings.default_routes, 'generation').routes).toEqual([]);
  });

  it('drops external providers from the chain when only local models are allowed', () => {
    const routes = [{ provider: 'anthropic' }, { provider: 'gone' }, { provider: 'openai' }];
    expect(usableRoutes(routes, providers, false)).toEqual([
      { provider: 'anthropic' },
      { provider: 'openai' },
    ]);
    expect(usableRoutes(routes, providers, true)).toEqual([{ provider: 'openai' }]);
  });
});

describe('editing a chain', () => {
  const chain = [{ provider: 'anthropic' }, { provider: 'openai' }, { provider: 'mock' }];

  it('moves, removes and keeps the order otherwise', () => {
    expect(moveRoute(chain, 2, -1).map((r) => r.provider)).toEqual(['anthropic', 'mock', 'openai']);
    expect(moveRoute(chain, 0, -1)).toBe(chain);
    expect(moveRoute(chain, 2, 1)).toBe(chain);
    expect(removeRoute(chain, 0).map((r) => r.provider)).toEqual(['openai', 'mock']);
  });

  it('adds an unused provider, a local one when required, up to five', () => {
    expect(addRoute([{ provider: 'mock' }], providers, false)).toEqual([
      { provider: 'mock' },
      { provider: 'anthropic' },
    ]);
    expect(addRoute([{ provider: 'mock' }], providers, true)).toEqual([
      { provider: 'mock' },
      { provider: 'openai' },
    ]);
    const full = Array.from({ length: 5 }, () => ({ provider: 'mock' }));
    expect(addRoute(full, providers, false)).toBe(full);
  });
});

describe('draftErrors', () => {
  it('reports external providers under local-only, unknown providers and bad limits', () => {
    const draft = {
      ...toDraft(settings),
      local_only: true,
      routes: {
        mentor: [{ provider: 'anthropic' }],
        summary: [{ provider: 'gone' }],
      },
      daily_user_tokens: -1,
      daily_tenant_tokens: 1e12,
    };
    expect(draftErrors(draft, providers)).toEqual({
      'routes.mentor': 'external_not_allowed',
      'routes.summary': 'not_found',
      daily_user_tokens: 'invalid',
      daily_tenant_tokens: 'max',
    });
    expect(draftErrors(toDraft(settings), providers)).toEqual({});
  });

  it('rejects an embedding route to a provider without embeddings', () => {
    const withEmbeddings: AiProvider[] = [
      { name: 'mock', local: true, models: { mentor: 'mock-mentor', embedding: 'mock-hash-1024' } },
      { name: 'anthropic', local: false, models: { mentor: 'claude-sonnet-5-5' } },
    ];
    const draft = { ...toDraft(settings), routes: { embedding: [{ provider: 'anthropic' }] } };
    expect(draftErrors(draft, withEmbeddings)).toEqual({ 'routes.embedding': 'not_supported' });
    const fine = { ...toDraft(settings), routes: { embedding: [{ provider: 'mock' }] } };
    expect(draftErrors(fine, withEmbeddings)).toEqual({});
  });
});
