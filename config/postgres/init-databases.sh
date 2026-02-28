#!/bin/bash
# ============================================================
# PostgreSQL init script — creates databases and users
# Runs once on first boot via /docker-entrypoint-initdb.d/
# ============================================================
set -euo pipefail

create_db_and_user() {
  local db="$1" user="$2" password="$3"
  echo "Creating database '$db' with user '$user'..."
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
    CREATE USER "$user" WITH PASSWORD '$password';
    CREATE DATABASE "$db" OWNER "$user";
    GRANT ALL PRIVILEGES ON DATABASE "$db" TO "$user";
SQL
}

create_db_and_user "keycloak" \
  "${KEYCLOAK_DB_USER:-keycloak}" \
  "${KEYCLOAK_DB_PASSWORD:-changeme-keycloak-db}"

create_db_and_user "mattermost" \
  "${MATTERMOST_DB_USER:-mattermost}" \
  "${MATTERMOST_DB_PASSWORD:-changeme-mattermost-db}"

create_db_and_user "wikijs" \
  "${WIKIJS_DB_USER:-wikijs}" \
  "${WIKIJS_DB_PASSWORD:-changeme-wikijs-db}"

echo "All databases and users created successfully."
