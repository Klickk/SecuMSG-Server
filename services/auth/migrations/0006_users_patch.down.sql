-- 0006_users_patch.down.sql
ALTER TABLE public.users
  DROP CONSTRAINT IF EXISTS users_username_key,
  DROP CONSTRAINT IF EXISTS users_email_key,
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS is_disabled,
  DROP COLUMN IF EXISTS username,
  DROP COLUMN IF EXISTS email_verified,
  DROP COLUMN IF EXISTS email;
