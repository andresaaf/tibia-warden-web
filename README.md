# Echo Warden Tracker

A self-hosted web app for a Tibia community to track personal **Echo Warden** kills
and coordinate live reveal announcements within groups.

- **Backend:** Go (chi, pgx, coder/websocket, Discord OAuth2)
- **Frontend:** SvelteKit (Svelte 5) SPA
- **Database:** PostgreSQL
- **Realtime:** WebSocket, one live room per group
- **Deploy:** Docker Compose (single app container serving API + WebSocket + the built SPA)

## Features

- **Discord login** — first sign-in prompts for a Tibia character name.
- **Groups** — public (discoverable) or private (join via one-time invite codes).
  Roles: owner, admin, member.
- **Warden List** — personal, global list of every creature; mark/unmark killed,
  search, and filter by difficulty (Harmless → Challenging).
- **Announcement room** — announce a reveal (creature, location, note); members
  respond **Coming** / **Ready** live; the author marks it **Killed**; then each
  present player clicks **I got it** to tick the creature on their own warden list
  and appear in the post's "got the kill" list.

## Project layout

```
backend/    Go API, WebSocket hub, embedded migrations, TibiaWiki creature sync, seeder
frontend/   SvelteKit SPA
data/        sample creature data for optional manual seeding (creatures.sample.json)
Dockerfile   multi-stage build (frontend + backend -> one image)
docker-compose.yml
.env.example
```

## Prerequisites

- Go 1.26+, Node 20+, and PostgreSQL 16 (or Docker for the compose setup).
- A Discord application: https://discord.com/developers/applications
  - Add an OAuth2 **Redirect** exactly matching `DISCORD_REDIRECT_URL`.
  - Copy the **Client ID** and **Client Secret** into your `.env`.

## Local development

1. Copy env and fill in Discord credentials + a random `SESSION_SECRET`:

   ```sh
   cp .env.example .env
   ```

   For dev, keep `DISCORD_REDIRECT_URL=http://localhost:5173/api/auth/discord/callback`
   (routed through the Vite proxy so the session cookie stays same-origin) and add
   that URL to your Discord app's redirects.

2. Start Postgres (any local instance) and point `DATABASE_URL` at it, e.g.:

   ```sh
   $env:DATABASE_URL="postgres://tww:tww@localhost:5432/tibia_warden?sslmode=disable"
   ```

3. Run the backend (loads env vars from your shell):

   ```sh
   cd backend
   go run ./cmd/server
   ```

   Migrations run automatically on startup, and the creature list is synced from
   the TibiaWiki API on each start — no manual seeding required (see
   [Creature data](#creature-data)).

4. Run the frontend:

   ```sh
   cd frontend
   npm install
   npm run dev
   ```

   Open http://localhost:5173. The Vite dev server proxies `/api` (including the
   WebSocket) to the Go backend on `:8080`.

## Creature data

By default the creature list is **synced automatically from the TibiaWiki API**
on every server start. The source is configured via `CREATURES_API_URL`
(default `https://tibiawiki.dev/api/creatures?expand=true`); the sync imports
creatures that have a bestiary difficulty and a Common/Uncommon occurrence, sets
their images, and safely prunes ones that no longer qualify (keeping any with
kill history or announcements). Restart the server to pick up game updates.

### Seeding a custom list instead (optional)

To manage the list yourself instead of the API sync, set `CREATURES_API_URL=`
(empty) to disable the sync, then load creatures with the seeder from a JSON
array or a CSV with a header row:

- **JSON:** `[{ "name": "Dragon", "difficulty": "Medium", "imageUrl": "" }, ...]`
- **CSV:** columns `name,difficulty` (optional `imageUrl`/`image`).

Valid difficulties: `Harmless, Trivial, Easy, Medium, Hard, Challenging`
(case-insensitive). Re-running the seeder upserts by creature name. A small
`data/creatures.sample.json` is included as a starting point.

```sh
go run ./cmd/seed -file ../data/creatures.json
```

## Production (Docker Compose)

The stack includes a **Caddy** reverse proxy that terminates HTTPS and
auto-provisions a Let's Encrypt certificate. Requirements: a domain whose DNS
points at the server, and ports **80** and **443** reachable from the internet.

On your Linux server:

1. `cp .env.example .env` and set real values:
   - `APP_DOMAIN` — your domain (e.g. `warden.example.com`).
   - `PUBLIC_BASE_URL`, `ALLOWED_ORIGINS` — `https://your-domain`.
   - `DISCORD_REDIRECT_URL` — `https://your-domain/api/auth/discord/callback`
     (add this exact URL to your Discord app's OAuth2 redirects).
   - `COOKIE_SECURE=true`.

2. Build and start:

   ```sh
   docker compose up -d --build
   ```

   Caddy obtains the certificate on first start and proxies HTTPS to the app
   (REST, SPA, and WebSocket). The app itself is not exposed on the host.
   Migrations run automatically on startup, and the creature list is synced from
   the TibiaWiki API — no seeding step required.

   To manage creatures manually instead (see [Creature data](#creature-data)),
   disable the sync with `CREATURES_API_URL=` and seed inside the container:

   ```sh
   docker compose exec app /app/seed -file /app/data/creatures.json
   ```

   (Mount or copy your data file into the container first, or bake it into the image.)

## Environment variables

See [.env.example](.env.example) for the full list with descriptions.
