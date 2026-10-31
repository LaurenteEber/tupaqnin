package config_test

import (
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/config"
)

func TestT9_MetaWithoutAPIKeyFailsNamingVariable(t *testing.T) {
	t.Setenv("TQ_PROVIDER", "meta")
	t.Setenv("META_API_KEY", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load: got no error, want one naming META_API_KEY")
	}
	if !strings.Contains(err.Error(), "META_API_KEY") {
		t.Fatalf("Load: error %q does not name META_API_KEY", err)
	}
}

func TestT9_FakeProviderNeedsNoAPIKey(t *testing.T) {
	t.Setenv("TQ_PROVIDER", "fake")
	t.Setenv("META_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Provider != "fake" {
		t.Fatalf("Load: got provider %q, want %q", cfg.Provider, "fake")
	}
}

func TestT9_Defaults(t *testing.T) {
	t.Setenv("TQ_PROVIDER", "fake")
	t.Setenv("META_API_KEY", "")
	t.Setenv("PORT", "")
	t.Setenv("TQ_DATA_DIR", "")
	t.Setenv("TQ_CAP_PROJECT_USD", "")
	t.Setenv("TQ_CAP_DAY_USD", "")
	t.Setenv("TQ_CAP_TOTAL_USD", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Load: got port %q, want %q", cfg.Port, "8080")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("Load: got data dir %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.SpendCapProjectUSD != 3 || cfg.SpendCapDayUSD != 10 || cfg.SpendCapTotalUSD != 60 {
		t.Errorf("Load: got caps %v/%v/%v, want 3/10/60",
			cfg.SpendCapProjectUSD, cfg.SpendCapDayUSD, cfg.SpendCapTotalUSD)
	}
}

func TestT9_InvalidValuesNameTheirVariable(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "provider", key: "TQ_PROVIDER", value: "other"},
		{name: "port", key: "PORT", value: "abc"},
		{name: "port too large", key: "PORT", value: "65536"},
		{name: "project cap", key: "TQ_CAP_PROJECT_USD", value: "x"},
		{name: "day cap", key: "TQ_CAP_DAY_USD", value: "x"},
		{name: "total cap", key: "TQ_CAP_TOTAL_USD", value: "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TQ_PROVIDER", "fake")
			t.Setenv("META_API_KEY", "")
			t.Setenv(tt.key, tt.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("Load: got no error, want one naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("Load: error %q does not name %s", err, tt.key)
			}
		})
	}
}

func TestB60_EvidenceContactAndBaseURLs(t *testing.T) {
	t.Setenv("TQ_PROVIDER", "fake")
	t.Setenv("META_API_KEY", "")
	t.Setenv("TQ_CONTACT_EMAIL", "eber@example.org")
	t.Setenv("TQ_EVIDENCE_BASE_URLS", "openalex=http://api.openalex.test/, arxiv=http://export.arxiv.test,crossref=https://api.crossref.test")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.ContactEmail != "eber@example.org" {
		t.Errorf("Load: got contact %q", cfg.ContactEmail)
	}
	want := map[string]string{"openalex": "http://api.openalex.test/", "arxiv": "http://export.arxiv.test", "crossref": "https://api.crossref.test"}
	if len(cfg.EvidenceBaseURLs) != len(want) {
		t.Fatalf("Load: got base URLs %v, want %v", cfg.EvidenceBaseURLs, want)
	}
	for k, v := range want {
		if cfg.EvidenceBaseURLs[k] != v {
			t.Errorf("Load: base URL %s: got %q, want %q", k, cfg.EvidenceBaseURLs[k], v)
		}
	}
}

func TestB60_EvidenceDefaultsAreEmpty(t *testing.T) {
	t.Setenv("TQ_PROVIDER", "fake")
	t.Setenv("META_API_KEY", "")
	t.Setenv("TQ_CONTACT_EMAIL", "")
	t.Setenv("TQ_EVIDENCE_BASE_URLS", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.ContactEmail != "" || len(cfg.EvidenceBaseURLs) != 0 {
		t.Errorf("Load: got contact %q and base URLs %v, want none (the real APIs)", cfg.ContactEmail, cfg.EvidenceBaseURLs)
	}
}

func TestB60_InvalidEvidenceValuesNameTheirVariable(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "contact without at", key: "TQ_CONTACT_EMAIL", value: "eber.example.org"},
		{name: "contact with header break", key: "TQ_CONTACT_EMAIL", value: "eber@example.org\r\nX-Evil: 1"},
		{name: "contact with paren", key: "TQ_CONTACT_EMAIL", value: "eber@example.org)"},
		{name: "unknown service", key: "TQ_EVIDENCE_BASE_URLS", value: "web=http://web.test"},
		{name: "no equals", key: "TQ_EVIDENCE_BASE_URLS", value: "openalex"},
		{name: "bad scheme", key: "TQ_EVIDENCE_BASE_URLS", value: "openalex=file:///etc"},
		{name: "no host", key: "TQ_EVIDENCE_BASE_URLS", value: "arxiv=http://"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TQ_PROVIDER", "fake")
			t.Setenv("META_API_KEY", "")
			t.Setenv(tt.key, tt.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("Load: got no error, want one naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("Load: error %q does not name %s", err, tt.key)
			}
		})
	}
}
