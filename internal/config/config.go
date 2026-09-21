// Package config loads the app's entire configuration from environment
// variables. There is no config file (PRD 10).
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Defaults from PRD 10.
const (
	DefaultDataDir              = "./data"
	DefaultAPIPort              = 9998
	DefaultWebPort              = 9999
	DefaultMaxUploadBytes       = 104857600 // 100 MB per attachment
	DefaultMaxAttachmentsPerReq = 20
	DefaultMaxRequestBytes      = 536870912 // 512 MB per multipart body
	DefaultSessionTTLHours      = 720       // 30 days
	DefaultArchiveAfterDays     = 365
)

// Config is the resolved configuration. Paths are absolute and slash-separated.
type Config struct {
	AuthUsersRaw string // parsed by internal/auth; kept raw so config has no dependency on it

	DataDir   string
	DBPath    string
	AttachDir string
	TmpDir    string

	APIPort int
	WebPort int // 0 means: do not open the SPA listener at all (what development sets)

	CORSOrigins []string

	MaxUploadBytes           int64
	MaxAttachmentsPerRequest int
	MaxRequestBytes          int64

	SessionTTL       time.Duration
	ArchiveAfterDays int
}

// SPAEnabled reports whether the second listener should be opened.
func (c *Config) SPAEnabled() bool { return c.WebPort != 0 }

// Load resolves the configuration using the supplied lookup function. Injecting
// getenv (rather than calling os.Getenv directly) keeps tests parallel-safe.
// Every problem found is reported together, so a misconfigured deployment is
// fixed in one pass instead of one restart per mistake.
func Load(getenv func(string) string) (*Config, error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	c := &Config{}

	c.AuthUsersRaw = strings.TrimSpace(getenv("AUTH_USERS"))
	if c.AuthUsersRaw == "" {
		fail("AUTH_USERS is required (format: username:password:Display Name:role, accounts separated by ';')")
	}

	dataDir := getenv("DATA_DIR")
	if strings.TrimSpace(dataDir) == "" {
		dataDir = DefaultDataDir
	}
	// '?' and '#' silently truncate the SQLite DSN, which would open a
	// different database than the one configured.
	if strings.ContainsAny(dataDir, "?#") {
		fail("DATA_DIR must not contain '?' or '#'")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		fail("DATA_DIR %q is not a usable path: %v", dataDir, err)
	} else {
		c.DataDir = abs
		c.DBPath = filepath.Join(abs, "app.db")
		c.AttachDir = filepath.Join(abs, "attachments")
		c.TmpDir = filepath.Join(abs, "tmp")
	}

	c.APIPort = intVar(getenv, "API_PORT", DefaultAPIPort, fail)
	if c.APIPort < 1 || c.APIPort > 65535 {
		// 0 is not "disabled" here: the API listener is the process's reason to exist.
		fail("API_PORT must be between 1 and 65535, got %d", c.APIPort)
	}
	c.WebPort = intVar(getenv, "WEB_PORT", DefaultWebPort, fail)
	if c.WebPort < 0 || c.WebPort > 65535 {
		fail("WEB_PORT must be 0 (no SPA listener) or between 1 and 65535, got %d", c.WebPort)
	}
	if c.WebPort != 0 && c.WebPort == c.APIPort {
		fail("WEB_PORT and API_PORT are both %d; they must differ (set WEB_PORT=0 in development)", c.WebPort)
	}

	c.CORSOrigins, err = parseOrigins(getenv("CORS_ORIGINS"))
	if err != nil {
		errs = append(errs, err)
	}

	c.MaxUploadBytes = int64(intVar(getenv, "MAX_UPLOAD_BYTES", DefaultMaxUploadBytes, fail))
	if c.MaxUploadBytes < 1 {
		fail("MAX_UPLOAD_BYTES must be positive, got %d", c.MaxUploadBytes)
	}
	c.MaxAttachmentsPerRequest = intVar(getenv, "MAX_ATTACHMENTS_PER_REQUEST", DefaultMaxAttachmentsPerReq, fail)
	if c.MaxAttachmentsPerRequest < 1 {
		fail("MAX_ATTACHMENTS_PER_REQUEST must be positive, got %d", c.MaxAttachmentsPerRequest)
	}
	c.MaxRequestBytes = int64(intVar(getenv, "MAX_REQUEST_BYTES", DefaultMaxRequestBytes, fail))
	if c.MaxRequestBytes < c.MaxUploadBytes {
		fail("MAX_REQUEST_BYTES (%d) must be at least MAX_UPLOAD_BYTES (%d), or no single file could ever be uploaded",
			c.MaxRequestBytes, c.MaxUploadBytes)
	}

	ttl := intVar(getenv, "SESSION_TTL_HOURS", DefaultSessionTTLHours, fail)
	if ttl < 1 {
		fail("SESSION_TTL_HOURS must be positive, got %d", ttl)
	}
	c.SessionTTL = time.Duration(ttl) * time.Hour

	c.ArchiveAfterDays = intVar(getenv, "ARCHIVE_AFTER_DAYS", DefaultArchiveAfterDays, fail)
	if c.ArchiveAfterDays < 0 {
		fail("ARCHIVE_AFTER_DAYS must not be negative, got %d", c.ArchiveAfterDays)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// EnsureDirs creates DATA_DIR and its subdirectories, and empties the staging
// directory. Anything left in tmp/ is the debris of a request that never
// committed, so a restart is the natural moment to clear it.
func (c *Config) EnsureDirs() error {
	for _, d := range []string{c.DataDir, c.AttachDir, c.TmpDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	entries, err := os.ReadDir(c.TmpDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", c.TmpDir, err)
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(c.TmpDir, e.Name()))
	}
	return nil
}

func intVar(getenv func(string) string, name string, def int, fail func(string, ...any)) int {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		fail("%s must be a whole number, got %q", name, raw)
		return def
	}
	return n
}

// parseOrigins validates CORS_ORIGINS. Empty — the normal case — means no CORS
// headers at all, because the SPA is same-origin.
func parseOrigins(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		o := strings.TrimSpace(part)
		if o == "" {
			continue
		}
		// Browsers reject the wildcard together with credentials, so accepting
		// it here would produce a configuration that can never work.
		if o == "*" {
			return nil, errors.New(`CORS_ORIGINS must not contain "*": browsers reject it on credentialed requests; list origins explicitly`)
		}
		u, err := url.Parse(o)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("CORS_ORIGINS entry %q is not an origin like http://host:9999", o)
		}
		if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("CORS_ORIGINS entry %q must be scheme://host[:port] with no path", o)
		}
		out = append(out, u.Scheme+"://"+u.Host)
	}
	return out, nil
}
