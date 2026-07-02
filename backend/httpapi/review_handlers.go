package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/freeeve/libcatalog/backend/auth"
	"github.com/freeeve/libcatalog/backend/suggest"
	"github.com/freeeve/libcatalog/backend/vocab"
)

var monthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

// registerReview mounts the staff moderation surface: the queue, batch
// review, manual/folk term governance, and the audit trail.
func registerReview(mux *http.ServeMux, svc *suggest.Service, verifier auth.TokenVerifier) {
	moderator := auth.Require(verifier, auth.RoleModerator)
	librarian := auth.Require(verifier, auth.RoleLibrarian)

	mux.Handle("GET /v1/queue", moderator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := suggest.QueueQuery{
			Status:     suggest.Status(r.URL.Query().Get("status")),
			Scheme:     r.URL.Query().Get("scheme"),
			Provenance: suggest.Provenance(r.URL.Query().Get("provenance")),
			Type:       suggest.SuggType(r.URL.Query().Get("type")),
			Cursor:     r.URL.Query().Get("cursor"),
		}
		page, err := svc.Queue(r.Context(), q)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "queue read failed")
			return
		}
		writeJSON(w, http.StatusOK, page)
	})))

	mux.Handle("POST /v1/review", moderator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		var req struct {
			Decisions []suggest.Decision `json:"decisions"`
			Publish   bool               `json:"publish"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request body")
			return
		}
		if len(req.Decisions) == 0 || len(req.Decisions) > 100 {
			writeError(w, http.StatusBadRequest, "1-100 decisions per batch")
			return
		}
		if req.Publish && !id.CanPublish() {
			writeError(w, http.StatusForbidden, "publishing requires the librarian role")
			return
		}
		if err := svc.Review(r.Context(), req.Decisions, id.Email); err != nil {
			if errors.Is(err, suggest.ErrBadTerm) {
				writeError(w, http.StatusBadRequest, "unknown substitute term")
				return
			}
			writeError(w, http.StatusInternalServerError, "review failed")
			return
		}
		resp := map[string]any{"reviewed": len(req.Decisions), "published": false}
		if req.Publish {
			// The publish pipeline lands with tasks/036; approvals are
			// durable in the queue until then.
			pending, err := svc.ApprovedUnpublished(r.Context())
			if err == nil {
				resp["approvedPending"] = len(pending)
			}
			resp["publishNote"] = "publisher not yet configured; approvals queued"
		}
		writeJSON(w, http.StatusOK, resp)
	})))

	mux.Handle("POST /v1/terms", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		var req struct {
			Action    string         `json:"action"` // manual | acceptFolk | blockFolk
			WorkID    string         `json:"workId,omitempty"`
			Term      *vocab.TermRef `json:"term,omitempty"`
			FolkTerm  string         `json:"folkTerm,omitempty"`
			WorkTitle string         `json:"workTitle,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request body")
			return
		}
		switch req.Action {
		case "manual":
			if req.Term == nil || !workIDPattern.MatchString(req.WorkID) {
				writeError(w, http.StatusBadRequest, "manual requires workId and term")
				return
			}
			err := svc.ManualTerm(r.Context(), req.WorkID, *req.Term, req.WorkTitle, id.Email)
			switch {
			case errors.Is(err, suggest.ErrBadTerm):
				writeError(w, http.StatusBadRequest, "unknown term")
			case err != nil:
				writeError(w, http.StatusConflict, err.Error())
			default:
				w.WriteHeader(http.StatusCreated)
			}
		case "acceptFolk", "blockFolk":
			norm, err := vocab.NormalizeFolk(req.FolkTerm)
			if err != nil {
				writeError(w, http.StatusBadRequest, "unusable folk term")
				return
			}
			status := suggest.FolkAccepted
			if req.Action == "blockFolk" {
				status = suggest.FolkBlocked
			}
			if err := svc.SetFolkStatus(r.Context(), norm, status, id.Email); err != nil {
				writeError(w, http.StatusNotFound, "unknown folk term")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusBadRequest, "unknown action")
		}
	})))

	mux.Handle("GET /v1/audit", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		month := r.URL.Query().Get("month")
		if !monthPattern.MatchString(month) {
			writeError(w, http.StatusBadRequest, "month must be YYYY-MM")
			return
		}
		entries, err := svc.Audit(r.Context(), month)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "audit read failed")
			return
		}
		if entries == nil {
			entries = []suggest.AuditEntry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"month": month, "entries": entries})
	})))
}
