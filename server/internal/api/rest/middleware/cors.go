package middleware

import "net/http"

// CORSMiddleware adds permissive CORS headers to a route group. Applied
// narrowly — only around the public (no-auth) routes — never globally,
// since the rest of the API relies on the app-secret HMAC gate
// (AppTokenMiddleware) rather than CORS for its own protection.
type CORSMiddleware struct{}

func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{}
}

func (m *CORSMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We rely on the reverse proxy (Nginx/Cloudflare) to set Access-Control-Allow-Origin.
		// Setting it here causes duplicate headers when the proxy also sets it.
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		// X-Conn-Id: the public feedback client reuses the main Dio
		// instance's interceptors (including AuthInterceptor, which sets
		// this on every request regardless of auth state), so it shows up
		// here too even though these routes don't require it.
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Conn-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
