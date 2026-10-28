package main

import (
	"strings"
	"testing"
)

func TestEncryptPasswordMatchesNodeCipher(t *testing.T) {
	// Expected value from Node: crypto aes-128-cbc with the same key/iv.
	if got, want := encryptPassword("hunter2"), "8805009E8F998663D6D8119DE721EA25"; got != want {
		t.Fatalf("encryptPassword = %s, want %s", got, want)
	}
}

func TestJWTRoundTrip(t *testing.T) {
	token := signJWT(JWTUser{Email: "a@b.c", JWT: "x"})
	u, err := verifyJWT(token)
	if err != nil || u.Email != "a@b.c" {
		t.Fatalf("verify failed: %v %+v", err, u)
	}
	if _, err := verifyJWT(token[:len(token)-2] + "xx"); err == nil {
		t.Fatal("tampered token accepted")
	}
}

func TestSanitizeFileTitle(t *testing.T) {
	cases := map[string]string{
		`A: "B"/C?`:   "A BC",
		"  dots...  ": "dots",
		"x\ty":        "xy", // tabs are control chars, stripped like in Node,
	}
	for in, want := range cases {
		if got := strings.TrimSpace(sanitizeFileTitle(in)); got != want {
			t.Errorf("sanitizeFileTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNegotiateLanguage(t *testing.T) {
	if got := negotiateLanguage([]string{"pt-BR", "sv-FI", "en"}); got != "sv" {
		t.Fatalf("got %s", got)
	}
	if got := matchLanguage("SV_fi"); got != "sv" {
		t.Fatalf("got %s", got)
	}
}
