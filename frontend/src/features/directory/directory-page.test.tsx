import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { TooltipProvider } from '@/design-system';
import { loadNamespaces } from '@/shared/i18n';

import type { DirectoryConnectionTest, DirectorySettings, DirectoryUserTest } from './api';
import { DirectoryPage } from './directory-page';

const server = setupServer();

beforeAll(async () => {
  server.listen({ onUnhandledFrame: 'error' });
  await loadNamespaces('directory', 'auth');
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  server.close();
});

const preset = {
  preset: 'openldap',
  user_filter: '(&(objectClass=inetOrgPerson)(uid={username}))',
  login_attr: 'uid',
  email_attr: 'mail',
  email_fallback_attr: null,
  name_attr: 'cn',
  given_name_attr: 'givenName',
  surname_attr: 'sn',
  id_attr: 'entryUUID',
  groups_attr: 'memberOf',
  nested_groups: true,
  base_dn_example: 'ou=Students,dc=kazutb,dc=local',
} as const;

const settings: DirectorySettings = {
  ...preset,
  enabled: false,
  url: 'ldap://openldap:389',
  start_tls: false,
  ca_cert_pem: null,
  timeout_seconds: 10,
  bind_dn: 'cn=qadam-svc,ou=Service Accounts,dc=kazutb,dc=local',
  user_base_dn: 'dc=kazutb,dc=local',
  role_mappings: [{ group_dn: 'cn=students,ou=Groups,dc=kazutb,dc=local', role: 'student' }],
  default_role: 'student',
  required_group_dn: null,
  auto_provision: true,
  sync_roles: false,
  login_label: { ru: 'Учётная запись КазУТБ' },
  username_hint: null,
  has_bind_password: true,
  configured: true,
  plain_ldap_allowed: true,
  presets: [preset],
};

const check: DirectoryConnectionTest = {
  ok: true,
  proof: 'proof-1',
  steps: [
    { step: 'connect', status: 'ok', code: null, detail: null, server: 'ldap://openldap:389' },
    { step: 'tls', status: 'warning', code: 'plain', detail: null },
    { step: 'bind', status: 'ok', code: null, detail: null },
    { step: 'search', status: 'ok', code: null, detail: null, count: 1 },
    {
      step: 'server',
      status: 'ok',
      code: null,
      detail: null,
      vendor: 'OpenLDAP',
      naming_contexts: ['dc=kazutb,dc=local'],
    },
  ],
};

const user: DirectoryUserTest = {
  found: true,
  code: null,
  steps: [],
  user: {
    dn: 'uid=sarsenova.a,ou=Students,dc=kazutb,dc=local',
    login: 'sarsenova.a',
    email: 'aigerim@qadam.test',
    full_name: 'Айгерім Сәрсенова',
    subject: '88d7df42-631f-1041-8bd2-4fc106c5232c',
    groups: ['cn=students,ou=Groups,dc=kazutb,dc=local'],
    roles: ['student'],
    sign_in: { allowed: true, outcome: 'link', reasons: [] },
  },
};

function serve() {
  const saved: unknown[] = [];
  const looked: unknown[] = [];
  server.use(
    http.get('*/api/v1/admin/directory', () =>
      HttpResponse.json(settings, { headers: { ETag: '"4"' } }),
    ),
    http.put('*/api/v1/admin/directory', async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      saved.push(body);
      return HttpResponse.json(
        { ...settings, enabled: body.enabled },
        { headers: { ETag: '"5"' } },
      );
    }),
    http.post('*/api/v1/admin/directory/test', () => HttpResponse.json(check)),
    http.post('*/api/v1/admin/directory/test-user', async ({ request }) => {
      looked.push(await request.json());
      return HttpResponse.json(user);
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}
    >
      <TooltipProvider>
        <DirectoryPage />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  return { saved, looked, user: userEvent.setup() };
}

describe('DirectoryPage', () => {
  it('checks a user in a dialog without saving the settings', async () => {
    const { saved, looked, user: actor } = serve();
    await actor.click(await screen.findByRole('button', { name: 'Проверить пользователя' }));
    const dialog = await screen.findByRole('dialog');
    await actor.type(within(dialog).getByLabelText('Логин'), 'sarsenova.a{Enter}');

    expect(await within(dialog).findByText('Вход будет разрешён')).toBeInTheDocument();
    expect(
      within(dialog).getByText('uid=sarsenova.a,ou=Students,dc=kazutb,dc=local'),
    ).toBeInTheDocument();
    expect(looked).toHaveLength(1);
    expect(looked[0]).toMatchObject({ username: 'sarsenova.a', settings: { url: settings.url } });
    // Submitting the search inside the dialog does not submit the settings form.
    expect(saved).toHaveLength(0);
  });

  it('turns sign-in on only after a successful check and sends its proof', async () => {
    const { saved, user: actor } = serve();
    const toggle = await screen.findByRole('switch', { name: /Включить вход через каталог/ });
    expect(toggle).toBeDisabled();

    await actor.click(screen.getByRole('button', { name: 'Проверить подключение' }));
    expect(await screen.findByText('Подключение работает')).toBeInTheDocument();
    expect(screen.getByText('Сервер: OpenLDAP')).toBeInTheDocument();
    expect(toggle).toBeEnabled();

    await actor.click(toggle);
    await actor.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => {
      expect(saved).toHaveLength(1);
    });
    expect(saved[0]).toMatchObject({ enabled: true, test_proof: 'proof-1' });
    // The stored bind password is kept: none is sent.
    expect(saved[0]).not.toHaveProperty('bind_password');
  });
});
