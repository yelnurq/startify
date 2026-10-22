-- name: GetTenantByDomain :one
SELECT t.*
  FROM tenant_domains d
  JOIN tenants t ON t.id = d.tenant_id
 WHERE d.domain = @domain;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = @slug;

-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = @id;

-- name: SetTenantBranding :exec
UPDATE tenants SET logo_file_id = @logo_file_id, cover_file_id = @cover_file_id WHERE id = @id;

-- name: GetTenantSettings :one
SELECT * FROM tenant_settings WHERE tenant_id = @tenant_id;

-- name: ListEnabledOidcProviders :many
SELECT id, name
  FROM tenant_oidc_providers
 WHERE tenant_id = @tenant_id AND enabled
 ORDER BY created_at;

-- name: GetOidcProvider :one
SELECT * FROM tenant_oidc_providers WHERE id = @id AND enabled;

-- name: UpsertTenant :one
INSERT INTO tenants (slug, name, short_name, default_locale, primary_color)
VALUES (@slug, @name, @short_name, @default_locale, @primary_color)
ON CONFLICT (slug) DO UPDATE
   SET name = EXCLUDED.name,
       short_name = EXCLUDED.short_name,
       default_locale = EXCLUDED.default_locale,
       primary_color = EXCLUDED.primary_color
RETURNING *;

-- name: UpsertTenantDomain :exec
INSERT INTO tenant_domains (domain, tenant_id, is_primary)
VALUES (@domain, @tenant_id, @is_primary)
ON CONFLICT (domain) DO UPDATE
   SET tenant_id = EXCLUDED.tenant_id, is_primary = EXCLUDED.is_primary;

-- name: UpsertTenantSettings :exec
INSERT INTO tenant_settings (tenant_id, self_signup, email_domains, consent_version, consent_text, ip_policy_version, ip_policy_text)
VALUES (@tenant_id, @self_signup, @email_domains, @consent_version, @consent_text, @ip_policy_version, @ip_policy_text)
ON CONFLICT (tenant_id) DO UPDATE
   SET self_signup = EXCLUDED.self_signup,
       email_domains = EXCLUDED.email_domains,
       consent_version = EXCLUDED.consent_version,
       consent_text = EXCLUDED.consent_text,
       ip_policy_version = EXCLUDED.ip_policy_version,
       ip_policy_text = EXCLUDED.ip_policy_text;

-- name: CreateOidcProvider :one
INSERT INTO tenant_oidc_providers (tenant_id, name, issuer, client_id, client_secret_enc, scopes, auto_provision)
VALUES (@tenant_id, @name, @issuer, @client_id, @client_secret_enc, @scopes, @auto_provision)
RETURNING id;

-- name: ListTenantsByIDs :many
SELECT t.id, t.slug, t.name, t.short_name, t.primary_color, t.logo_file_id, t.cover_file_id,
       coalesce((SELECT d.domain FROM tenant_domains d WHERE d.tenant_id = t.id AND d.is_primary LIMIT 1), '')::text AS primary_domain
  FROM tenants t
 WHERE t.id = ANY(@ids::uuid[]);
