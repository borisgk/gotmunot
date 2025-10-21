package main

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword hashes the given password using bcrypt.
func hashPassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}
	return string(hashedPassword)
}

// checkPasswordHash compares a hashed password with a plain text password.
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}