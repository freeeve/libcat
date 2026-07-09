package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/suggest"
	"github.com/freeeve/libcat/backend/workindex"
)

// coverMaxBytes bounds an uploaded cover; typical covers are well under 1MB.
const coverMaxBytes = 2 << 20

// coverTypes maps accepted upload content types to blob extensions.
var coverTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// coverContentType reads the declared upload type. RFC 9110 §8.3.1 makes type
// and subtype case-insensitive, so "Image/PNG" is a correct spelling of
// "image/png" and was being refused with a 415 (tasks/243).
func coverContentType(header string) (ext string, ok bool) {
	declared := strings.ToLower(strings.TrimSpace(strings.Split(header, ";")[0]))
	ext, ok = coverTypes[declared]
	return ext, ok
}

// sniffCover reports the image type of data as its magic bytes declare it,
// which is "" for anything that is not one of the three cover formats. The
// bytes decide, not the request header: a header alone let an HTML document be
// stored and served as image/png, and let a JPEG be stored at a .png path
// (tasks/243).
func sniffCover(data []byte) string {
	sniffed := http.DetectContentType(data)
	if _, ok := coverTypes[sniffed]; ok {
		return sniffed
	}
	return ""
}

// sweepStaleCovers deletes a work's cover blobs in every format except the one
// just written.
//
// Replacing a JPEG with a PNG repointed the grain and left the JPEG serving
// from its public, unauthenticated, guessable URL forever -- nothing referenced
// it, so nothing would ever collect it. A cataloger replaces a cover precisely
// when the old one is wrong: wrong edition, rights complaint, an image that
// should not have been published. A takedown that looks done was not done
// (tasks/243).
//
// Called only after the new bytes are stored, so a failed write never destroys
// the cover it was replacing.
func sweepStaleCovers(r *http.Request, bs blob.Store, workID, keep string) {
	for _, ext := range coverExts {
		if ext == keep {
			continue
		}
		_ = bs.Delete(r.Context(), bibframe.CoverBlobPath(workID, ext))
	}
}

// coverExts is every extension a cover may be stored under.
var coverExts = []string{"jpg", "png", "webp"}

// registerCovers mounts per-work cover art (tasks/215, 058 item 2): PUT
// stores the image bytes in the blob store and records the editorial
// lcat:extra/cover URL the OPAC's cover slot already reads (tasks/022/025);
// DELETE removes both. GET serves the bytes publicly -- covers are display
// assets the static site republishes anyway.
func registerCovers(mux *http.ServeMux, bs blob.Store, ix *workindex.Index, queue *suggest.Service, verifier auth.TokenVerifier) {
	librarian := auth.Require(verifier, auth.RoleLibrarian)

	mux.Handle("PUT /v1/works/{id}/cover", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		workID := r.PathValue("id")
		if !workIDPattern.MatchString(workID) {
			writeError(w, http.StatusBadRequest, "bad work id")
			return
		}
		declared := r.Header.Get("Content-Type")
		ext, ok := coverContentType(declared)
		if !ok {
			writeError(w, http.StatusUnsupportedMediaType,
				"cover must be image/jpeg, image/png, or image/webp; got "+strconv.Quote(declared))
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, coverMaxBytes))
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "cover too large (2MB cap)")
			return
		}
		if len(data) == 0 {
			writeError(w, http.StatusBadRequest, "empty body")
			return
		}
		// The bytes must be the image the header claims, or the blob's declared
		// type is a lie to the OPAC and to `lcat export -covers`.
		switch sniffed := sniffCover(data); {
		case sniffed == "":
			writeError(w, http.StatusBadRequest, "body is not a jpeg, png, or webp image")
			return
		case coverTypes[sniffed] != ext:
			writeError(w, http.StatusBadRequest, "body is "+sniffed+", not the declared "+strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0])))
			return
		}
		url := "covers/" + workID + "." + ext
		// Grain first: SetCover verifies the work exists, so a typo'd id
		// never stores orphan bytes.
		etag, err := mutateWorkGrain(r, bs, ix, workID, func(g []byte) ([]byte, error) {
			return bibframe.SetCover(g, workID, url)
		})
		if err != nil {
			writeMutateError(w, err)
			return
		}
		if _, err := bs.Put(r.Context(), bibframe.CoverBlobPath(workID, ext), data, blob.PutOptions{}); err != nil {
			writeError(w, http.StatusInternalServerError, "cover store failed")
			return
		}
		sweepStaleCovers(r, bs, workID, ext)
		if queue != nil {
			queue.WriteAudit(r.Context(), suggest.AuditEntry{
				WorkID: workID, Action: "COVER_SET", Actor: id.Email, ETag: etag, Note: url,
			})
		}
		writeJSON(w, http.StatusOK, map[string]string{"workId": workID, "cover": url, "etag": etag})
	})))

	mux.Handle("DELETE /v1/works/{id}/cover", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		workID := r.PathValue("id")
		if !workIDPattern.MatchString(workID) {
			writeError(w, http.StatusBadRequest, "bad work id")
			return
		}
		etag, err := mutateWorkGrain(r, bs, ix, workID, func(g []byte) ([]byte, error) {
			return bibframe.SetCover(g, workID, "")
		})
		if err != nil {
			writeMutateError(w, err)
			return
		}
		sweepStaleCovers(r, bs, workID, "")
		if queue != nil {
			queue.WriteAudit(r.Context(), suggest.AuditEntry{
				WorkID: workID, Action: "COVER_REMOVE", Actor: id.Email, ETag: etag,
			})
		}
		w.WriteHeader(http.StatusNoContent)
	})))

	// Public read: the admin SPA and any preview render from here; the
	// static site ships its own copies (lcat export -covers).
	mux.HandleFunc("GET /covers/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := r.PathValue("file")
		dot := strings.LastIndexByte(file, '.')
		if dot < 0 {
			writeError(w, http.StatusNotFound, "no such cover")
			return
		}
		workID, ext := file[:dot], file[dot+1:]
		ct := ""
		for typ, e := range coverTypes {
			if e == ext {
				ct = typ
			}
		}
		if ct == "" || !workIDPattern.MatchString(workID) {
			writeError(w, http.StatusNotFound, "no such cover")
			return
		}
		data, etag, err := bs.Get(r.Context(), bibframe.CoverBlobPath(workID, ext))
		if err != nil {
			writeError(w, http.StatusNotFound, "no such cover")
			return
		}
		// A same-format replacement keeps the URL, so without a validator every
		// cache between the store and the reader served the old image for up to
		// an hour after a correction. The server was right; the readers were not
		// (tasks/243).
		quoted := `"` + etag + `"`
		w.Header().Set("ETag", quoted)
		w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
		// The bytes are sniffed on upload, but a blob predating that check could
		// still be anything; nosniff retires the question.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if match := r.Header.Get("If-None-Match"); match != "" && (match == quoted || match == "*") {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", ct)
		_, _ = w.Write(data)
	})
}
