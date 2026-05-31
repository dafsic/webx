package main

import (
	"net/http"
	"strings"
)

// corsAllowedHeaders are the request headers a browser may send cross-origin.
const corsAllowedHeaders = "Accept, Authorization, Content-Type, Origin, X-Requested-With"

// corsAllowedMethods are the methods exposed cross-origin.
const corsAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

// corsMiddleware applies CORS headers based on an allow-list of origins and
// short-circuits preflight (OPTIONS) requests. The allow-list pattern is
// borrowed from the reference api gateway; "*" disables the allow-list and
// echoes any origin (without credentials, per the CORS spec).
func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowAny := len(allowedOrigins) == 0 || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			switch {
			case allowAny:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case originAllowed(allowedOrigins, origin):
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether origin is in the allow-list (case-insensitive,
// exact match).
func originAllowed(allowedOrigins []string, origin string) bool {
	for _, a := range allowedOrigins {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
