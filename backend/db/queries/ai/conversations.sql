-- name: EnsureConversation :one
INSERT INTO ai_conversations (tenant_id, subject_type, subject_id, topic)
VALUES (app_current_tenant(), @subject_type, @subject_id, @topic)
ON CONFLICT (tenant_id, subject_type, subject_id, topic) DO UPDATE SET updated_at = now()
RETURNING *;

-- name: FindConversation :one
SELECT * FROM ai_conversations
 WHERE subject_type = @subject_type AND subject_id = @subject_id AND topic = @topic;

-- name: InsertMessage :one
INSERT INTO ai_messages (tenant_id, conversation_id, role, author_id, content, status, provider, model, prompt_version, meta)
VALUES (app_current_tenant(), @conversation_id, @role, sqlc.narg('author_id'), @content, @status,
        sqlc.narg('provider'), sqlc.narg('model'), sqlc.narg('prompt_version'), @meta)
RETURNING *;

-- name: ListMessages :many
SELECT * FROM ai_messages
 WHERE conversation_id = @conversation_id
   AND (sqlc.narg('before_id')::uuid IS NULL OR id < sqlc.narg('before_id')::uuid)
 ORDER BY id DESC
 LIMIT @row_limit;
