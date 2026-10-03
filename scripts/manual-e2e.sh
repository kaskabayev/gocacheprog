#!/usr/bin/env bash
# Manual integration test for the S3 remote cache.
#
# Requires the docker compose stack from docker-compose.yml to be running:
#
#   docker compose up -d
#   ./scripts/manual-e2e.sh
#   docker compose down
#
# Simulates two machines sharing one S3 bucket:
#   A: empty local cache -> compiles and uploads
#   B: empty local cache -> must be served entirely from S3
#   C: S3 down           -> must still build (local-only degradation)
set -euo pipefail

cd "$(dirname "$0")/.."

ENDPOINT="${S3_ENDPOINT:-http://127.0.0.1:8333}"
BUCKET="${S3_BUCKET:-gobuilds}"
ACCESS_KEY="${S3_ACCESS_KEY:-gocachekey}"
SECRET_KEY="${S3_SECRET_KEY:-gocachesecret}"

fail() { echo "FAIL: $*" >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
root="$(pwd)"

echo "==> building gocacheprog"
go build -o "$work/gocache" .

echo "==> checking S3 at $ENDPOINT"
go run ./scripts/s3admin || fail "cannot reach bucket at $ENDPOINT (is the compose stack up?)"

# Fresh project with unique content per run so no prior cache can answer.
proj="$work/proj"
mkdir -p "$proj"
cd "$proj"
go mod init "e2e-$RANDOM" >/dev/null
cat > main.go <<EOF
package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// run $RANDOM makes every run unique.
func main() {
	fmt.Println(strings.ToUpper("hi"), http.StatusOK, sort.Ints)
}
EOF

machine() { echo "$work/gocache --cache-dir $1 --s3-bucket $BUCKET --s3-endpoint $ENDPOINT"; }
export AWS_ACCESS_KEY_ID="$ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$SECRET_KEY"

echo "==> machine A: cold build, compiles and uploads"
a="$(GOCACHEPROG="$(machine "$work/cacheA")" go build -v . 2>&1 | wc -l)"
echo "    packages compiled: $a"
[ "$a" -gt 0 ] || fail "machine A compiled nothing, expected a cold build"

echo "==> machine B: empty local cache, same bucket"
sleep 3 # give background uploads a moment on slow disks
b="$(GOCACHEPROG="$(machine "$work/cacheB")" go build -v . 2>&1 | wc -l)"
echo "    packages compiled: $b"
[ "$b" -eq 0 ] || fail "machine B recompiled $b packages, expected full S3 hits"

echo "PASS: machine B was served entirely from the remote cache"

if (cd "$root" && docker compose stop s3) >/dev/null 2>&1; then
	echo "==> machine C: S3 down, fresh cache must still build"
	c="$(GOCACHEPROG="$(machine "$work/cacheC")" go build -v . 2>&1 | wc -l)"
	GOCACHEPROG="$(machine "$work/cacheC")" go build . || fail "build failed with S3 down"
	echo "    packages compiled: $c (degraded to local-only, as designed)"
	(cd "$root" && docker compose start s3) >/dev/null
	echo "PASS: builds survive a dead remote cache"
else
	echo "SKIP: machine C degradation test (docker compose stack not found)"
fi
