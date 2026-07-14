# syntax=docker/dockerfile:1

# --- Stage 1: build the SvelteKit SPA ---
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build the Go server ---
FROM golang:1.26-alpine AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed

# --- Stage 3: minimal runtime image ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=backend /out/seed /app/seed
COPY --from=frontend /app/frontend/build /app/static
COPY data/ /app/data/
ENV STATIC_DIR=/app/static
EXPOSE 8080
ENTRYPOINT ["/app/server"]
