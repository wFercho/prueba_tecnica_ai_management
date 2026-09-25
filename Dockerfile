# syntax=docker/dockerfile:1

# One image, one port. The API serves the built dashboard itself, so the browser
# talks to a single origin and the stack needs no reverse proxy and no CORS.
#
# The build is three stages because the two toolchains have nothing in common: Node
# turns the frontend into static files, Go turns the backend into a static binary,
# and the runtime image carries neither.

# ---------- frontend: React -> static files ----------
FROM node:24-alpine AS frontend
WORKDIR /src/frontend

# The lockfile is copied on its own so that a change in a component does not
# reinstall the dependency tree.
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm build

# ---------- backend: Go -> two static binaries ----------
FROM golang:1.27-alpine AS backend
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO off makes the binaries static, so the runtime image does not need libc.
# -trimpath and -s -w keep the build paths and the symbol table out of the image.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# ---------- tests: the same source, with the toolchain that checks it ----------
# A separate stage so that running the suite needs no Go on the host and no
# hand-written checkout path. `make test` uses it.
#
# The race detector needs cgo, so the stage carries a C toolchain. That is a
# test-only cost: the runtime image stays a static binary on a bare alpine.
FROM golang:1.27-alpine AS tests
RUN apk add --no-cache gcc musl-dev
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# The suite is not only unit tests: two of them read the delivered CSVs, one to
# assert the dataset yields the four expected cases and one to assert the default
# configuration points at that data. The data sits beside backend/ in the
# repository, so it is copied beside it here too and the paths under test are the
# real ones.
COPY data/ /src/data/
ENV CGO_ENABLED=1

# ---------- runtime ----------
FROM alpine:3.22 AS runtime

# ca-certificates: the narrator talks to the OpenAI API over TLS. tzdata: the log
# timestamps are rendered in the container's zone.
RUN apk add --no-cache ca-certificates tzdata

# A user that owns nothing and can write nowhere. The API holds no credentials on
# disk, so there is nothing for it to write.
RUN adduser -D -H -u 10001 app

WORKDIR /app
COPY --from=backend /out/api /app/api
COPY --from=backend /out/seed /app/seed
# The server serves the dashboard from here, and the seeder reads the delivered
# CSVs from /app/data. Both paths are what the configuration defaults to.
COPY --from=frontend /src/frontend/dist /app/frontend/dist
COPY data/ /app/data/

USER app
ENV PORT=8080 LOG_LEVEL=info
EXPOSE 8080

# A container that is up but not serving is not up. /meters needs no run to exist,
# so it answers as soon as the API is listening.
HEALTHCHECK --interval=5s --timeout=3s --start-period=3s --retries=5 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/meters" >/dev/null || exit 1

ENTRYPOINT ["/app/api"]
