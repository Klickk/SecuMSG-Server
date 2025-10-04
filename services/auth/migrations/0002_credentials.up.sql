CREATE TABLE IF NOT EXISTS password_credentials (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  algo text NOT NULL,
  hash bytea NOT NULL,
  salt bytea NOT NULL,
  params_json jsonb NOT NULL,
  password_ver int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id bytea NOT NULL UNIQUE,
  public_key bytea NOT NULL,
  sign_count int NOT NULL DEFAULT 0,
  aaguid bytea,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
