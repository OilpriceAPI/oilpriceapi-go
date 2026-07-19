#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
server_pid=""

cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

go run "$root_dir/scripts/fixture-server" -url-file "$tmp_dir/fixture-url" >"$tmp_dir/server.log" 2>&1 &
server_pid=$!
for _ in {1..50}; do
  [[ -s "$tmp_dir/fixture-url" ]] && break
  sleep 0.1
done
[[ -s "$tmp_dir/fixture-url" ]] || { echo "fixture server did not start" >&2; exit 1; }
base_url="$(<"$tmp_dir/fixture-url")"

mkdir "$tmp_dir/consumer"
cp "$root_dir/example/main.go" "$tmp_dir/consumer/main.go"
pushd "$tmp_dir/consumer" >/dev/null
go mod init example-smoke >/dev/null
if [[ -n "${SDK_REF:-}" ]]; then
  go get "github.com/OilpriceAPI/oilpriceapi-go@${SDK_REF}" >/dev/null
else
  go mod edit -replace "github.com/OilpriceAPI/oilpriceapi-go=$root_dir"
  go get github.com/OilpriceAPI/oilpriceapi-go >/dev/null
fi

OILPRICEAPI_BASE_URL="$base_url" OILPRICEAPI_KEY="valid-smoke-key" go run . >"$tmp_dir/success.out" 2>&1
grep -q '^BRENT_CRUDE_USD 71.80 USD/barrel as of 2026-07-19T12:00:00Z (source: market_reporting)$' "$tmp_dir/success.out"

set +e
env -u OILPRICEAPI_KEY OILPRICEAPI_BASE_URL="$base_url" go run . >"$tmp_dir/missing.out" 2>&1
missing_status=$?
OILPRICEAPI_BASE_URL="$base_url" OILPRICEAPI_KEY="invalid-smoke-key" go run . >"$tmp_dir/auth.out" 2>&1
auth_status=$?
OILPRICEAPI_BASE_URL="$base_url" OILPRICEAPI_KEY="locked-smoke-key" go run . >"$tmp_dir/locked.out" 2>&1
locked_status=$?
OILPRICEAPI_BASE_URL="$base_url" OILPRICEAPI_KEY="limited-smoke-key" go run . >"$tmp_dir/limited.out" 2>&1
limited_status=$?
set -e

[[ "$missing_status" -ne 0 && "$auth_status" -ne 0 && "$locked_status" -ne 0 && "$limited_status" -ne 0 ]]
grep -q 'OILPRICEAPI_KEY is required' "$tmp_dir/missing.out"
grep -q 'authentication failed; replace OILPRICEAPI_KEY' "$tmp_dir/auth.out"
grep -q 'cannot access the requested dataset' "$tmp_dir/locked.out"
grep -q 'retry after 3 seconds' "$tmp_dir/limited.out"

if grep -R -E 'valid-smoke-key|invalid-smoke-key|locked-smoke-key|limited-smoke-key' "$tmp_dir"/*.out; then
  echo "example output exposed a credential" >&2
  exit 1
fi
popd >/dev/null

echo "clean-install example smoke passed (success, missing config, 401, 403, 429)"
