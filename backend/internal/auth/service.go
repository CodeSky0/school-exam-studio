package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
)

const maxSchoolCodeAttempts = 5

type Service struct {
	repository *Repository
	hasher     *PasswordHasher
	tokens     *TokenManager
	dummyHash  string
}

func NewService(repository *Repository, hasher *PasswordHasher, jwtSecret string, tokenTTL time.Duration) *Service {
	dummyHash, _ := hasher.Hash("dummy-password-for-timing")
	return &Service{
		repository: repository,
		hasher:     hasher,
		tokens:     NewTokenManager(jwtSecret, tokenTTL),
		dummyHash:  dummyHash,
	}
}

func (s *Service) Tokens() *TokenManager {
	return s.tokens
}

func (s *Service) InitializationStatus(ctx context.Context) (bool, error) {
	return s.repository.Initialized(ctx)
}

func (s *Service) Initialize(ctx context.Context, input InitializeInput) (InitializeResult, error) {
	normalized, err := normalizeInitializeInput(input)
	if err != nil {
		return InitializeResult{}, err
	}
	passwordHash, err := s.hasher.Hash(normalized.Password)
	if err != nil {
		return InitializeResult{}, err
	}
	for range maxSchoolCodeAttempts {
		schoolCode, err := generateSchoolCode()
		if err != nil {
			return InitializeResult{}, err
		}
		result, err := s.repository.Initialize(ctx, normalized, passwordHash, schoolCode)
		if errors.Is(err, errSchoolCodeConflict) {
			continue
		}
		return result, err
	}
	return InitializeResult{}, errors.New("could not generate a unique school code")
}

func generateSchoolCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", fmt.Errorf("generate school code: %w", err)
	}
	return fmt.Sprintf("%08d", value.Int64()), nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	normalized, err := normalizeRegisterInput(input)
	if err != nil {
		return User{}, err
	}
	passwordHash, err := s.hasher.Hash(normalized.Password)
	if err != nil {
		return User{}, err
	}
	return s.repository.CreateTeacher(ctx, normalized, passwordHash)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email, emailErr := normalizeEmail(input.Email)
	if emailErr != nil || validatePassword(input.Password, "password") != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	schoolEntity, err := s.repository.SingleSchool(ctx)
	if ent.IsNotFound(err) {
		s.hasher.Verify(s.dummyHash, input.Password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	entity, err := s.repository.UserByEmail(ctx, schoolEntity.ID, email)
	if ent.IsNotFound(err) {
		s.hasher.Verify(s.dummyHash, input.Password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if !s.hasher.Verify(entity.PasswordHash, input.Password) || entity.Status != StatusActive {
		return LoginResult{}, ErrInvalidCredentials
	}
	principal := Principal{
		UserID:         entity.ID,
		SchoolID:       entity.SchoolID,
		Role:           entity.Role,
		Subject:        entity.Subject,
		SessionVersion: entity.SessionVersion,
	}
	token, err := s.tokens.Issue(principal)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: userFromEnt(entity)}, nil
}

func (s *Service) Me(ctx context.Context, principal Principal) (User, error) {
	entity, err := s.repository.UserByID(ctx, principal.SchoolID, principal.UserID)
	if ent.IsNotFound(err) {
		return User{}, ErrSessionInvalid
	}
	if err != nil {
		return User{}, err
	}
	if entity.Status != StatusActive {
		return User{}, ErrSessionInvalid
	}
	return userFromEnt(entity), nil
}

func (s *Service) Authenticate(ctx context.Context, claimed Principal) (Principal, error) {
	if claimed.UserID == uuid.Nil || claimed.SchoolID == uuid.Nil ||
		!validRole(claimed.Role) || claimed.SessionVersion < 1 {
		return Principal{}, ErrSessionInvalid
	}
	entity, err := s.repository.UserByID(ctx, claimed.SchoolID, claimed.UserID)
	if ent.IsNotFound(err) {
		return Principal{}, ErrSessionInvalid
	}
	if err != nil {
		return Principal{}, err
	}
	if entity.Status != StatusActive || entity.Role != claimed.Role ||
		entity.SessionVersion != claimed.SessionVersion {
		return Principal{}, ErrSessionInvalid
	}
	return Principal{
		UserID:         entity.ID,
		SchoolID:       entity.SchoolID,
		Role:           entity.Role,
		Subject:        entity.Subject,
		SessionVersion: entity.SessionVersion,
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, principal Principal, input ChangePasswordInput) error {
	if err := validatePassword(input.NewPassword, "new_password"); err != nil {
		return err
	}
	entity, err := s.repository.UserByID(ctx, principal.SchoolID, principal.UserID)
	if ent.IsNotFound(err) {
		return ErrSessionInvalid
	}
	if err != nil {
		return err
	}
	if entity.Status != StatusActive || entity.Role != principal.Role ||
		entity.SessionVersion != principal.SessionVersion {
		return ErrSessionInvalid
	}
	if !s.hasher.Verify(entity.PasswordHash, input.OldPassword) {
		return ErrOldPassword
	}
	passwordHash, err := s.hasher.Hash(input.NewPassword)
	if err != nil {
		return err
	}
	return s.repository.UpdatePassword(ctx, principal, passwordHash)
}
