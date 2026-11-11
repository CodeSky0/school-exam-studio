package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testTokenSecret = "01234567890123456789012345678901"

func testPrincipal() Principal {
	return Principal{
		UserID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SchoolID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Role:           RoleTeacher,
		SessionVersion: 3,
	}
}

func TestTokenClaimsAndExpiration(t *testing.T) {
	manager := NewTokenManager(testTokenSecret, 2*time.Hour)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	principal := testPrincipal()
	rawToken, err := manager.Issue(principal)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := manager.Parse(rawToken)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != principal {
		t.Fatalf("unexpected principal: %+v", parsed)
	}
	var claims Claims
	_, err = jwt.ParseWithClaims(rawToken, &claims, func(*jwt.Token) (any, error) {
		return []byte(testTokenSecret), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time); got != 2*time.Hour {
		t.Fatalf("token lifetime = %s, want 2h", got)
	}
	if claims.UserID != principal.UserID.String() || claims.SchoolID != principal.SchoolID.String() ||
		claims.Role != principal.Role || claims.SessionVersion != principal.SessionVersion {
		t.Fatalf("required claims missing: %+v", claims)
	}
}

func TestTokenRejectsTamperingWrongAlgorithmAndExpiration(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	manager := NewTokenManager(testTokenSecret, 2*time.Hour)
	manager.now = func() time.Time { return now }
	rawToken, err := manager.Issue(testPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	last := rawToken[len(rawToken)-1]
	replacement := byte('x')
	if last == replacement {
		replacement = 'y'
	}
	tampered := rawToken[:len(rawToken)-1] + string(replacement)
	if _, err := manager.Parse(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected tampered token rejection, got %v", err)
	}

	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id": testPrincipal().UserID.String(),
		"exp":     now.Add(time.Hour).Unix(),
	})
	noneToken, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(noneToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected alg=none rejection, got %v", err)
	}

	manager.now = func() time.Time { return now.Add(2*time.Hour + time.Second) }
	if _, err := manager.Parse(rawToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
}
