-- +goose Up
-- Codes of faculties, departments and study groups compare without regard to
-- case, as imports already read them: "IT" and "it" are one faculty, not two
-- (audit Н-9). citext makes the unique constraints and the lookups by code
-- case-insensitive; text converts to citext without rewriting the tables.
-- A university that already has codes differing only in case must merge them
-- first, or the unique constraints cannot be rebuilt.
ALTER TABLE faculties ALTER COLUMN code TYPE citext;
ALTER TABLE departments ALTER COLUMN code TYPE citext;
ALTER TABLE study_groups ALTER COLUMN code TYPE citext;

-- +goose Down
ALTER TABLE study_groups ALTER COLUMN code TYPE text;
ALTER TABLE departments ALTER COLUMN code TYPE text;
ALTER TABLE faculties ALTER COLUMN code TYPE text;
