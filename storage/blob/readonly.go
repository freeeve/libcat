package blob

import (
	"context"
	"errors"
)

// ErrReadOnly reports a write attempted against a read-only store.
var ErrReadOnly = errors.New("blob: store is read-only")

// readOnly wraps a Store so every mutation fails with ErrReadOnly while reads
// (Get, List) pass straight through. It backs the deployment-wide read-only
// demo mode: the grain store and everything else that lives in blob (editing
// profiles, vocabulary snapshots, authority grains, export artifacts) becomes
// immutable, so a public playground can be explored without persisting.
//
// It deliberately does not forward the Signer capability: a wrapped store never
// advertises presigned GETs, so callers fall back to serving bytes -- correct,
// just less efficient. Reads still work through Get.
type readOnly struct{ Store }

// ReadOnly returns a read-only view of s: Get and List are served from s;
// Put and Delete return ErrReadOnly.
func ReadOnly(s Store) Store { return readOnly{s} }

func (readOnly) Put(context.Context, string, []byte, PutOptions) (string, error) {
	return "", ErrReadOnly
}

func (readOnly) Delete(context.Context, string) error { return ErrReadOnly }
