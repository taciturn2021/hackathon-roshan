package platform

import (
	"context"
	"net/http"
	"strings"
)

// Principal is the signed-in user, set on the request context by the identity middleware.
type Principal struct {
	UserID int64
	Role   string // "worker" or "customer"
}

type principalKey struct{}

// WithPrincipal returns ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the signed-in user. ok is false on routes without the auth middleware.
func PrincipalFrom(ctx context.Context) (p Principal, ok bool) {
	p, ok = ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// MustPrincipal returns the signed-in user and panics if the auth middleware did not run (programmer error).
func MustPrincipal(r *http.Request) Principal {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		panic("platform: route is missing the auth middleware")
	}
	return p
}

// BearerToken reads "Authorization: Bearer <token>". Browsers cannot set headers on a
// WebSocket handshake, so `?token=` is accepted as a fallback (used only by /api/ws).
func BearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return r.URL.Query().Get("token")
}
