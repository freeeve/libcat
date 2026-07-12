// POST /v1/availability/sip2 -- the OPAC's proxied transport for live shelf
// status: item barcodes in, the normalized rollup out, credentials and the
// TCP protocol held server-side. CORS-open (shelf status is public), 502
// when the ILS is down, per-item degradation mid-session.
package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/freeeve/libcodex/sip2"
)

// fakeACS answers a SIP2 session: login accepted, then Item Information
// responses from a canned table keyed by barcode.
func fakeACS(t *testing.T, items map[string]string) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	return func(ctx context.Context, addr string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			r := bufio.NewReader(server)
			for {
				line, err := r.ReadString('\r')
				if err != nil {
					return
				}
				line = strings.TrimRight(line, "\r")
				switch {
				case strings.HasPrefix(line, "93"):
					server.Write([]byte("941AY0AZFDFB\r"))
				case strings.HasPrefix(line, "17"):
					barcode := ""
					for _, f := range strings.Split(line, "|") {
						if strings.HasPrefix(f, "AB") {
							barcode = f[2:]
						}
					}
					resp, ok := items[barcode]
					if !ok {
						resp = "1801xx0000020260712    010101AB" + barcode + "|AJ|"
					}
					server.Write([]byte(resp + "\r"))
				default:
					return
				}
			}
		}()
		return client, nil
	}
}

func availabilityAPI(t *testing.T, dial func(context.Context, string) (net.Conn, error)) http.Handler {
	t.Helper()
	client := &sip2.Client{Address: "acs.test:6001", User: "term", Password: "pw", Dial: dial}
	mux := http.NewServeMux()
	registerAvailability(mux, client, nil)
	return mux
}

func TestAvailabilitySIP2Bridge(t *testing.T) {
	const date = "20260712    010101"
	h := availabilityAPI(t, fakeACS(t, map[string]string{
		"b-on-shelf": "1803xx0000" + date + "ABb-on-shelf|AJGiovanni's Room|APMain Stacks|CSPS3552.A45|",
		"b-loaned":   "1804xx0000" + date + "ABb-loaned|AJStone Butch Blues|AH20260801|CF2|",
	}))

	rec := request(t, h, http.MethodPost, "/v1/availability/sip2", "", "", map[string]any{
		"ids": []string{"b-on-shelf", "b-loaned"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("bridge = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q, want * (a static OPAC origin must reach this)", got)
	}
	var page struct {
		Items map[string]sip2Item `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	onShelf := page.Items["b-on-shelf"]
	if onShelf.Status != "available" || onShelf.Location != "Main Stacks" || onShelf.CallNumber != "PS3552.A45" {
		t.Fatalf("on-shelf item = %+v", onShelf)
	}
	loaned := page.Items["b-loaned"]
	if loaned.Status != "loaned" || loaned.DueDate != "20260801" || loaned.HoldQueue != "2" {
		t.Fatalf("loaned item = %+v", loaned)
	}

	// Validation floor.
	if rec := request(t, h, http.MethodPost, "/v1/availability/sip2", "", "", map[string]any{"ids": []string{}}); rec.Code != http.StatusBadRequest {
		t.Errorf("empty ids = %d, want 400", rec.Code)
	}
	big := make([]string, 51)
	for i := range big {
		big[i] = "b"
	}
	if rec := request(t, h, http.MethodPost, "/v1/availability/sip2", "", "", map[string]any{"ids": big}); rec.Code != http.StatusBadRequest {
		t.Errorf("51 ids = %d, want 400", rec.Code)
	}
}

func TestAvailabilitySIP2ILSDown(t *testing.T) {
	h := availabilityAPI(t, func(ctx context.Context, addr string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	})
	rec := request(t, h, http.MethodPost, "/v1/availability/sip2", "", "", map[string]any{"ids": []string{"b1"}})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("down ILS = %d (%s), want 502", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "refused") {
		t.Fatalf("body leaks dial detail: %s", rec.Body)
	}
}
