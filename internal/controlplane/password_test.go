package controlplane

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestDerivePBKDF2MatchesReferenceVector(t *testing.T) {
	t.Parallel()
	want, err := hex.DecodeString("ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43")
	if err != nil {
		t.Fatal(err)
	}
	got := derivePBKDF2([]byte("password"), []byte("salt"), 2, 32)
	if string(got) != string(want) {
		t.Fatalf("derivePBKDF2() = %x, want %x", got, want)
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "valid", password: "a-long-password"},
		{name: "unicode", password: "contraseña-muy-segura"},
		{name: "too short", password: "short", wantErr: true},
		{name: "too long", password: strings.Repeat("x", maxPasswordBytes+1), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := hashPassword(tc.password)
			if tc.wantErr {
				if err == nil {
					t.Fatal("hashPassword() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !verifyPassword(encoded, tc.password) {
				t.Fatal("verifyPassword() = false, want true")
			}
			if verifyPassword(encoded, tc.password+"-wrong") {
				t.Fatal("verifyPassword(wrong) = true, want false")
			}
		})
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	t.Parallel()
	for _, encoded := range []string{
		"",
		"bcrypt$1$abc$abc",
		"pbkdf2-sha256$nope$abc$abc",
		"pbkdf2-sha256$1$abc$abc",
		"pbkdf2-sha256$999999999$abc$abc",
	} {
		if verifyPassword(encoded, "a-long-password") {
			t.Fatalf("verifyPassword(%q) = true, want false", encoded)
		}
	}
}
