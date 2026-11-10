package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func init() {
	decoder.IgnoreUnknownKeys(true)
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{Error: message})
}

func DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func DecodeQuery(r *http.Request, v any) error {
	return decoder.Decode(v, r.URL.Query())
}

// ParseCSVParam splits a comma-separated query param (e.g.
// "?projectIds=a,b,c") into its trimmed, non-empty parts — used by the
// project-IDs-list batch endpoints (modules/labels/members/overviews) that
// fetch one resource across several projects in a single request instead
// of one request per project. Returns nil (not an empty, non-nil slice) if
// the param is absent or empty, so callers can treat "nil" as "no IDs
// given" uniformly.
func ParseCSVParam(r *http.Request, key string) []string {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
