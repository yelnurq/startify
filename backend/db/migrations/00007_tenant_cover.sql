-- +goose Up
-- The university's photo for the hero of its public pages, next to its seal
-- (logo_file_id): both are public files of the university.
ALTER TABLE tenants ADD COLUMN cover_file_id uuid;

ALTER TABLE files DROP CONSTRAINT files_purpose_check;
ALTER TABLE files ADD CONSTRAINT files_purpose_check
  CHECK (purpose IN ('avatar', 'tenant_logo', 'tenant_cover', 'project_cover', 'attachment', 'knowledge', 'generated'));

-- +goose Down
DELETE FROM files WHERE purpose = 'tenant_cover';
ALTER TABLE files DROP CONSTRAINT files_purpose_check;
ALTER TABLE files ADD CONSTRAINT files_purpose_check
  CHECK (purpose IN ('avatar', 'tenant_logo', 'project_cover', 'attachment', 'knowledge', 'generated'));
ALTER TABLE tenants DROP COLUMN cover_file_id;
