package main

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "my-super-secret-password"

	// Test hashing
	hash := hashPassword(password)
	if hash == "" {
		t.Fatal("hashPassword returned an empty string")
	}

	// Test valid password check
	if !checkPasswordHash(password, hash) {
		t.Errorf("checkPasswordHash should have returned true for a correct password")
	}

	// Test invalid password check
	if checkPasswordHash("wrong-password", hash) {
		t.Errorf("checkPasswordHash should have returned false for an incorrect password")
	}
}