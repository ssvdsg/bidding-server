#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
BACKEND_BIN="$ROOT_DIR/bidding-server"

if [ -f "/root/.nvm/nvm.sh" ]; then
  # Vite 8 requires Node 20.19+ / 22.12+
  # Use the installed Node 22 runtime when available.
  . "/root/.nvm/nvm.sh"
  nvm use 22 >/dev/null
fi
if [ -x "/usr/lib/go-1.21/bin/go" ]; then
  export PATH="/usr/lib/go-1.21/bin:$PATH"
fi

echo "==> [1/3] Build frontend"
cd "$FRONTEND_DIR"
if [ ! -d "node_modules" ]; then
  npm install --include=optional --no-audit --no-fund
fi
npm run build

echo "==> [2/3] Build backend"
cd "$ROOT_DIR"
go build -buildvcs=false -o "$BACKEND_BIN" .

echo "==> [3/3] Run backend"
exec "$BACKEND_BIN"
