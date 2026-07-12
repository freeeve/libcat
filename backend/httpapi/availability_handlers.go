package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/freeeve/libcodex/sip2"
)

// sip2AvailabilityCap bounds one bridge request's item fan-out: a work page
// asks for a handful of barcodes, and the SIP2 session behind this is
// serial per connection.
const sip2AvailabilityCap = 50

// sip2Item is one item's live status, normalized for the OPAC adapter: the
// SIP2 circulation-status table folded to the availability model's rollup,
// with the raw status label kept for display honesty.
type sip2Item struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // available | loaned | unavailable | unknown
	StatusText string `json:"statusText,omitempty"`
	Title      string `json:"title,omitempty"`
	DueDate    string `json:"dueDate,omitempty"`
	Location   string `json:"location,omitempty"`
	CallNumber string `json:"callNumber,omitempty"`
	HoldQueue  string `json:"holdQueue,omitempty"`
}

// sip2Rollup folds the SIP2 circulation-status codes (01-13) onto the
// normalized availability statuses. 09 (waiting to be re-shelved) counts
// available: the copy is in the building and findable shortly.
var sip2Rollup = map[string]string{
	"03": "available", "09": "available",
	"04": "loaned", "05": "loaned", "07": "loaned",
	"02": "unavailable", "06": "unavailable", "08": "unavailable",
	"10": "unavailable", "11": "unavailable", "12": "unavailable", "13": "unavailable",
	"01": "unknown",
}

// registerAvailability mounts the SIP2 availability bridge: the OPAC's
// proxied transport for a protocol no browser can speak. Public and
// CORS-open -- shelf status is what the catalog exists to publish -- with
// the ILS credentials held server-side. One SIP2 session per request; a
// dead ILS degrades items to "unknown" client-side via the error status.
func registerAvailability(mux *http.ServeMux, client *sip2.Client, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	cors := func(w http.ResponseWriter) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}

	mux.HandleFunc("OPTIONS /v1/availability/sip2", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /v1/availability/sip2", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad request body")
			return
		}
		if len(req.IDs) == 0 || len(req.IDs) > sip2AvailabilityCap {
			writeError(w, http.StatusBadRequest, "1-50 ids per request")
			return
		}
		conn, err := client.Connect(r.Context())
		if err != nil {
			logger.Error("sip2 connect failed", "addr", client.Address, "err", err)
			writeError(w, http.StatusBadGateway, "the ILS is not answering")
			return
		}
		defer conn.Close()
		items := make(map[string]sip2Item, len(req.IDs))
		for _, id := range req.IDs {
			if id == "" {
				continue
			}
			info, err := conn.ItemInformation(r.Context(), id)
			if err != nil {
				// Mid-session failure: report what landed, mark the rest
				// unknown rather than failing the page's whole strip.
				logger.Warn("sip2 item information failed", "item", id, "err", err)
				items[id] = sip2Item{ID: id, Status: "unknown"}
				continue
			}
			status, ok := sip2Rollup[info.CirculationStatus]
			if !ok {
				status = "unknown"
			}
			items[id] = sip2Item{
				ID:         id,
				Status:     status,
				StatusText: info.StatusLabel,
				Title:      info.Title,
				DueDate:    info.DueDate,
				Location:   firstNonEmpty(info.CurrentLocation, info.PermanentLocation),
				CallNumber: info.CallNumber,
				HoldQueue:  info.HoldQueueLength,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
