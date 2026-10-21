-- name: ListFaculties :many
SELECT * FROM faculties ORDER BY code;

-- name: ListDepartments :many
SELECT * FROM departments ORDER BY code;

-- name: GetFaculty :one
SELECT * FROM faculties WHERE id = @id;

-- name: GetDepartment :one
SELECT * FROM departments WHERE id = @id;

-- name: GetStudyGroup :one
SELECT * FROM study_groups WHERE id = @id;

-- name: GetFacultiesByIDs :many
SELECT * FROM faculties WHERE id = ANY(@ids::uuid[]);

-- name: GetDepartmentsByIDs :many
SELECT * FROM departments WHERE id = ANY(@ids::uuid[]);

-- name: GetStudyGroupsByIDs :many
SELECT * FROM study_groups WHERE id = ANY(@ids::uuid[]);

-- name: ListStudyGroups :many
SELECT * FROM study_groups
 WHERE (sqlc.narg('faculty_id')::uuid IS NULL OR faculty_id = sqlc.narg('faculty_id')::uuid)
   AND (sqlc.narg('department_id')::uuid IS NULL OR department_id = sqlc.narg('department_id')::uuid)
   AND (sqlc.narg('prefix')::text IS NULL OR lower(code) LIKE lower(sqlc.narg('prefix')::text) || '%')
 ORDER BY code
 LIMIT @row_limit;

-- name: FindFacultyByCode :one
SELECT * FROM faculties WHERE code = @code;

-- name: FindDepartmentByCode :one
SELECT * FROM departments WHERE code = @code;

-- name: FindStudyGroupByCode :one
SELECT * FROM study_groups WHERE code = @code;

-- name: UpsertFaculty :one
INSERT INTO faculties (tenant_id, code, name)
VALUES (app_current_tenant(), @code, @name)
-- Names merge by language: a file with only Russian names keeps the Kazakh ones.
ON CONFLICT (tenant_id, code) DO UPDATE SET name = faculties.name || EXCLUDED.name
RETURNING *;

-- name: UpsertDepartment :one
INSERT INTO departments (tenant_id, faculty_id, code, name)
VALUES (app_current_tenant(), @faculty_id, @code, @name)
ON CONFLICT (tenant_id, code) DO UPDATE
   SET faculty_id = EXCLUDED.faculty_id, name = departments.name || EXCLUDED.name
RETURNING *;

-- name: UpsertStudyGroup :one
INSERT INTO study_groups (tenant_id, faculty_id, department_id, code, study_level, admission_year)
VALUES (app_current_tenant(), @faculty_id, @department_id, @code, @study_level, @admission_year)
ON CONFLICT (tenant_id, code) DO UPDATE
   SET faculty_id = EXCLUDED.faculty_id,
       department_id = CASE
                         WHEN EXCLUDED.department_id IS NULL AND study_groups.faculty_id = EXCLUDED.faculty_id
                           THEN study_groups.department_id
                         ELSE EXCLUDED.department_id
                       END,
       study_level = EXCLUDED.study_level,
       admission_year = coalesce(EXCLUDED.admission_year, study_groups.admission_year)
RETURNING *;
