# libcat container image: lcatd (the backend) plus lcat (the projector CLI the
# rebuild command shells out to). Build from the repo root:
#
#   docker build -t libcat:dev .
#
# The image is the container/self-host deployment shape. It is a peer of the
# Lambda shape, not a dev convenience: same handler, same config, same probes.

# ---- stage 1: the cataloging SPA -------------------------------------------
# backend/ui/dist is go:embed'ed into lcatd. The dist/ committed to git is a
# placeholder, so this stage must run before the Go build or the binary ships
# an app shell that renders nothing.
FROM node:24-alpine AS ui

WORKDIR /ui
COPY backend/ui/package.json backend/ui/package-lock.json ./
RUN npm ci
COPY backend/ui/ ./
RUN npm run build

# ---- stage 2: the binaries --------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src
# The two modules resolve against each other from the working tree rather than
# the module proxy, so an image always contains the source it was built from.
# go.work is untracked (it carries a local roaringrange replace), so synthesize
# a clean one here instead of copying the developer's.
COPY go.mod go.sum ./
COPY backend/go.mod backend/go.sum ./backend/
RUN go work init . ./backend && go mod download && cd backend && go mod download

COPY . .
COPY --from=ui /ui/dist ./backend/ui/dist

# Fail loudly rather than embed the placeholder. A silently-empty admin UI is
# the kind of bug that survives a green build and every server-side test.
RUN grep -q 'lcat-ui-placeholder' backend/ui/dist/index.html \
    && { echo 'FATAL: backend/ui/dist is the committed placeholder, not the built SPA' >&2; exit 1; } \
    || echo 'ok: dist/ carries a real SPA build'

ARG VERSION=dev
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/lcatd ./backend/cmd/lcatd \
 && go build -trimpath -ldflags="-s -w" -o /out/lcat ./cmd/lcat

# ---- stage 3: the runtime ---------------------------------------------------
# distroless/static: no shell, no package manager, ca-certificates present for
# OIDC discovery, S3, and vocabulary downloads. Runs as nonroot (uid 65532).
# Note there is no HEALTHCHECK: the image has no shell to run one. Orchestrators
# probe over HTTP, which needs no in-container binary -- see docs/deploy.md.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/lcatd /usr/local/bin/lcatd
COPY --from=build /out/lcat /usr/local/bin/lcat

ENV LCATD_LISTEN_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/lcatd"]
