#!/usr/bin/env bash
set -euo pipefail

: "${SUB2API_IMAGE:?}"
: "${EXPECTED_VERSION:?}"
repo_root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
smoke_dir=$(mktemp -d)
project="sub2api-smoke-${GITHUB_RUN_ID:-$$}-${GITHUB_RUN_ATTEMPT:-1}"
compose=(docker compose --project-name "$project" --env-file "$smoke_dir/.env" -f "$repo_root/deploy/fork/compose.yml")

cleanup() {
  status=$?
  if (( status != 0 )); then
    "${compose[@]}" ps || true
    "${compose[@]}" logs --tail=60 || true
  fi
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf -- "$smoke_dir"
  exit "$status"
}
trap cleanup EXIT

umask 077
{
  echo "SUB2API_IMAGE=$SUB2API_IMAGE"
  echo 'BIND_HOST=127.0.0.1'
  echo 'SERVER_PORT=18080'
  echo 'ADMIN_EMAIL=smoke@example.com'
  for key in POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY ADMIN_PASSWORD; do
    value=$(openssl rand -hex 32)
    if [[ -n ${GITHUB_ACTIONS:-} ]]; then echo "::add-mask::$value" >&2; fi
    echo "$key=$value"
  done
} > "$smoke_dir/.env"

"${compose[@]}" config --quiet
version_output=$(docker run --rm --entrypoint /app/sub2api "$SUB2API_IMAGE" --version 2>&1)
[[ "$version_output" == *"Sub2API $EXPECTED_VERSION "* ]]
"${compose[@]}" up -d --wait --wait-timeout 240
curl --fail --silent --show-error http://127.0.0.1:18080/health
curl --fail --silent --show-error http://127.0.0.1:18080/ > "$smoke_dir/index.html"
grep -qi '<html' "$smoke_dir/index.html"

# 重建数据库容器后仍须保留初始化的用户表，覆盖 PostgreSQL 18 的挂载路径。
users_before=$("${compose[@]}" exec -T postgres psql -U sub2api -d sub2api -Atqc 'SELECT count(*) FROM users')
[[ "$users_before" -ge 1 ]]
"${compose[@]}" up -d --no-deps --force-recreate --wait --wait-timeout 120 postgres
users_after=$("${compose[@]}" exec -T postgres psql -U sub2api -d sub2api -Atqc 'SELECT count(*) FROM users')
[[ "$users_before" == "$users_after" ]]
"${compose[@]}" up -d --no-deps --force-recreate --wait --wait-timeout 120 sub2api
curl --fail --silent --show-error http://127.0.0.1:18080/health
echo '镜像版本、初始化、页面和数据库持久化验证通过。'
