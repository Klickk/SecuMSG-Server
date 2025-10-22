UPDATE sessions
SET user_agent = substring(user_agent FROM 1 FOR 512)
WHERE user_agent IS NOT NULL AND length(user_agent) > 512;

UPDATE audit_logs
SET user_agent = substring(user_agent FROM 1 FOR 512)
WHERE user_agent IS NOT NULL AND length(user_agent) > 512;

ALTER TABLE sessions
    ADD CONSTRAINT chk_sessions_user_agent_length CHECK (user_agent IS NULL OR length(user_agent) <= 512);

ALTER TABLE audit_logs
    ADD CONSTRAINT chk_audit_logs_user_agent_length CHECK (user_agent IS NULL OR length(user_agent) <= 512);
