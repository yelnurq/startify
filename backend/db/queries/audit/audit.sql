-- name: InsertAuditEvent :exec
INSERT INTO audit_log (tenant_id, actor_id, action, target_type, target_id, ip, user_agent, metadata)
VALUES (@tenant_id, @actor_id, @action, @target_type, @target_id, @ip, @user_agent, @metadata);
