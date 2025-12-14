package auth

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "my-super-secret-password"

	// Test hashing
	hash := HashPassword(password)
	if hash == "" {
		t.Fatal("HashPassword returned an empty string")
	}

	// Test valid password check
	if !CheckPasswordHash(password, hash) {
		t.Errorf("CheckPasswordHash should have returned true for a correct password")
	}

	// Test invalid password check
	if CheckPasswordHash("wrong-password", hash) {
		t.Errorf("CheckPasswordHash should have returned false for an incorrect password")
	}
}
