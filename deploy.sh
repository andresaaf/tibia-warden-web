#!/usr/bin/env bash
# Deploy the latest main to this host. Run this on the server (e.g. over SSH):
#
#   ssh baz@192.168.1.2 '~/tibia-warden-web/deploy.sh'
#
# It fast-forwards the checkout to origin/main, rebuilds the app image
# (frontend SPA + Go server) and recreates only the app container. The db and
# caddy containers keep running untouched.
set -euo pipefail

# Work from the repo root regardless of where the script is invoked from.
cd "$(dirname "$0")"

# Pick whichever compose CLI is available.
if docker compose version >/dev/null 2>&1; then
	compose() { docker compose "$@"; }
elif docker-compose version >/dev/null 2>&1; then
	compose() { docker-compose "$@"; }
else
	echo "error: neither 'docker compose' nor 'docker-compose' is available" >&2
	exit 1
fi

echo "==> Fetching and fast-forwarding to origin/main"
git fetch origin main
git merge --ff-only origin/main

echo "==> Rebuilding and recreating the app container"
compose up -d --build app

echo "==> Status"
compose ps app

echo "==> Recent app logs"
compose logs app --tail 15

echo "==> Deployed $(git rev-parse --short HEAD)"
