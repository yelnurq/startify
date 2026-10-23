import type { Role } from '@/features/auth/api';

import {
  DIRECTORY_LIMITS,
  type DirectoryPreset,
  type DirectoryPresetDefaults,
  type DirectorySettings,
  type DirectorySettingsUpdate,
} from './api';

export const LANGS = ['kk', 'ru', 'en'] as const;
export type Lang = (typeof LANGS)[number];
type Texts = Record<Lang, string>;

/** A row of the role table; `key` only identifies the row while editing. */
export interface MappingDraft {
  key: number;
  group_dn: string;
  role: Role;
}

/** The form of the settings: every value as the administrator types it. */
export interface DirectoryDraft {
  enabled: boolean;
  preset: DirectoryPreset;
  url: string;
  start_tls: boolean;
  ca_cert_pem: string;
  timeout_seconds: string;
  bind_dn: string;
  /** A new bind password; empty keeps the stored one. */
  bind_password: string;
  user_base_dn: string;
  user_filter: string;
  login_attr: string;
  email_attr: string;
  email_fallback_attr: string;
  name_attr: string;
  given_name_attr: string;
  surname_attr: string;
  id_attr: string;
  groups_attr: string;
  nested_groups: boolean;
  role_mappings: MappingDraft[];
  default_role: Role;
  required_group_dn: string;
  auto_provision: boolean;
  sync_roles: boolean;
  login_label: Texts;
  username_hint: Texts;
}

/** Fields a preset fills. */
export const PRESET_FIELDS = [
  'user_filter',
  'login_attr',
  'email_attr',
  'email_fallback_attr',
  'name_attr',
  'given_name_attr',
  'surname_attr',
  'id_attr',
  'groups_attr',
  'nested_groups',
] as const satisfies readonly (keyof DirectoryDraft & keyof DirectoryPresetDefaults)[];

export const PLACEHOLDER = '{username}';

let nextKey = 0;
export function mappingKey(): number {
  nextKey += 1;
  return nextKey;
}

function texts(value: { kk?: string; ru?: string; en?: string } | null | undefined): Texts {
  return { kk: value?.kk ?? '', ru: value?.ru ?? '', en: value?.en ?? '' };
}

export function toDraft(s: DirectorySettings): DirectoryDraft {
  return {
    enabled: s.enabled,
    preset: s.preset,
    url: s.url,
    start_tls: s.start_tls,
    ca_cert_pem: s.ca_cert_pem ?? '',
    timeout_seconds: String(s.timeout_seconds),
    bind_dn: s.bind_dn,
    bind_password: '',
    user_base_dn: s.user_base_dn,
    user_filter: s.user_filter,
    login_attr: s.login_attr,
    email_attr: s.email_attr,
    email_fallback_attr: s.email_fallback_attr ?? '',
    name_attr: s.name_attr,
    given_name_attr: s.given_name_attr ?? '',
    surname_attr: s.surname_attr ?? '',
    id_attr: s.id_attr,
    groups_attr: s.groups_attr,
    nested_groups: s.nested_groups,
    role_mappings: s.role_mappings.map((m) => ({ key: mappingKey(), ...m })),
    default_role: s.default_role,
    required_group_dn: s.required_group_dn ?? '',
    auto_provision: s.auto_provision,
    sync_roles: s.sync_roles,
    login_label: texts(s.login_label),
    username_hint: texts(s.username_hint),
  };
}

const orNull = (value: string): string | null => (value.trim() === '' ? null : value.trim());

function localized(value: Texts): { kk?: string; ru?: string; en?: string } | null {
  const out: { kk?: string; ru?: string; en?: string } = {};
  for (const lang of LANGS) {
    const text = value[lang].trim();
    if (text !== '') out[lang] = text;
  }
  return Object.keys(out).length > 0 ? out : null;
}

/** The body of an update (and of the checks) for the draft. */
export function toUpdate(d: DirectoryDraft, testProof?: string | null): DirectorySettingsUpdate {
  const body: DirectorySettingsUpdate = {
    enabled: d.enabled,
    preset: d.preset,
    url: d.url.trim(),
    start_tls: d.start_tls,
    ca_cert_pem: orNull(d.ca_cert_pem),
    timeout_seconds: Number(d.timeout_seconds) || 0,
    bind_dn: d.bind_dn.trim(),
    user_base_dn: d.user_base_dn.trim(),
    user_filter: d.user_filter.trim(),
    login_attr: d.login_attr.trim(),
    email_attr: d.email_attr.trim(),
    email_fallback_attr: orNull(d.email_fallback_attr),
    name_attr: d.name_attr.trim(),
    given_name_attr: orNull(d.given_name_attr),
    surname_attr: orNull(d.surname_attr),
    id_attr: d.id_attr.trim(),
    groups_attr: d.groups_attr.trim(),
    nested_groups: d.nested_groups,
    role_mappings: d.role_mappings.map(({ group_dn, role }) => ({
      group_dn: group_dn.trim(),
      role,
    })),
    default_role: d.default_role,
    required_group_dn: orNull(d.required_group_dn),
    auto_provision: d.auto_provision,
    sync_roles: d.sync_roles,
    login_label: localized(d.login_label),
    username_hint: localized(d.username_hint),
  };
  // The password is never echoed back: send it only when a new one is typed.
  if (d.bind_password !== '') body.bind_password = d.bind_password;
  if (testProof) body.test_proof = testProof;
  return body;
}

export function presetDefaults(
  presets: DirectorySettings['presets'],
  name: DirectoryPreset,
): DirectoryPresetDefaults | undefined {
  return presets.find((p) => p.preset === name);
}

/** The draft with the filter and attributes of a preset. */
export function applyPreset(d: DirectoryDraft, p: DirectoryPresetDefaults): DirectoryDraft {
  return {
    ...d,
    preset: p.preset,
    user_filter: p.user_filter,
    login_attr: p.login_attr,
    email_attr: p.email_attr,
    email_fallback_attr: p.email_fallback_attr ?? '',
    name_attr: p.name_attr,
    given_name_attr: p.given_name_attr ?? '',
    surname_attr: p.surname_attr ?? '',
    id_attr: p.id_attr,
    groups_attr: p.groups_attr,
    nested_groups: p.nested_groups,
  };
}

/** Whether the administrator changed what the current preset filled. */
export function presetEdited(d: DirectoryDraft, presets: DirectorySettings['presets']): boolean {
  const p = presetDefaults(presets, d.preset);
  if (!p) return false;
  const filled = applyPreset(d, p);
  return PRESET_FIELDS.some((field) => filled[field] !== d[field]);
}

/**
 * What a connection check depends on (the server's own rule): a change of it
 * needs a new check before sign-in is turned on.
 */
export function connectionKey(d: DirectoryDraft): string {
  return JSON.stringify([
    d.url.trim().split(/[\s,]+/),
    d.start_tls,
    d.ca_cert_pem.trim(),
    d.bind_dn.trim(),
    d.bind_password,
    d.user_base_dn.trim(),
  ]);
}

export function sameDraft(a: DirectoryDraft, b: DirectoryDraft): boolean {
  const plain = (d: DirectoryDraft) => ({
    ...d,
    role_mappings: d.role_mappings.map(({ group_dn, role }) => ({ group_dn, role })),
  });
  return JSON.stringify(plain(a)) === JSON.stringify(plain(b));
}

/** Plain ldap:// without StartTLS sends passwords unencrypted. */
export function isPlain(d: DirectoryDraft): boolean {
  return (
    !d.start_tls &&
    d.url
      .trim()
      .split(/[\s,]+/)
      .some((u) => u.toLowerCase().startsWith('ldap://'))
  );
}

const URL_PATTERN = /^ldaps?:\/\/[^\s/?#@]+\/?$/i;
const ATTR_PATTERN = /^(?:[A-Za-z][A-Za-z0-9-]{0,63}|[012](?:\.\d+)+)$/;

/** Field errors the form can tell before asking the server (codes of common:validation). */
export function draftErrors(d: DirectoryDraft, plainAllowed: boolean): Record<string, string> {
  const errors: Record<string, string> = {};
  const required = (field: keyof DirectoryDraft, value: string) => {
    if (value.trim() === '') errors[field] = 'required';
  };
  const urls = d.url
    .trim()
    .split(/[\s,]+/)
    .filter(Boolean);
  if (urls.length === 0) errors.url = 'required';
  else if (urls.some((u) => !URL_PATTERN.test(u))) errors.url = 'ldap_url';
  else if (!plainAllowed && isPlain(d)) errors.url = 'plain_ldap';
  const timeout = Number(d.timeout_seconds);
  if (!Number.isInteger(timeout) || timeout < 1) errors.timeout_seconds = 'min';
  else if (timeout > 60) errors.timeout_seconds = 'max';
  required('bind_dn', d.bind_dn);
  required('user_base_dn', d.user_base_dn);
  if (d.user_filter.trim() === '') errors.user_filter = 'required';
  else if (!d.user_filter.includes(PLACEHOLDER)) errors.user_filter = 'ldap_placeholder';
  for (const field of [
    'login_attr',
    'email_attr',
    'name_attr',
    'id_attr',
    'groups_attr',
  ] as const) {
    if (d[field].trim() === '') errors[field] = 'required';
    else if (!ATTR_PATTERN.test(d[field].trim())) errors[field] = 'ldap_attribute';
  }
  for (const field of ['email_fallback_attr', 'given_name_attr', 'surname_attr'] as const) {
    if (d[field].trim() !== '' && !ATTR_PATTERN.test(d[field].trim()))
      errors[field] = 'ldap_attribute';
  }
  d.role_mappings.forEach((m, i) => {
    if (m.group_dn.trim() === '') errors[`role_mappings.${String(i)}.group_dn`] = 'required';
  });
  if (d.role_mappings.length > DIRECTORY_LIMITS.mappings) errors.role_mappings = 'maxItems';
  return errors;
}
