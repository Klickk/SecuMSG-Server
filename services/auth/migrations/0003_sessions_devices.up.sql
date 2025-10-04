CREATE TABLE IF NOT EXISTS devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  platform text NOT NULL,
  push_token text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  revoked_at timestamptz
);

CREATE TABLE IF NOT EXISTS device_key_bundles (
  device_id uuid PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
  identity_key_pub bytea NOT NULL,
  signed_prekey_pub bytea NOT NULL,
  signed_prekey_sig bytea NOT NULL,
  one_time_prekeys jsonb NOT NULL,
  last_rotated_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id uuid NULL REFERENCES devices(id) ON DELETE SET NULL,
  refresh_id uuid NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL,
  ip inet,
  user_agent text
);
