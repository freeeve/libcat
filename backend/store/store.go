// Package store defines the backend's document datastore: a composite-key
// (pk/sk) record store with optimistic-concurrency conditional writes, prefix
// queries, TTL, and atomic counters. The shape is the portable intersection of
// serverless document stores -- DynamoDB first (store/dynamo), with
// Firestore/Cosmos-style stores implementable later. Deliberately absent:
// transactions and secondary indexes. Services compose conditional puts
// instead of transactions and maintain explicit index items (e.g. a
// STATUS#<status> partition mirroring an aggregate's status); aggregates are
// the source of truth and index items are repairable.
//
// Single-table layout used by the backend services (one partition family per
// line; sk shapes are owned by each service):
//
//	WORK#<workId>    / SUGG#<scheme>#<term>#<ADD|REMOVE>   suggestion aggregates
//	WORK#<workId>    / SUPP#... (TTL)                      supporter dedup markers
//	WORK#<workId>    / REJ#<scheme>#<term>                 tombstones
//	STATUS#<status>  / <ts>#<workId>#...                   review-queue index items
//	FOLK#<normTerm>  / TERM                                folksonomy term lifecycle
//	AUDIT#<YYYY-MM>  / <ts>#<id>                           audit entries
//	RATE#<ipHash>    / <window> (counters, TTL)            rate limiting
//	USER#<email>     / PROFILE|CRED|ROLE#...               local users
//	DRAFT#<user>     / <draftId>                           editor drafts
//	JOB#EXPORT       / <jobId>                             export jobs
//	LEASE#ingest     / LOCK (TTL)                          advisory ingest lease
package store

import (
	"context"
	"errors"
	"iter"
	"time"
)

// ErrNotFound reports that no record exists at the requested key.
var ErrNotFound = errors.New("store: not found")

// ErrConditionFailed reports that a conditional Put or Delete lost its race.
// Callers recover by re-reading the record and retrying.
var ErrConditionFailed = errors.New("store: condition failed")

// Key is a record's composite key: PK selects the partition, SK orders and
// addresses records within it. Both must be non-empty.
type Key struct {
	PK string
	SK string
}

// Record is one stored document. Data is an opaque payload (services JSON-
// encode their domain types). Version is the optimistic-concurrency token:
// it is 0 for a record that has never been stored and increments on every
// successful Put. A zero ExpireAt means no TTL; expiry is lazy in stores with
// native TTL (DynamoDB deletes within ~a day), so services must treat
// ExpireAt as a floor on invisibility, not an exact deadline.
type Record struct {
	Key      Key
	Data     []byte
	Version  int64
	ExpireAt time.Time
}

// Cond selects a Put/Delete precondition.
type Cond int

const (
	// CondNone applies the write unconditionally.
	CondNone Cond = iota
	// CondIfAbsent succeeds only if no record exists at the key.
	CondIfAbsent
	// CondIfVersion succeeds only if the stored record's Version equals the
	// given Record.Version (for Put/Delete of an existing record) or, when
	// Record.Version is 0, if no record exists (create).
	CondIfVersion
)

// QueryOpt tunes a Query.
type QueryOpt struct {
	// Descending reverses the SK order (newest-first for timestamp SKs).
	Descending bool
	// Limit caps the number of records yielded; 0 means no cap.
	Limit int
	// StartAfter resumes a paginated query strictly after this SK.
	StartAfter string
}

// Store is the document datastore. Implementations must be safe for
// concurrent use.
type Store interface {
	// Get returns the record at k, or ErrNotFound.
	Get(ctx context.Context, k Key) (Record, error)
	// Put writes r subject to cond and returns the stored record with its
	// incremented Version. Violated conditions return ErrConditionFailed.
	Put(ctx context.Context, r Record, cond Cond) (Record, error)
	// Delete removes the record at r.Key subject to cond (CondIfVersion
	// compares r.Version). Deleting a missing record returns ErrNotFound.
	Delete(ctx context.Context, r Record, cond Cond) error
	// Query yields the partition pk's records whose SK starts with skPrefix,
	// in SK order (reversed by opt.Descending).
	Query(ctx context.Context, pk, skPrefix string, opt QueryOpt) iter.Seq2[Record, error]
	// Increment atomically adds delta to the counter at k and returns the
	// new value, creating the counter at delta if absent. A non-zero
	// expireAt sets the counter's TTL. Counter keys must not collide with
	// record keys.
	Increment(ctx context.Context, k Key, delta int64, expireAt time.Time) (int64, error)
}

func validateKey(k Key) error {
	if k.PK == "" || k.SK == "" {
		return errors.New("store: empty key component")
	}
	return nil
}

// expired reports whether a record with the given ExpireAt should be
// invisible at now (strict-TTL stores only).
func expired(expireAt time.Time, now time.Time) bool {
	return !expireAt.IsZero() && !expireAt.After(now)
}
