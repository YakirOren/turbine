#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DB_PATH="$PROJECT_ROOT/demo/pb_data/data.db"
OUTPUT="$SCRIPT_DIR/src/types/pocketbase-types.ts"

if [ ! -f "$DB_PATH" ]; then
  echo "Error: database not found at $DB_PATH"
  echo "Run the backend first to create the database."
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT")"
npx pocketbase-typegen --db "$DB_PATH" -o "$OUTPUT"
echo "Types generated at $OUTPUT"
