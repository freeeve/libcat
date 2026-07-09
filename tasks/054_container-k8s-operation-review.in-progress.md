# 054 -- Review: container/k8s-style operation for the backend

## Context

The backend is deliberately Lambda-optional: `backend/cmd/lcatd` is a plain
net/http server (the Lambda command wraps the same handler), so it already
runs locally for testing with the in-memory datastore and a DirStore grain
tree -- no AWS. tasks/040 covers a docker-compose dev stack (lcatd + MinIO +
DynamoDB-local) and container *deployment docs*. This task is the fuller
review the maintainer asked for: whether and how to support first-class
container/k8s operation as a peer of the Lambda shape, not just a dev
convenience.

## Scope (review first, then implement what's applicable)

1. **Local-dev ergonomics now**: document the no-cloud loop (`LCATD_LOCAL_AUTH
   =1 LCATD_ABUSE_SECRET=... LCATD_BLOB_DIR=... go run ./cmd/lcatd`); decide
   whether a `lcatd --dev` flag should bundle sensible defaults (ephemeral
   key, mem store, bootstrap admin) into one switch.
2. **Container image**: multi-stage Dockerfile (static binary, distroless or
   alpine), image for lcatd + lcatd-worker; publish story (GHCR?).
3. **Compose dev stack** (overlaps 040 -- decide which task owns it): lcatd +
   MinIO + DynamoDB-local + a rebuild-webhook receiver stub; seed script for
   a fixture catalog.
4. **k8s readiness**: the API is stateless (all state in blob store +
   document store) and the ingest lease already provides single-flight
   coordination, so horizontal scaling should be safe -- verify. Add
   /v1/healthz-based liveness/readiness probes (readiness could check store
   connectivity), graceful-shutdown behavior under SIGTERM (exists), and
   decide manifests vs Helm chart vs "document kustomize examples only."
5. **Worker model outside Lambda**: lcatd-worker (or in-process goroutines)
   consuming publish/export/enrich work -- define how the SQS trigger maps to
   a container-native alternative (webhook receiver or store-polling loop) so
   k8s deployments need no AWS messaging.
6. **Secrets/config**: env-only today; review k8s secret mounting and
   whether file-based config (LCATD_CONFIG_FILE) is worth adding.
7. **Hugo-watcher dev loop (maintainer idea)**: exploit `hugo server`'s file
   watcher as the local preview of the whole editorial cycle. Shape: a local
   `trigger.Notifier` (e.g. `trigger.Command` or a small `lcatd --dev`
   built-in) that, on grains-changed, runs `lcat serialize && lcat project`
   into the running Hugo site's data dir -- the watcher sees catalog.json
   change and live-reloads, so an edit published in the cataloging UI appears
   in the discovery site within seconds, no cloud, no CI. Assessment so far:
   -- rebuild loop: YES, cheap and high-value; it is just a Notifier impl
      plus docs (hugo already watches the data dir).
   -- hosting the admin/review SPA *through* hugo: workable for integration
      demos (drop the built dist/ under static/admin/), but day-to-day SPA
      dev wants Vite's dev server (HMR) and production wants lcatd's
      go:embed; recommend documenting all three rather than making hugo the
      SPA host.
   Scope the Notifier impl + a `docs/local-dev.md` walkthrough here.
8. **S3 Tables (assessed 2026-07, maintainer question): not for current
   stores; candidate for a future analytics tier.** S3 Tables is managed
   Iceberg -- columnar/batch, engine-queried, seconds latency, no per-item
   conditional ops. The sidecar needs ms point reads + CAS (queue
   transitions, lease, refresh rotation): DynamoDB-class. Grains are
   path-addressed RDF documents needing ETag CAS and human/git readability:
   plain-S3-class. Revisit if/when these arrive: audit-trail analytics at
   consortium scale, availability-history time series (deeplibby-shaped,
   hundreds of millions of rows), or Parquet as an export format for
   data-science consumers.
9. **Decide scope**: which pieces are core-repo deliverables vs deployment-
   repo examples, and fold the result into tasks/040 or supersede it.

## Acceptance

- Written recommendation (in this task file or docs/) with the decided scope.
- Whatever is deemed applicable implemented: at minimum the Dockerfile,
  compose stack, probes, and the no-AWS worker path have concrete homes
  (here or 040).
