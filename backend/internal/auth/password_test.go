package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("correct password did not verify")
	}
}

func TestHashPassword_SaltIsRandom(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("two hashes of the same password are identical — salt not random")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := HashPassword("s3cret-value")

	ok, err := VerifyPassword("not-the-password", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	for _, bad := range []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=65536,t=1,p=4$only-one-part",
		"$bcrypt$v=19$m=1$x$y",
	} {
		if ok, err := VerifyPassword("x", bad); ok || err == nil {
			t.Errorf("VerifyPassword(%q) = (%v, %v), want (false, err)", bad, ok, err)
		}
	}
}
