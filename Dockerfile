# syntax=docker/dockerfile:1.6

# ─── Stage 1: build the Next.js client to a static export ───────────────
FROM node:22-alpine AS client-builder
WORKDIR /app/client

# Install dependencies first (better layer caching)
COPY client/package.json client/package-lock.json ./
RUN npm ci

# Copy the rest of the client source and build
COPY client/ .
RUN npm run build
# `next build` with `output: 'export'` produces ./out


# ─── Stage 2: build the Go server binary ────────────────────────────────
FROM golang:1.24-alpine AS server-builder
WORKDIR /app/server

# Install dependencies first (better layer caching)
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy the rest of the server source and build a static binary
COPY server/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /spacetag .


# ─── Stage 3: tiny runtime image ────────────────────────────────────────
FROM alpine:3.20
WORKDIR /app

# Copy the Go binary and the static client files
COPY --from=server-builder /spacetag /app/spacetag
COPY --from=client-builder /app/client/out /app/client/out

# Railway sets PORT dynamically, but expose 8080 by default for local use
ENV PORT=8080
EXPOSE 8080

# The Go server reads PORT from the environment and serves the static client
ENTRYPOINT ["/app/spacetag", "-static", "/app/client/out"]
