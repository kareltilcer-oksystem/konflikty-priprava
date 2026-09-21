package auth

import (
	"strings"
	"testing"
)

const sample = `admin:tajne123:Petr Admin:admin;jan:heslo1:Jan Novák:editor;eva:heslo2:Eva Dvořáková:editor`

func TestParseUsersSample(t *testing.T) {
	reg, err := ParseUsers(sample)
	if err != nil {
		t.Fatalf("ParseUsers: %v", err)
	}
	if reg.Count() != 3 {
		t.Errorf("Count = %d, want 3", reg.Count())
	}
	if reg.AdminCount() != 1 {
		t.Errorf("AdminCount = %d, want 1", reg.AdminCount())
	}
	a, ok := reg.Lookup("eva")
	if !ok {
		t.Fatal("eva not found")
	}
	if a.DisplayName != "Eva Dvořáková" || a.Role != RoleEditor || a.IsAdmin() {
		t.Errorf("eva = %+v, want the editor Eva Dvořáková", a)
	}
	if adm, _ := reg.Lookup("admin"); !adm.IsAdmin() {
		t.Error("admin should carry the admin role")
	}
}

func TestParseUsersRejectsMalformed(t *testing.T) {
	cases := []struct{ name, raw string }{
		{"three fields", "admin:tajne:Petr Admin"},
		{"five fields — colon in the display name", "admin:tajne:Petr Admin, CTO::admin"},
		{"colon in the password", "admin:taj:ne:Petr Admin:admin"},
		{"bad role", "admin:tajne:Petr Admin:owner"},
		{"empty username", ":tajne:Petr Admin:admin"},
		{"empty password", "admin::Petr Admin:admin"},
		{"empty display name", "admin:tajne::admin"},
		{"duplicate username", "admin:a:A:admin;admin:b:B:editor"},
		{"no accounts", ""},
		{"no admin", "jan:heslo1:Jan Novák:editor"},
	}
	for _, c := range cases {
		if _, err := ParseUsers(c.raw); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}

// A misconfigured AUTH_USERS is reported at start-up; the message must never
// carry a password into the log.
func TestParseErrorsNeverLeakPasswords(t *testing.T) {
	const secret = "sup3rsecret"
	raws := []string{
		"admin:" + secret + ":Petr Admin",                               // too few fields
		"admin:" + secret + ":Petr Admin, CTO::admin",                   // too many fields
		"admin:" + secret + ":Petr Admin:owner",                         // bad role
		"admin:" + secret + ":Petr:admin;admin:" + secret + ":P:editor", // duplicate
	}
	for _, raw := range raws {
		_, err := ParseUsers(raw)
		if err == nil {
			t.Fatalf("expected an error for %q", raw)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error message leaked the password: %v", err)
		}
	}
}

// A trailing ';' in a .env file is an easy mistake and should be tolerated.
func TestParseUsersToleratesTrailingSeparator(t *testing.T) {
	reg, err := ParseUsers(sample + ";")
	if err != nil {
		t.Fatalf("trailing ';' should be tolerated: %v", err)
	}
	if reg.Count() != 3 {
		t.Errorf("Count = %d, want 3", reg.Count())
	}
}

func TestVerify(t *testing.T) {
	reg, err := ParseUsers(sample)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := reg.Verify("jan", "heslo1"); !ok || a.Username != "jan" {
		t.Error("correct credentials should verify")
	}
	if _, ok := reg.Verify("jan", "heslo2"); ok {
		t.Error("wrong password must not verify")
	}
	if _, ok := reg.Verify("nikdo", "heslo1"); ok {
		t.Error("unknown username must not verify")
	}
	if _, ok := reg.Verify("jan", ""); ok {
		t.Error("empty password must not verify")
	}
}

func TestNewTokenIsUniqueAndHex(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 256; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != TokenBytes*2 {
			t.Fatalf("token length %d, want %d", len(tok), TokenBytes*2)
		}
		if seen[tok] {
			t.Fatal("NewToken returned a duplicate")
		}
		seen[tok] = true
	}
}
