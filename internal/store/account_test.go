package store

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// B3-22 row 5, B-91: the filler that an unknown user is compared against costs
// what a real hash costs, or its time would tell who exists.
func TestB3_22_5_FillerHashCostsTheSame(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(fillerPasswordHash))
	if err != nil {
		t.Fatalf("filler is not bcrypt: %v", err)
	}
	if cost != 12 || PasswordCost != 12 {
		t.Fatalf("filler cost %d and PasswordCost %d, want 12", cost, PasswordCost)
	}
	if PasswordMatches("", "anything-at-all") || PasswordMatches("not bcrypt", "anything-at-all") {
		t.Fatal("a bad hash matched")
	}
	CompareWithFiller("anything-at-all")
}

func TestB91_ValidPassword(t *testing.T) {
	cases := map[string]bool{
		"123456789":  false,
		"1234567890": true,
		"ññññññññññ": true,
	}
	for password, want := range cases {
		if got := ValidPassword(password); got != want {
			t.Fatalf("ValidPassword(%q) = %v, want %v", password, got, want)
		}
	}
}

// B3-22 row 1, B-91: a stranger cannot grow the maps without limit, and the
// entries that go first are the stale ones, then the oldest.
func TestB3_22_1_AttemptsStayBounded(t *testing.T) {
	l := NewLoginAttempts()
	at := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxTrackedAttempts + 500 {
		if wait := l.Reserve(fmt.Sprintf("ip-%d", i), fmt.Sprintf("name-%d", i), at.Add(time.Duration(i)*time.Millisecond)); wait != 0 {
			t.Fatalf("attempt %d waited %v", i, wait)
		}
	}
	if len(l.ips) > maxTrackedAttempts || len(l.names) > maxTrackedAttempts {
		t.Fatalf("tracked %d IPs and %d names, want at most %d", len(l.ips), len(l.names), maxTrackedAttempts)
	}
	if _, ok := l.ips["ip-0"]; ok {
		t.Fatal("the oldest IP stayed")
	}
	if _, ok := l.ips[fmt.Sprintf("ip-%d", maxTrackedAttempts+499)]; !ok {
		t.Fatal("the newest IP went")
	}

	// Past the window, a new attempt sweeps every stale entry.
	l.Reserve("late", "late", at.Add(time.Hour))
	if len(l.ips) != 1 || len(l.names) != 1 {
		t.Fatalf("after the window: %d IPs, %d names", len(l.ips), len(l.names))
	}
}

// A streak is forgotten 15 minutes after its last failure.
func TestB91_StreakIsForgotten(t *testing.T) {
	l := NewLoginAttempts()
	at := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	for i := range 3 {
		if wait := l.Reserve(fmt.Sprintf("ip-%d", i), "name", at); wait != 0 {
			t.Fatalf("failure %d waited %v", i+1, wait)
		}
	}
	if wait := l.Reserve("ip-9", "name", at); wait != time.Second {
		t.Fatalf("wait after 3 failures: %v", wait)
	}
	at = at.Add(attemptWindow)
	if wait := l.Reserve("ip-9", "name", at); wait != 0 {
		t.Fatalf("past the window: %v", wait)
	}
	if l.names["name"].fails != 1 {
		t.Fatalf("streak: %d, want a new one", l.names["name"].fails)
	}
}
