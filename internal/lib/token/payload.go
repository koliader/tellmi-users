package token

import (
	"time"

	"github.com/golang-jwt/jwt"
)

type Payload struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiredAt time.Time `json:"expired_at"`
}

func NewPayload(id int64, role string, duration time.Duration) (*Payload, error) {
	payload := &Payload{
		ID:        id,
		Role:      role,
		IssuedAt:  time.Now(),
		ExpiredAt: time.Now().Add(duration),
	}
	return payload, nil
}

func (p *Payload) Valid() error {
	// Check if the token is expired
	if time.Now().After(p.ExpiredAt) {
		return jwt.NewValidationError("token is expired", jwt.ValidationErrorExpired)
	}
	return nil
}
