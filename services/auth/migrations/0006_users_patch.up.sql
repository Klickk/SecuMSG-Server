CREATE EXTENSION IF NOT EXISTS citext;

ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS email           citext,
  ADD COLUMN IF NOT EXISTS email_verified  boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS username        citext,
  ADD COLUMN IF NOT EXISTS is_disabled     boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS updated_at      timestamptz NOT NULL DEFAULT now();

-- Uniqueness (match what your code expects)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'users_email_key' AND conrelid = 'public.users'::regclass
  ) THEN
    ALTER TABLE public.users ADD CONSTRAINT users_email_key UNIQUE (email);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'users_username_key' AND conrelid = 'public.users'::regclass
  ) THEN
    ALTER TABLE public.users ADD CONSTRAINT users_username_key UNIQUE (username);
  END IF;
END $$;