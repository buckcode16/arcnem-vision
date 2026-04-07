#!/bin/bash
set -e

echo "Generating migrations..."

bun run --cwd /app/packages/db db:generate

echo "Migrations generated successfully!"
