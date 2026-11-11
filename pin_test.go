package main

import (
	"testing"

	"github.com/caarlos0/env/v10"
)

func TestPairingPINConfiguration(t *testing.T) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{}}); err == nil {
		t.Fatal("expected missing HOMEKIT_PIN to reject configuration")
	}
}

func TestValidatePIN(t *testing.T) {
	for _, test := range []struct {
		name  string
		pin   string
		valid bool
	}{
		{"missing", "", false},
		{"shared library default", "00102003", false},
		{"known insecure", "12345678", false},
		{"nondigits", "0314abcd", false},
		{"incorrect length", "0314515", false},
		{"deployment pin", "03145154", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validatePIN(test.pin)
			if (err == nil) != test.valid {
				t.Fatalf("expected valid=%t, got error=%v", test.valid, err)
			}
		})
	}
}
