-- +goose Up
-- Sign-in through the university's directory (Active Directory or another
-- LDAP server), configured by the university administrator: one row per
-- university. The bind password is sealed with the keyring (AES-GCM) and is
-- never returned by the API.
CREATE TABLE tenant_directory_settings (
  tenant_id           uuid PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
  enabled             boolean NOT NULL DEFAULT false,
  preset              text NOT NULL CHECK (preset IN ('active_directory', 'openldap', 'freeipa', 'custom')),
  -- One or more ldap:// or ldaps:// addresses separated by spaces, tried in order.
  url                 text NOT NULL,
  start_tls           boolean NOT NULL DEFAULT false,
  -- Certificate of the university's certification authority (PEM), when the
  -- server certificate is not signed by a public one.
  ca_cert_pem         text,
  timeout_seconds     smallint NOT NULL DEFAULT 10 CHECK (timeout_seconds BETWEEN 1 AND 60),
  bind_dn             text NOT NULL,
  bind_password_enc   bytea,
  user_base_dn        text NOT NULL,
  -- LDAP filter with the {username} placeholder.
  user_filter         text NOT NULL,
  login_attr          text NOT NULL,
  email_attr          text NOT NULL,
  email_fallback_attr text,
  name_attr           text NOT NULL,
  given_name_attr     text,
  surname_attr        text,
  -- Permanent identifier of an entry (objectGUID, entryUUID, ipaUniqueID).
  id_attr             text NOT NULL,
  groups_attr         text NOT NULL DEFAULT 'memberOf',
  nested_groups       boolean NOT NULL DEFAULT false,
  -- [{"group_dn": "CN=Qadam Admins,OU=Groups,DC=kazutb,DC=local", "role": "tenant_admin"}, ...]
  role_mappings       jsonb NOT NULL DEFAULT '[]',
  default_role        text NOT NULL DEFAULT 'student' CHECK (default_role IN (
                        'student', 'supervisor', 'mentor', 'jury', 'innovation_manager',
                        'tenant_admin', 'leadership', 'partner')),
  required_group_dn   text,
  auto_provision      boolean NOT NULL DEFAULT true,
  sync_roles          boolean NOT NULL DEFAULT false,
  -- {"kk": "...", "ru": "Учётная запись КазУТБ", "en": "..."}; empty: the default label.
  login_label         jsonb NOT NULL DEFAULT '{}',
  username_hint       jsonb NOT NULL DEFAULT '{}',
  -- Optimistic locking: every saved change increments the version (ETag).
  version             integer NOT NULL DEFAULT 1,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CHECK (NOT enabled OR bind_password_enc IS NOT NULL)
);
CREATE TRIGGER tenant_directory_settings_updated_at BEFORE UPDATE ON tenant_directory_settings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('tenant_directory_settings');

-- +goose Down
DROP TABLE tenant_directory_settings;
