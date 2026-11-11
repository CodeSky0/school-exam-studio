package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHasher(t *testing.T) {
	hasher := NewPasswordHasher(bcrypt.MinCost)
	const password = "CorrectHorse123!"
	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, password) {
		t.Fatal("hash contains plaintext password")
	}
	if !hasher.Verify(hash, password) {
		t.Fatal("correct password was rejected")
	}
	if hasher.Verify(hash, "WrongHorse123!") {
		t.Fatal("wrong password was accepted")
	}
}

func TestPasswordHasherRejectsMoreThan72Bytes(t *testing.T) {
	hasher := NewPasswordHasher(bcrypt.MinCost)
	if _, err := hasher.Hash(strings.Repeat("a", 73)); err == nil {
		t.Fatal("expected bcrypt password length error")
	}
}
