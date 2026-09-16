package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// maxJSONBytes bounds a JSON request body. The multipart routes apply their own,
// far larger caps; a global limit could not serve both.
const maxJSONBytes = 1 << 20

// errBadJSON marks a body the client got wrong.
var errBadJSON = errors.New("malformed JSON body")

// decodeJSON reads a JSON body strictly: unknown fields are rejected, so a
// typo'd field name fails loudly instead of being silently ignored, and
// trailing content is refused.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errBadJSON
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errBadJSON
	}
	return nil
}

// pathInt64 reads a numeric path parameter.
func pathInt64(r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}

// queryBool reads an optional boolean query parameter. A missing parameter is
// nil — "no filter" — which is different from an explicit false.
func queryBool(r *http.Request, name string) (*bool, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, true
	}
	switch strings.ToLower(raw) {
	case "true", "1":
		v := true
		return &v, true
	case "false", "0":
		v := false
		return &v, true
	}
	return nil, false
}

// isJSONRequest reports whether the body is JSON rather than multipart.
func isJSONRequest(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return ct == "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "application/json")
}
