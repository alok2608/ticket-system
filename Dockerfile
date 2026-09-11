# ---- build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Dependencies first so Docker can cache them between builds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is disabled: the SQLite driver (modernc.org/sqlite) is pure Go,
# so the result is a single static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ticket-system .

# ---- run stage ----
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/ticket-system .

ENV GIN_MODE=release
# DB_PATH is intentionally left unset: it defaults to ./tickets.db, which
# resolves to /app/tickets.db under WORKDIR. Leaving it unset lets
# `docker run -e DB_PATH=...` or a mounted .env override the location.

EXPOSE 8080
CMD ["./ticket-system"]
