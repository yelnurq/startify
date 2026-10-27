-- name: CreateDocument :one
INSERT INTO knowledge_documents (tenant_id, title, body, file_id, created_by)
VALUES (app_current_tenant(), @title, @body, @file_id, @created_by)
RETURNING *;

-- name: GetDocument :one
SELECT * FROM knowledge_documents WHERE id = @id;

-- name: ListDocuments :many
SELECT id, title, file_id, status, chunk_count, error, created_by, created_at, updated_at,
       length(body)::int AS body_length
  FROM knowledge_documents
 WHERE tenant_id = app_current_tenant()
   AND (sqlc.narg('before_id')::uuid IS NULL OR id < sqlc.narg('before_id')::uuid)
 ORDER BY id DESC
 LIMIT @row_limit;

-- name: DeleteDocument :one
DELETE FROM knowledge_documents WHERE id = @id RETURNING file_id;

-- name: SetDocumentReady :exec
UPDATE knowledge_documents SET status = 'ready', chunk_count = @chunk_count, error = NULL, index_failures = 0 WHERE id = @id;

-- name: SetDocumentFailed :exec
UPDATE knowledge_documents SET status = 'failed', error = @error, index_failures = index_failures + 1 WHERE id = @id;

-- name: FailStaleDocuments :execrows
-- New documents whose indexing job died would stay "indexing" forever.
UPDATE knowledge_documents SET status = 'failed', error = @error, index_failures = index_failures + 1
 WHERE status = 'pending' AND updated_at < @before;

-- name: DeleteChunks :exec
DELETE FROM knowledge_chunks WHERE document_id = @document_id;

-- name: InsertChunk :exec
INSERT INTO knowledge_chunks (tenant_id, document_id, ordinal, heading, content, embedding, embedding_model)
VALUES (app_current_tenant(), @document_id, @ordinal, @heading, @content, @embedding::vector, @embedding_model);

-- name: SearchChunks :many
SELECT c.id, c.document_id, d.title, c.heading, c.content,
       (c.embedding <=> @query::vector)::float8 AS distance
  FROM knowledge_chunks c
  JOIN knowledge_documents d ON d.id = c.document_id
 WHERE c.tenant_id = app_current_tenant() AND c.embedding_model = @embedding_model AND d.status = 'ready'
   -- Vectors of another size cannot be compared (a model reconfigured under the same name).
   AND vector_dims(c.embedding) = vector_dims(@query::vector)
 ORDER BY c.embedding <=> @query::vector
 LIMIT @row_limit;

-- name: SearchChunksText :many
SELECT c.id, c.document_id, d.title, c.heading, c.content,
       ts_rank(c.search, plainto_tsquery('simple', @query))::float8 AS rank
  FROM knowledge_chunks c
  JOIN knowledge_documents d ON d.id = c.document_id
 WHERE c.tenant_id = app_current_tenant() AND c.search @@ plainto_tsquery('simple', @query) AND d.status = 'ready'
 ORDER BY rank DESC
 LIMIT @row_limit;

-- name: TenantsWithDocuments :many
SELECT DISTINCT tenant_id FROM knowledge_documents ORDER BY tenant_id;

-- name: ListDocumentsForReindex :many
-- Documents without passages of the current model, and failed ones after a
-- pause until they failed max_failures times.
-- The hourly run takes a batch; the next one continues.
SELECT id FROM knowledge_documents d
 WHERE d.tenant_id = app_current_tenant()
   AND ((d.status = 'failed' AND d.updated_at < @failed_before AND d.index_failures < @max_failures)
    OR (d.status <> 'failed' AND (d.status <> 'ready' OR d.chunk_count > 0) AND NOT EXISTS (
         SELECT 1 FROM knowledge_chunks c WHERE c.document_id = d.id AND c.embedding_model = @embedding_model)))
 ORDER BY d.id
 LIMIT @row_limit;
