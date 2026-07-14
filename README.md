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
backend/    Go API, WebSocket hub, embedded migrations, seeder
frontend/   SvelteKit SPA
data/        creature data files (creatures.sample.json provided)
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

2. Start Postgres (any local instance) and point `DATABASE_URL` at it.

3. Seed the creature list (migrations run automatically):

   ```sh
   cd backend
   $env:DATABASE_URL="postgres://tww:tww@localhost:5432/tibia_warden?sslmode=disable"
   go run ./cmd/seed -file ../data/creatures.sample.json
   ```

4. Run the backend (loads env vars from your shell):

   ```sh
   cd backend
   go run ./cmd/server
   ```

5. Run the frontend:

   ```sh
   cd frontend
   npm install
   npm run dev
   ```

   Open http://localhost:5173. The Vite dev server proxies `/api` (including the
   WebSocket) to the Go backend on `:8080`.

## Seeding your own creature data

Provide a JSON array or a CSV with a header row:

- **JSON:** `[{ "name": "Dragon", "difficulty": "Medium", "imageUrl": "" }, ...]`
- **CSV:** columns `name,difficulty` (optional `imageUrl`/`image`).

Valid difficulties: `Harmless, Trivial, Easy, Medium, Hard, Challenging`
(case-insensitive). Re-running the seeder upserts by creature name.

```sh
go run ./cmd/seed -file ../data/creatures.json
```

## Production (Docker Compose)

On your Linux home server:

1. `cp .env.example .env` and set real values. Use your public HTTPS URL for
   `PUBLIC_BASE_URL`, `ALLOWED_ORIGINS`, and `DISCORD_REDIRECT_URL`
   (e.g. `https://warden.example.com/api/auth/discord/callback`), and set
   `COOKIE_SECURE=true`.

2. Build and start:

   ```sh
   docker compose up -d --build
   ```

   The app container serves the SPA, REST API, and WebSocket on port `8080`.
   Migrations run automatically on startup.

3. Seed creatures inside the running container:

   ```sh
   docker compose exec app /app/seed -file /app/data/creatures.json
   ```

   (Mount or copy your data file into the container first, or bake it into the image.)

4. Put a TLS-terminating reverse proxy (Caddy, nginx, or Traefik) in front of
   port `8080` for HTTPS. WebSocket upgrades on `/api/groups/{id}/ws` must be
   forwarded.

## Environment variables

See [.env.example](.env.example) for the full list with descriptions.
