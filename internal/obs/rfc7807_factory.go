package obs

import (
	"encoding/json"
	"net/http"
)

// RFC7807 represents a Problem Details payload (RFC 7807).
type RFC7807 struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
}

// WriteProblem emits a Problem Details response with the given status, type, and title.
func WriteProblem(w http.ResponseWriter, status int, typ, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(RFC7807{
		Type:   typ,
		Title:  title,
		Status: status,
	})
}
