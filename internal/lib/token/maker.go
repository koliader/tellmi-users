package token

import "time"

// Maker is an interface for managing tokens
type Maker interface {
	CreateToken(username string, role string, duration time.Duration) (string, error)
	VerifyToken(tokenString string) (*Payload, error)
}
