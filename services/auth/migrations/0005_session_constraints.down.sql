ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS chk_sessions_user_agent_length;

ALTER TABLE audit_logs
    DROP CONSTRAINT IF EXISTS chk_audit_logs_user_agent_length;
