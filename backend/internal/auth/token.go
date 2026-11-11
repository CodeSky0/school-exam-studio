package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const tokenIssuer = "school-exam-studio"

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID         string `json:"user_id"`
	SchoolID       string `json:"school_id"`
	Role           string `json:"role"`
	SessionVersion int    `json:"session_version"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}
}

func (m *TokenManager) Issue(principal Principal) (string, error) {
	if len(m.secret) < 32 {
		return "", errors.New("JWT secret must contain at least 32 bytes")
	}
	if principal.UserID == uuid.Nil || principal.SchoolID == uuid.Nil ||
		!validRole(principal.Role) || principal.SessionVersion < 1 {
		return "", errors.New("invalid token principal")
	}
	now := m.now().UTC()
	claims := Claims{
		UserID:         principal.UserID.String(),
		SchoolID:       principal.SchoolID.String(),
		Role:           principal.Role,
		SessionVersion: principal.SessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   principal.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) Parse(rawToken string) (Principal, error) {
	if len(m.secret) < 32 || rawToken == "" {
		return Principal{}, ErrInvalidToken
	}
	var claims Claims
	token, err := jwt.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || !token.Valid {
		return Principal{}, ErrInvalidToken
	}
	principal, err := principalFromClaims(&claims)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	return principal, nil
}

func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}

func principalFromClaims(claims *Claims) (Principal, error) {
	userID, err := uuid.Parse(claims.UserID)
	if err != nil || userID == uuid.Nil {
		return Principal{}, ErrInvalidToken
	}
	schoolID, err := uuid.Parse(claims.SchoolID)
	if err != nil || schoolID == uuid.Nil {
		return Principal{}, ErrInvalidToken
	}
	if claims.Subject != claims.UserID || !validRole(claims.Role) || claims.SessionVersion < 1 {
		return Principal{}, ErrInvalidToken
	}
	return Principal{
		UserID:         userID,
		SchoolID:       schoolID,
		Role:           claims.Role,
		SessionVersion: claims.SessionVersion,
	}, nil
}
