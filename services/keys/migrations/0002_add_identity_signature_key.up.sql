ALTER TABLE identity_keys
    ADD COLUMN IF NOT EXISTS signature_key text;

UPDATE identity_keys
SET signature_key = public_key
WHERE signature_key IS NULL;

ALTER TABLE identity_keys
    ALTER COLUMN signature_key SET NOT NULL;
