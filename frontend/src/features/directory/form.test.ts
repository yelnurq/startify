import { describe, expect, it } from 'vitest';

import type { DirectoryPresetDefaults, DirectorySettings } from './api';
import {
  applyPreset,
  connectionKey,
  draftErrors,
  isPlain,
  presetDefaults,
  presetEdited,
  sameDraft,
  toDraft,
  toUpdate,
} from './form';

const activeDirectory: DirectoryPresetDefaults = {
  preset: 'active_directory',
  user_filter: '(&(objectCategory=person)(objectClass=user)(sAMAccountName={username}))',
  login_attr: 'sAMAccountName',
  email_attr: 'mail',
  email_fallback_attr: 'userPrincipalName',
  name_attr: 'displayName',
  given_name_attr: 'givenName',
  surname_attr: 'sn',
  id_attr: 'objectGUID',
  groups_attr: 'memberOf',
  nested_groups: true,
  base_dn_example: 'OU=Students,DC=kazutb,DC=local',
};
const openLDAP: DirectoryPresetDefaults = {
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
  nested_groups: false,
  base_dn_example: 'ou=Students,dc=kazutb,dc=local',
};
const presets = [activeDirectory, openLDAP];

const settings: DirectorySettings = {
  ...activeDirectory,
  enabled: false,
  preset: 'active_directory',
  url: 'ldaps://dc01.kazutb.local:636',
  start_tls: false,
  ca_cert_pem: null,
  timeout_seconds: 10,
  bind_dn: 'CN=qadam-svc,OU=Service Accounts,DC=kazutb,DC=local',
  user_base_dn: 'OU=Students,DC=kazutb,DC=local',
  role_mappings: [{ group_dn: 'CN=Admins,OU=Groups,DC=kazutb,DC=local', role: 'tenant_admin' }],
  default_role: 'student',
  required_group_dn: null,
  auto_provision: true,
  sync_roles: false,
  login_label: { ru: 'Учётная запись КазУТБ' },
  username_hint: null,
  has_bind_password: true,
  configured: true,
  plain_ldap_allowed: true,
  presets,
};

describe('directory form', () => {
  it('round-trips the settings and never sends an empty password', () => {
    const draft = toDraft(settings);
    const body = toUpdate(draft);
    expect(body).not.toHaveProperty('bind_password');
    expect(body).not.toHaveProperty('test_proof');
    expect(body.login_label).toEqual({ ru: 'Учётная запись КазУТБ' });
    expect(body.username_hint).toBeNull();
    expect(body.role_mappings).toEqual(settings.role_mappings);
    expect(sameDraft(draft, toDraft(settings))).toBe(true);

    const withPassword = toUpdate({ ...draft, bind_password: 'New-Bind-Password' }, 'proof');
    expect(withPassword.bind_password).toBe('New-Bind-Password');
    expect(withPassword.test_proof).toBe('proof');
  });

  it('fills a preset and notices edits of its fields', () => {
    const draft = toDraft(settings);
    expect(presetEdited(draft, presets)).toBe(false);
    expect(presetEdited({ ...draft, login_attr: 'userPrincipalName' }, presets)).toBe(true);
    // The connection is not part of a preset.
    expect(presetEdited({ ...draft, url: 'ldaps://dc02' }, presets)).toBe(false);

    const openldap = applyPreset(draft, openLDAP);
    expect(presetDefaults(presets, 'openldap')).toBe(openLDAP);
    expect(openldap).toMatchObject({
      preset: 'openldap',
      login_attr: 'uid',
      id_attr: 'entryUUID',
      email_fallback_attr: '',
      nested_groups: false,
      url: settings.url,
    });
  });

  it('keys a check by the connection only', () => {
    const draft = toDraft(settings);
    const key = connectionKey(draft);
    expect(connectionKey({ ...draft, login_label: { kk: 'x', ru: '', en: '' } })).toBe(key);
    expect(connectionKey({ ...draft, sync_roles: true })).toBe(key);
    expect(connectionKey({ ...draft, bind_password: 'other' })).not.toBe(key);
    expect(connectionKey({ ...draft, user_base_dn: 'OU=Staff,DC=kazutb,DC=local' })).not.toBe(key);
  });

  it('tells unencrypted connections and plain field errors', () => {
    const draft = toDraft(settings);
    expect(draftErrors(draft, false)).toEqual({});
    const plain = { ...draft, url: 'ldap://dc01.kazutb.local:389' };
    expect(isPlain(plain)).toBe(true);
    expect(isPlain({ ...plain, start_tls: true })).toBe(false);
    expect(draftErrors(plain, false)).toEqual({ url: 'plain_ldap' });
    expect(draftErrors(plain, true)).toEqual({});
    expect(
      draftErrors(
        {
          ...draft,
          url: 'https://dc01',
          user_filter: '(sAMAccountName=ivanov)',
          login_attr: 'sam account',
          timeout_seconds: '0',
          role_mappings: [{ key: 1, group_dn: ' ', role: 'student' }],
        },
        true,
      ),
    ).toEqual({
      url: 'ldap_url',
      user_filter: 'ldap_placeholder',
      login_attr: 'ldap_attribute',
      timeout_seconds: 'min',
      'role_mappings.0.group_dn': 'required',
    });
  });
});
