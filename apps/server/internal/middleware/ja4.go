package middleware

import (
	"log"
	"net/http"
)

// JA4Middleware is a placeholder for JA4 fingerprinting.
// Real JA4 implementation requires low-level access to ClientHello (e.g. using `utls`).
// This middleware currently just logs that it's running.
func JA4Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement JA4 Fingerprinting
		// 1. Extract TLS ClientHello
		// 2. Compute JA4 Signature
		// 3. Check against Whitelist/Blacklist

		// For now, we pass everything
		log.Printf("Process request: %s %s", r.Method, r.URL.Path)

		next.ServeHTTP(w, r)
	})
}
