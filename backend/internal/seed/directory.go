package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/directory"
	"github.com/yelnurq/startify/backend/internal/directory/directorydb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// The demo directory of KazUTB: the OpenLDAP service of
// docker-compose.dev.yml, seeded from deploy/ldap (README, "Демо-каталог").
const (
	DemoDirectoryURL      = "ldap://openldap:389"
	DemoDirectoryBindDN   = "cn=qadam-svc,ou=Service Accounts,dc=kazutb,dc=local"
	DemoDirectoryPassword = "Qadam-Ldap-Bind-2026" //nolint:gosec // G101: the demo directory of deploy/ldap, development only
	DemoDirectoryBase     = "dc=kazutb,dc=local"
)

// DemoDirectoryRoles maps the groups of the demo directory to roles.
var DemoDirectoryRoles = []directory.RoleMapping{
	{GroupDN: "cn=admins,ou=Groups,dc=kazutb,dc=local", Role: "tenant_admin"},
	{GroupDN: "cn=innovation,ou=Groups,dc=kazutb,dc=local", Role: "innovation_manager"},
	{GroupDN: "cn=staff,ou=Groups,dc=kazutb,dc=local", Role: "supervisor"},
	{GroupDN: "cn=students,ou=Groups,dc=kazutb,dc=local", Role: "student"},
}

// Directory gives KazUTB settings of directory sign-in that point at the
// demo directory at url, turned off: the administrator checks the connection
// and turns it on. Settings saved before (by the administrator) stay.
func Directory(ctx context.Context, database *db.DB, keys *secrets.Keyring, url string) error {
	sealed, err := keys.Seal([]byte(DemoDirectoryPassword))
	if err != nil {
		return err
	}
	mappings, err := json.Marshal(DemoDirectoryRoles)
	if err != nil {
		return err
	}
	preset, _ := directory.PresetNamed(directory.PresetOpenLDAP)
	err = database.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		t, err := tenantdb.New(database.Conn(ctx)).GetTenantBySlug(ctx, KazUTB.Slug)
		if err != nil {
			return err
		}
		return database.Tx(db.WithTenant(ctx, t.ID), func(ctx context.Context) error {
			q := directorydb.New(database.Conn(ctx))
			if _, err := q.GetSettings(ctx); err == nil {
				return nil
			} else if !db.IsNoRows(err) {
				return err
			}
			_, err := q.UpsertSettings(ctx, directorydb.UpsertSettingsParams{
				Enabled: false, Preset: preset.Name, Url: url, TimeoutSeconds: 10,
				BindDn: DemoDirectoryBindDN, BindPasswordEnc: sealed, UserBaseDn: DemoDirectoryBase, UserFilter: preset.UserFilter,
				LoginAttr: preset.LoginAttr, EmailAttr: preset.EmailAttr, NameAttr: preset.NameAttr,
				GivenNameAttr: &preset.GivenNameAttr, SurnameAttr: &preset.SurnameAttr, IDAttr: preset.IDAttr,
				GroupsAttr: preset.GroupsAttr, NestedGroups: true, RoleMappings: mappings, DefaultRole: "student",
				AutoProvision: true,
				LoginLabel:    apiconv.Text{"kk": "ҚазТБУ есептік жазбасы", "ru": "Учётная запись КазУТБ", "en": "KazUTB account"}.JSON(),
				UsernameHint: apiconv.Text{
					"kk": "Университет поштасындағы логин, мысалы sarsenova.a",
					"ru": "Логин как в почте университета, например sarsenova.a",
					"en": "Your university mail login, e.g. sarsenova.a",
				}.JSON(),
			})
			return err
		})
	})
	if err != nil {
		return fmt.Errorf("seed: directory: %w", err)
	}
	return nil
}
