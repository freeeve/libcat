# 165: Explore ethical (non-AWS) hosting for the backend

Explore running lcatd production off AWS in a green/ethical datacenter, using
the seams that already exist -- this should be configuration and ops work, not
code changes.

## What we already have

- `cmd/lcatd` is a first-class standalone/container entrypoint (the Lambda
  shape is secondary; writable-Lambda workers were deferred anyway, and a
  long-lived container runs the vocab/export drains natively).
- `LCATD_AWS_ENDPOINT` redirects both the S3 and DynamoDB clients to any
  compatible endpoint (blobs3 already does path-style for MinIO).
- Store is an interface: `store/dynamo` uses conditional puts/version checks
  but no TransactWriteItems, which stays inside what ScyllaDB Alternator
  (open-source DynamoDB-compatible API) supports. `store/mem` + snapshot is
  the small-scale fallback.
- Triggers sit behind `trigger.Trigger` (tasks/159); an in-process ticker or
  NATS replaces EventBridge/SQS.

## Candidate stack

lcatd container + MinIO or Garage for blobs + ScyllaDB/Alternator (or
mem+snapshot) for the store, `LCATD_AWS_ENDPOINT` pointed locally.

## Provider notes (researched 2026-07-08)

- Hetzner: hydro (DE) / wind+hydro (FI) directly sourced; cheapest; the only
  one with US locations -- Ashburn VA + Hillsboro OR, but those are
  cloud-VM-only rented colo: no object storage in US regions (self-host
  MinIO/Garage on volumes) and the renewable story is documented for the
  DE/FI parks, much less so for the US sites.
- Leafcloud (NL): strongest ethical-datacenter story (servers in residential
  buildings, waste heat warms their water); OpenStack + k8s. EU only.
- Infomaniak (CH): all-hydro, heat reuse, strong privacy posture; OpenStack.
  EU only.
- Scaleway (FR): 100% renewable, most AWS-shaped (S3-compatible storage,
  serverless containers). EU only.
- Context: Green Web Foundation stopped accepting carbon offsets as a
  fossil-free claim (2026-01), which favors direct-renewable providers over
  offset-heavy hyperscalers.

## Open questions

- Latency: if users/QLL are US-based, EU-only providers may be a real cost;
  Hetzner US is the compromise (with self-hosted object storage).
- Verify Alternator against the store contract: run `store/storetest` against
  a local ScyllaDB Alternator container (the same way dynamo is tested against
  DynamoDB-local).
- Backup/restore story for MinIO/Garage + Scylla off-AWS.
- Egress pricing comparison vs current AWS bill.

Deliverable: pick a provider, stand up a prototype deploy of the candidate
stack, document the recipe (compose file or small k8s manifest) under docs/.
