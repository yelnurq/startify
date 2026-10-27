-- +goose Up
-- A moderator's take-down holds a showcase page: until a moderator approves it
-- again the team cannot publish it, inside the university or outside.
ALTER TABLE showcase_entries ADD COLUMN held boolean NOT NULL DEFAULT false;

-- Whether a generated material was written from sensitive answers (unit
-- economics, the novelty and the unfair advantage of the solution), which a
-- project may hide from everyone outside its team. Unknown for materials
-- generated before: they count as sensitive.
ALTER TABLE project_materials ADD COLUMN sensitive boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE project_materials DROP COLUMN sensitive;
ALTER TABLE showcase_entries DROP COLUMN held;
