// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package handler

import "net/http"

// SecurityHeaders returns middleware that sets security headers on every response.
// When tlsMode is "tls" or "mtls", HSTS is also added.
// Handlers can override Cache-Control after this middleware runs (last-writer-wins).
func SecurityHeaders(tlsMode string) func(http.Handler) http.Handler {
	hsts := tlsMode == "tls" || tlsMode == "mtls"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Frame-Options", "DENY")
			if hsts {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
