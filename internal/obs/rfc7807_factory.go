package obs

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RFC7807 represents a Problem Details payload (RFC 7807).
type RFC7807 struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

// WriteProblem emits a Problem Details response with the given status, type, and title.
func WriteProblem(w http.ResponseWriter, status int, typ, title string) {
	WriteProblemDetailed(w, status, typ, title, title, "")
}

// WriteProblemDetailed emits a Problem Details response with optional detail and instance context.
func WriteProblemDetailed(w http.ResponseWriter, status int, typ, title, detail, instance string) {
	if strings.TrimSpace(detail) == "" {
		detail = title
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(RFC7807{
		Type:     typ,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	})
}

// WriteProblemForRequest emits a Problem response with request-path instance metadata.
func WriteProblemForRequest(w http.ResponseWriter, r *http.Request, status int, typ, title, detail string) {
	instance := ""
	if r != nil && r.URL != nil {
		instance = r.URL.Path
	}
	WriteProblemDetailed(w, status, typ, title, detail, instance)
}
