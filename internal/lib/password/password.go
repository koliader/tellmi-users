package password

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/alexedwards/argon2id"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// hashParams is tuned for latency under concurrent logins: 16 MiB memory at a
// single iteration. Parallelism is clamped so lanes never oversubscribe a
// throttled container quota (GOMAXPROCS reflects the node, not the cgroup
// limit).
// Note: verification always uses the params embedded in the stored hash, so
// existing hashes keep their original cost until they are re-hashed on the
// next successful login (see NeedsRehash).
var hashParams = &argon2id.Params{
	Memory:      16 * 1024,
	Iterations:  1,
	Parallelism: uint8(min(runtime.GOMAXPROCS(0), 4)),
	SaltLength:  16,
	KeyLength:   32,
}

// maxHashConcurrency bounds how many argon2 operations run at once. Every
// hash transiently allocates Memory (32 MiB, or 64 MiB for pre-tuning hashes),
// so without a cap a CPU-throttled pod can queue enough concurrent hashes to
// blow past its memory limit and get OOMKilled.
const maxHashConcurrency = 4

var hashSemaphore = make(chan struct{}, maxHashConcurrency)

var tracer = otel.Tracer("tellmi-users/password")

func acquireHashSlot() func() {
	hashSemaphore <- struct{}{}
	return func() { <-hashSemaphore }
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// decodeParams pulls m/t/p out of an argon2id PHC string
// ($argon2id$v=19$m=32768,t=1,p=4$...$...).
func decodeParams(hashed string) (memory, iterations, parallelism uint32) {
	parts := strings.Split(hashed, "$")
	if len(parts) < 4 {
		return 0, 0, 0
	}
	for _, kv := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			continue
		}
		v, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			continue
		}
		switch pair[0] {
		case "m":
			memory = uint32(v)
		case "t":
			iterations = uint32(v)
		case "p":
			parallelism = uint32(v)
		}
	}
	return memory, iterations, parallelism
}

// NeedsRehash reports whether a stored hash uses costlier params than the
// current hashParams (e.g. a legacy 32 MiB hash). Such hashes are re-encrypted
// with the current params on their next successful login.
func NeedsRehash(hashed string) bool {
	m, t, _ := decodeParams(hashed)
	return m > uint32(hashParams.Memory) || t > uint32(hashParams.Iterations)
}

func HashPassword(ctx context.Context, password string) (string, error) {
	ctx, span := tracer.Start(ctx, "password.hash")
	defer span.End()
	span.SetAttributes(
		attribute.Int("argon2.memory_kib", int(hashParams.Memory)),
		attribute.Int("argon2.iterations", int(hashParams.Iterations)),
		attribute.Int("argon2.parallelism", int(hashParams.Parallelism)),
	)

	if password == "" {
		span.RecordError(fmt.Errorf("password can not be empty"))
		span.SetStatus(codes.Error, "empty password")
		return "", fmt.Errorf("password can not be empty")
	}
	release := acquireHashSlot()
	defer release()
	hashed, err := argon2id.CreateHash(password, hashParams)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "hash failed")
		return "", fmt.Errorf("failed to hash password: %v", err)
	}
	return hashed, nil
}

func CheckPassword(ctx context.Context, hashedPassword string, password string) error {
	ctx, span := tracer.Start(ctx, "password.verify")
	defer span.End()
	if m, t, p := decodeParams(hashedPassword); m > 0 {
		span.SetAttributes(
			attribute.Int("argon2.memory_kib", int(m)),
			attribute.Int("argon2.iterations", int(t)),
			attribute.Int("argon2.parallelism", int(p)),
		)
	}

	release := acquireHashSlot()
	defer release()
	match, err := argon2id.ComparePasswordAndHash(password, hashedPassword)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "verify failed")
		return fmt.Errorf("failed to compare password: %v", err)
	}
	if !match {
		span.SetStatus(codes.Error, "password mismatch")
		return fmt.Errorf("password does not match")
	}
	return nil
}
