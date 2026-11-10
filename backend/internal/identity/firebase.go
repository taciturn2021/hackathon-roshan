package identity

import (
	"context"
	"fmt"
	"log"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type firebaseVerifier struct{ client *auth.Client }

// NewFirebaseVerifier verifies ID tokens for projectID. Verification only needs Google's public
// keys, so credsFile (a service-account JSON) is optional; without it the SDK runs unauthenticated.
func NewFirebaseVerifier(ctx context.Context, projectID, credsFile string) (Verifier, error) {
	if projectID == "" {
		return nil, fmt.Errorf("FIREBASE_PROJECT_ID is not set")
	}
	opt := option.WithoutAuthentication()
	if credsFile != "" {
		opt = option.WithCredentialsFile(credsFile)
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, opt)
	if err != nil {
		return nil, fmt.Errorf("firebase app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase auth: %w", err)
	}
	return firebaseVerifier{client}, nil
}

func (f firebaseVerifier) Verify(ctx context.Context, idToken string) (Token, error) {
	t, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Token{}, err
	}
	claim := func(k string) string { s, _ := t.Claims[k].(string); return s }
	return Token{UID: t.UID, Email: claim("email"), Phone: claim("phone_number"), Name: claim("name"), Picture: claim("picture")}, nil
}

// DevVerifier accepts tokens of the form "dev:<uid>" (for curl, seed checks and the race test)
// and passes everything else to next. Enabled only when AUTH_DEV=1; never set it in production.
// ponytail: dev tokens let anyone act as any user; acceptable only on a local or test stack.
func DevVerifier(next Verifier) Verifier {
	log.Print("WARNING: AUTH_DEV=1, accepting dev:<uid> tokens")
	return devVerifier{next}
}

type devVerifier struct{ next Verifier }

func (d devVerifier) Verify(ctx context.Context, idToken string) (Token, error) {
	if uid, ok := strings.CutPrefix(idToken, "dev:"); ok && uid != "" {
		return Token{UID: uid, Name: uid}, nil
	}
	if d.next == nil {
		return Token{}, fmt.Errorf("not a dev token")
	}
	return d.next.Verify(ctx, idToken)
}
