package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/auth"
)

// auditSnapshotPrefix is the blob tree dated audit snapshots live under, one
// directory per scope key.
const auditSnapshotPrefix = "data/audit/"

// auditSnapshot is one recorded audit: the full response plus the day it was
// taken. Recording is an explicit act (a librarian presses the button at a
// meaningful moment -- post-weeding, post-acquisition), not a side effect of
// deploys, and one snapshot per scope per day keeps re-presses idempotent.
type auditSnapshot struct {
	Date string `json:"date"`
	auditResponse
}

// scopeKey names a filter set's snapshot directory: "all" unfiltered, else a
// short digest of the normalized filter terms (filesystem-safe whatever the
// extras keys hold).
func scopeKey(filters auditFilterSet) string {
	if len(filters) == 0 {
		return "all"
	}
	sum := sha256.Sum256([]byte(filters.cacheKey()))
	return hex.EncodeToString(sum[:6])
}

// snapshotPath is one snapshot's blob key.
func snapshotPath(key, date string) string {
	return auditSnapshotPrefix + key + "/" + date + ".json"
}

// registerAuditSnapshots mounts the audit-history surface (the 384/398 trend
// backbone): POST records today's report for the given scope, GET lists the
// dated series the trend charts draw.
func registerAuditSnapshots(mux *http.ServeMux, bs blob.Store, verifier auth.TokenVerifier, compute func(r *http.Request) (auditResponse, auditFilterSet, int, error)) {
	librarian := auth.Require(verifier, auth.RoleLibrarian)

	mux.Handle("POST /v1/audit/diversity/snapshots", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, filters, status, err := compute(r)
		if err != nil {
			writeError(w, status, err.Error())
			return
		}
		snap := auditSnapshot{Date: time.Now().UTC().Format("2006-01-02"), auditResponse: resp}
		data, err := json.Marshal(snap)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encode snapshot")
			return
		}
		path := snapshotPath(scopeKey(filters), snap.Date)
		if _, err := bs.Put(r.Context(), path, data, blob.PutOptions{ContentType: "application/json"}); err != nil {
			writeGrainWriteError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, snap)
	})))

	mux.Handle("GET /v1/audit/diversity/snapshots", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters, err := auditFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		prefix := auditSnapshotPrefix + scopeKey(filters) + "/"
		var snaps []auditSnapshot
		for entry, err := range bs.List(r.Context(), prefix) {
			if err != nil {
				writeError(w, http.StatusInternalServerError, "list snapshots")
				return
			}
			if !strings.HasSuffix(entry.Path, ".json") {
				continue
			}
			data, _, err := bs.Get(r.Context(), entry.Path)
			if err != nil {
				continue
			}
			var snap auditSnapshot
			if json.Unmarshal(data, &snap) != nil || snap.Date == "" {
				continue // an unreadable snapshot never poisons the series
			}
			snaps = append(snaps, snap)
		}
		sort.Slice(snaps, func(i, j int) bool { return snaps[i].Date < snaps[j].Date })
		writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
	})))
}
