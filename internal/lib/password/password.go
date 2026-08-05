package password

import (
	"fmt"

	"github.com/alexedwards/argon2id"
)

var hashParams = argon2id.DefaultParams

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password can not be empty")
	}
	hashed, err := argon2id.CreateHash(password, hashParams)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %v", err)
	}
	return hashed, nil
}

func CheckPassword(hashedPassword string, password string) error {
	match, err := argon2id.ComparePasswordAndHash(password, hashedPassword)
	if err != nil {
		return fmt.Errorf("failed to compare password: %v", err)
	}
	if !match {
		return fmt.Errorf("password does not match")
	}
	return nil
}
