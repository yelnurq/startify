-- name: GetSettings :one
SELECT * FROM tenant_directory_settings WHERE tenant_id = app_current_tenant();

-- name: LockSettings :one
SELECT * FROM tenant_directory_settings WHERE tenant_id = app_current_tenant() FOR UPDATE;

-- name: UpsertSettings :one
-- A NULL bind_password_enc keeps the stored password; every change increments the version.
INSERT INTO tenant_directory_settings (
  tenant_id, enabled, preset, url, start_tls, ca_cert_pem, timeout_seconds, bind_dn, bind_password_enc,
  user_base_dn, user_filter, login_attr, email_attr, email_fallback_attr, name_attr, given_name_attr,
  surname_attr, id_attr, groups_attr, nested_groups, role_mappings, default_role, required_group_dn,
  auto_provision, sync_roles, login_label, username_hint
) VALUES (
  app_current_tenant(), @enabled, @preset, @url, @start_tls, @ca_cert_pem, @timeout_seconds, @bind_dn,
  sqlc.narg('bind_password_enc'), @user_base_dn, @user_filter, @login_attr, @email_attr, @email_fallback_attr,
  @name_attr, @given_name_attr, @surname_attr, @id_attr, @groups_attr, @nested_groups, @role_mappings,
  @default_role, @required_group_dn, @auto_provision, @sync_roles, @login_label, @username_hint
)
ON CONFLICT (tenant_id) DO UPDATE
   SET enabled = EXCLUDED.enabled,
       preset = EXCLUDED.preset,
       url = EXCLUDED.url,
       start_tls = EXCLUDED.start_tls,
       ca_cert_pem = EXCLUDED.ca_cert_pem,
       timeout_seconds = EXCLUDED.timeout_seconds,
       bind_dn = EXCLUDED.bind_dn,
       bind_password_enc = coalesce(EXCLUDED.bind_password_enc, tenant_directory_settings.bind_password_enc),
       user_base_dn = EXCLUDED.user_base_dn,
       user_filter = EXCLUDED.user_filter,
       login_attr = EXCLUDED.login_attr,
       email_attr = EXCLUDED.email_attr,
       email_fallback_attr = EXCLUDED.email_fallback_attr,
       name_attr = EXCLUDED.name_attr,
       given_name_attr = EXCLUDED.given_name_attr,
       surname_attr = EXCLUDED.surname_attr,
       id_attr = EXCLUDED.id_attr,
       groups_attr = EXCLUDED.groups_attr,
       nested_groups = EXCLUDED.nested_groups,
       role_mappings = EXCLUDED.role_mappings,
       default_role = EXCLUDED.default_role,
       required_group_dn = EXCLUDED.required_group_dn,
       auto_provision = EXCLUDED.auto_provision,
       sync_roles = EXCLUDED.sync_roles,
       login_label = EXCLUDED.login_label,
       username_hint = EXCLUDED.username_hint,
       version = tenant_directory_settings.version + 1
RETURNING *;
