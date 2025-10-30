#!/usr/bin/env bash
set -euo pipefail

# This script runs during first cluster init.
# It uses psql to check for DBs and create them if missing,
# then enables extensions in each DB.

psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d postgres <<'SQL'
DO $$
BEGIN
  -- Make sure the owner role exists; in your setup POSTGRES_USER is 'app', so it's fine.
  -- If you use POSTGRES_USER=postgres, you can CREATE ROLE app here instead.

  -- We cannot CREATE DATABASE inside DO, so we only do checks here.
END$$;
SQL

# Helper to create a DB if it doesn't exist
create_db_if_missing() {
  local db="$1"
  if ! psql -U "${POSTGRES_USER}" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${db}'" | grep -q 1; then
    echo "Creating database ${db}"
    psql -U "${POSTGRES_USER}" -d postgres -c "CREATE DATABASE ${db} OWNER ${POSTGRES_USER};"
  else
    echo "Database ${db} already exists"
  fi
}

create_db_if_missing "authdb"
create_db_if_missing "keysdb"
create_db_if_missing "messagesdb"

# Enable common extensions in all DBs
for DB in authdb keysdb messagesdb; do
  psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${DB}" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"
  psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${DB}" -c "CREATE EXTENSION IF NOT EXISTS citext;"
done

echo "Init script completed."
