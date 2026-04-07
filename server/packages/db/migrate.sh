#!/bin/bash
set -euo pipefail

echo "Running migrations..."

# Retry logic for migrations in case Postgres isn't ready yet
MAX_RETRIES=30
RETRY_COUNT=0

is_retryable_error() {
  local output="$1"

  case "$output" in
    *"ECONNREFUSED"*|*"Connection refused"*|*"ENOTFOUND"*|*"no such host"*|*"could not connect to server"*|*"database system is starting up"*|*"timeout expired"*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

# Custom migration runner - bypasses drizzle-kit's broken dynamic import resolution
while true; do
  set +e
  OUTPUT=$(bun run /app/packages/db/runMigrations.ts 2>&1)
  EXIT_CODE=$?
  set -e

  echo "$OUTPUT"

  if [ $EXIT_CODE -eq 0 ]; then
    break
  fi

  RETRY_COUNT=$((RETRY_COUNT+1))

  if ! is_retryable_error "$OUTPUT"; then
    echo "Migration failed with a non-retryable error"
    exit $EXIT_CODE
  fi

  if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
    echo "Failed to run migrations after $MAX_RETRIES attempts"
    exit $EXIT_CODE
  fi

  echo "Attempt $RETRY_COUNT/$MAX_RETRIES: Migration failed (possibly waiting for Postgres), retrying in 2 seconds..."
  sleep 2
done

echo "Migrations completed successfully!"
