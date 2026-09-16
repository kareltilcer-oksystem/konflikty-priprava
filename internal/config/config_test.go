package config

import (
	"strings"
	"testing"
	"time"
)

// env builds a getenv function from a map, so tests never touch the process
// environment and can run in parallel.
func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	c, err := Load(env(map[string]string{"AUTH_USERS": "admin:a:A:admin"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIPort != DefaultAPIPort || c.WebPort != DefaultWebPort {
		t.Errorf("ports = %d/%d, want %d/%d", c.APIPort, c.WebPort, DefaultAPIPort, DefaultWebPort)
	}
	if c.MaxUploadBytes != DefaultMaxUploadBytes || c.MaxRequestBytes != DefaultMaxRequestBytes {
		t.Errorf("upload caps = %d/%d", c.MaxUploadBytes, c.MaxRequestBytes)
	}
	if c.MaxAttachmentsPerRequest != 20 {
		t.Errorf("MaxAttachmentsPerRequest = %d, want 20", c.MaxAttachmentsPerRequest)
	}
	if c.SessionTTL != 720*time.Hour {
		t.Errorf("SessionTTL = %v, want 720h", c.SessionTTL)
	}
	if c.ArchiveAfterDays != 365 {
		t.Errorf("ArchiveAfterDays = %d, want 365", c.ArchiveAfterDays)
	}
	if len(c.CORSOrigins) != 0 {
		t.Errorf("CORSOrigins = %v, want empty by default", c.CORSOrigins)
	}
	if !c.SPAEnabled() {
		t.Error("the SPA listener should be enabled by default")
	}
	if !strings.HasSuffix(c.DBPath, "app.db") {
		t.Errorf("DBPath = %q, want it to end in app.db", c.DBPath)
	}
}

// WEB_PORT=0 is what development sets, because Vite owns 9999.
func TestWebPortZeroDisablesSPAListener(t *testing.T) {
	t.Parallel()
	c, err := Load(env(map[string]string{"AUTH_USERS": "admin:a:A:admin", "WEB_PORT": "0"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.SPAEnabled() {
		t.Error("WEB_PORT=0 must not open the SPA listener")
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		envv map[string]string
		want string
	}{
		{"missing AUTH_USERS", map[string]string{}, "AUTH_USERS"},
		{"same ports", map[string]string{"AUTH_USERS": "a:a:A:admin", "WEB_PORT": "9998"}, "must differ"},
		{"api port 0", map[string]string{"AUTH_USERS": "a:a:A:admin", "API_PORT": "0"}, "API_PORT"},
		{"non-numeric port", map[string]string{"AUTH_USERS": "a:a:A:admin", "API_PORT": "nope"}, "whole number"},
		{"cors wildcard", map[string]string{"AUTH_USERS": "a:a:A:admin", "CORS_ORIGINS": "*"}, `"*"`},
		{"cors with path", map[string]string{"AUTH_USERS": "a:a:A:admin", "CORS_ORIGINS": "http://srv:9999/app"}, "no path"},
		{"cors not an origin", map[string]string{"AUTH_USERS": "a:a:A:admin", "CORS_ORIGINS": "srv:9999"}, "not an origin"},
		{"datadir with query char", map[string]string{"AUTH_USERS": "a:a:A:admin", "DATA_DIR": "./da?ta"}, "'?'"},
		{"request smaller than file", map[string]string{"AUTH_USERS": "a:a:A:admin", "MAX_REQUEST_BYTES": "1000", "MAX_UPLOAD_BYTES": "2000"}, "at least"},
		{"zero ttl", map[string]string{"AUTH_USERS": "a:a:A:admin", "SESSION_TTL_HOURS": "0"}, "SESSION_TTL_HOURS"},
	}
	for _, c := range cases {
		_, err := Load(env(c.envv))
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.want)
		}
	}
}

// Every problem should be reported at once, not one restart at a time.
func TestLoadReportsAllProblemsTogether(t *testing.T) {
	t.Parallel()
	_, err := Load(env(map[string]string{"API_PORT": "0", "SESSION_TTL_HOURS": "-1"}))
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{"AUTH_USERS", "API_PORT", "SESSION_TTL_HOURS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("combined error does not mention %s: %s", want, msg)
		}
	}
}

func TestParseOrigins(t *testing.T) {
	t.Parallel()
	got, err := parseOrigins(" http://vyvoj-srv:9999 , http://10.0.0.7:9999 ")
	if err != nil {
		t.Fatalf("parseOrigins: %v", err)
	}
	want := []string{"http://vyvoj-srv:9999", "http://10.0.0.7:9999"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origin %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEnsureDirsClearsStaging(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(env(map[string]string{"AUTH_USERS": "a:a:A:admin", "DATA_DIR": dir}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	// Debris from a request that never committed must not survive a restart.
	stale := c.TmpDir + "/leftover.bin"
	if err := writeFile(stale, "x"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs (second call): %v", err)
	}
	if fileExists(stale) {
		t.Error("EnsureDirs should have emptied the staging directory")
	}
}
