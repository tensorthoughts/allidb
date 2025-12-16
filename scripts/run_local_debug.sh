#!/usr/bin/env bash
set -euo pipefail

# Determine repo root (this script lives in scripts/)
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

echo "[allidb] Building debug binary (no external AI)..."
make quick-build

CONFIG_PATH="${ROOT_DIR}/configs/allidb.yaml"

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "[allidb] Config not found at $CONFIG_PATH"
  exit 1
fi

echo "[allidb] Starting AlliDB in DEBUG mode without external AI integration"
echo "  Config: $CONFIG_PATH"
echo "  Log level: DEBUG"
echo "  AI embeddings provider: local"
echo "  AI LLM provider: local"
echo

# Environment overrides:
# - Turn up log level
# - Override AI providers to "local" so no external calls are made
# - Optionally disable entity extraction
# export ALLIDB_OBSERVABILITY_LOG_LEVEL=DEBUG
# export ALLIDB_AI_EMBEDDINGS_PROVIDER=local
# export ALLIDB_AI_LLM_PROVIDER=local
# export ALLIDB_AI_ENTITY_EXTRACTION_ENABLED=false

"${ROOT_DIR}/build/allidbd" -config "$CONFIG_PATH"


