// Package config reads the application's configuration from
// environment variables only (T-9) and builds its JSON logger (T-10).
// There is no config file. Secrets and document content are never
// logged; a provider error is logged with its code and type only.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Provider names accepted in TQ_PROVIDER (B-6).
const (
	ProviderMeta = "meta"
	ProviderFake = "fake"
)

// Config is the application configuration, loaded from the
// environment. Spend caps are in USD (specification §6).
type Config struct {
	Provider           string
	MetaAPIKey         string
	Port               string
	DataDir            string
	SpendCapProjectUSD float64
	SpendCapDayUSD     float64
	SpendCapTotalUSD   float64

	// ContactEmail goes in the User-Agent of the evidence connectors
	// (TQ_CONTACT_EMAIL, B-60); OpenAlex asks for it. Empty: no contact.
	ContactEmail string
	// EvidenceBaseURLs replaces the OpenAlex, arXiv and Crossref base
	// URLs (TQ_EVIDENCE_BASE_URLS, tests only), as
	// "openalex=URL,arxiv=URL,crossref=URL". Empty: the real APIs.
	EvidenceBaseURLs map[string]string
}

// DataDir is TQ_DATA_DIR or its default. The invite command needs only
// this, not the provider's configuration (B-92).
func DataDir() string { return envOr("TQ_DATA_DIR", "./data") }

// Load reads and validates the configuration. An empty variable takes
// its default; if a required variable is missing or a value is
// invalid, the error names the variable.
func Load() (Config, error) {
	cfg := Config{
		Provider:   envOr("TQ_PROVIDER", ProviderMeta),
		MetaAPIKey: os.Getenv("META_API_KEY"),
		Port:       envOr("PORT", "8080"),
		DataDir:    DataDir(),
	}

	switch cfg.Provider {
	case ProviderMeta:
		if cfg.MetaAPIKey == "" {
			return Config{}, fmt.Errorf("META_API_KEY is required when TQ_PROVIDER=%s", ProviderMeta)
		}
	case ProviderFake:
	default:
		return Config{}, fmt.Errorf("TQ_PROVIDER: invalid value %q: want %q or %q", cfg.Provider, ProviderMeta, ProviderFake)
	}

	if err := validatePort(cfg.Port); err != nil {
		return Config{}, err
	}

	var err error
	if cfg.SpendCapProjectUSD, err = parseCap("TQ_CAP_PROJECT_USD", 3); err != nil {
		return Config{}, err
	}
	if cfg.SpendCapDayUSD, err = parseCap("TQ_CAP_DAY_USD", 10); err != nil {
		return Config{}, err
	}
	if cfg.SpendCapTotalUSD, err = parseCap("TQ_CAP_TOTAL_USD", 60); err != nil {
		return Config{}, err
	}

	if cfg.ContactEmail, err = parseContact("TQ_CONTACT_EMAIL"); err != nil {
		return Config{}, err
	}
	if cfg.EvidenceBaseURLs, err = parseBaseURLs("TQ_EVIDENCE_BASE_URLS"); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// NewLogger returns the application logger: JSON to standard output
// (T-10). Callers must never log document text, human messages, the
// API key or raw provider error bodies through it.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func validatePort(raw string) error {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT: invalid value %q: want a number between 1 and 65535", raw)
	}
	return nil
}

func parseCap(key string, fallback float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid value %q: want a number", key, raw)
	}
	return value, nil
}

// parseContact reads the contact e-mail. It goes into a header, so a
// value with spaces, control characters or parentheses is refused.
// [EL DISEÑO NO LO DICE] how to validate it: an @ and none of those.
func parseContact(key string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "@") || strings.ContainsAny(raw, " ()<>\t\r\n") {
		return "", fmt.Errorf("%s: invalid value: want an e-mail address", key)
	}
	for _, r := range raw {
		if r < 0x21 || r == 0x7f {
			return "", fmt.Errorf("%s: invalid value: want an e-mail address", key)
		}
	}
	return raw, nil
}

// evidenceServices are the services TQ_EVIDENCE_BASE_URLS may point
// elsewhere (they match internal/evidence's names).
var evidenceServices = map[string]bool{"openalex": true, "arxiv": true, "crossref": true}

// parseBaseURLs reads "service=URL" pairs separated by commas.
// [EL DISEÑO NO LO DICE] the format of TQ_EVIDENCE_BASE_URLS.
func parseBaseURLs(key string) (map[string]string, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || !evidenceServices[name] {
			return nil, fmt.Errorf("%s: invalid entry %q: want openalex, arxiv or crossref = URL", key, name)
		}
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("%s: invalid URL for %s: want http or https with a host", key, name)
		}
		out[name] = value
	}
	return out, nil
}
