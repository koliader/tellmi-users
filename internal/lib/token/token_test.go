package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testSecretKey = "abcdefghijklmnopqrstuvwxyz123456"

func TestNewJWTMaker(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)
	require.NotNil(t, maker)
}

func TestNewJWTMakerShortKey(t *testing.T) {
	_, err := NewJWTMaker("short")
	require.Error(t, err)
}

func TestCreateAndVerifyToken(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	username := "testuser"
	role := "USER"
	duration := time.Minute

	tokenStr, err := maker.CreateToken(username, role, duration)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	payload, err := maker.VerifyToken(tokenStr)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	require.Equal(t, username, payload.Username)
	require.Equal(t, role, payload.Role)
	require.WithinDuration(t, time.Now(), payload.IssuedAt, time.Second)
	require.WithinDuration(t, time.Now().Add(duration), payload.ExpiredAt, time.Second)
}

func TestVerifyExpiredToken(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	tokenStr, err := maker.CreateToken("user", "USER", -time.Minute)
	require.NoError(t, err)

	_, err = maker.VerifyToken(tokenStr)
	require.Error(t, err)
}

func TestVerifyInvalidToken(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	_, err = maker.VerifyToken("completely.invalid.token")
	require.Error(t, err)
}

func TestVerifyTokenWithDifferentSecret(t *testing.T) {
	key1 := "abcdefghijklmnopqrstuvwxyzextra1"
	key2 := "abcdefghijklmnopqrstuvwxyzextra2"

	maker1, err := NewJWTMaker(key1)
	require.NoError(t, err)

	maker2, err := NewJWTMaker(key2)
	require.NoError(t, err)

	tokenStr, err := maker1.CreateToken("user", "USER", time.Minute)
	require.NoError(t, err)

	_, err = maker2.VerifyToken(tokenStr)
	require.Error(t, err)
}

func TestNewPayload(t *testing.T) {
	username := "alice"
	role := "ADMIN"
	duration := time.Hour

	before := time.Now()
	payload, err := NewPayload(username, role, duration)
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, username, payload.Username)
	require.Equal(t, role, payload.Role)
	require.False(t, payload.IssuedAt.Before(before))
	require.False(t, payload.IssuedAt.After(after))
	require.WithinDuration(t, payload.IssuedAt.Add(duration), payload.ExpiredAt, time.Second)
}

func TestPayloadValid(t *testing.T) {
	payload, err := NewPayload("user", "USER", time.Hour)
	require.NoError(t, err)
	require.NoError(t, payload.Valid())
}

func TestPayloadValidExpired(t *testing.T) {
	payload, err := NewPayload("user", "USER", -time.Hour)
	require.NoError(t, err)
	require.Error(t, payload.Valid())
}

func TestCreateTokenEmptyUsername(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	tokenStr, err := maker.CreateToken("", "USER", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	payload, err := maker.VerifyToken(tokenStr)
	require.NoError(t, err)
	require.Equal(t, "", payload.Username)
}

func TestDifferentRoles(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	roles := []string{"USER", "ADMIN", "MODERATOR"}
	for _, role := range roles {
		tokenStr, err := maker.CreateToken("user", role, time.Minute)
		require.NoError(t, err)

		payload, err := maker.VerifyToken(tokenStr)
		require.NoError(t, err)
		require.Equal(t, role, payload.Role)
	}
}

func TestTokenRoundTripLongDuration(t *testing.T) {
	maker, err := NewJWTMaker(testSecretKey)
	require.NoError(t, err)

	tokenStr, err := maker.CreateToken("user", "USER", 365*24*time.Hour)
	require.NoError(t, err)

	payload, err := maker.VerifyToken(tokenStr)
	require.NoError(t, err)
	require.False(t, payload.Valid() != nil, "token should be valid for 1 year")
}
