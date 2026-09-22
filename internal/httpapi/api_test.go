package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/auth"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/config"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

const testUsers = `admin:tajne123:Petr Admin:admin;jan:heslo1:Jan Novák:editor;eva:heslo2:Eva Dvořáková:editor`

// fixedNow pins "today" so the archive cutoff is deterministic.
var fixedNow = time.Date(2026, 9, 16, 8, 30, 0, 0, time.UTC)

type harness struct {
	t       *testing.T
	handler http.Handler
	cfg     *config.Config
}

// newHarness builds an API over a throwaway database and data directory.
func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir() // first, so Close runs before RemoveAll (Windows)

	cfg, err := config.Load(func(k string) string {
		switch k {
		case "AUTH_USERS":
			return testUsers
		case "DATA_DIR":
			return dir
		case "MAX_UPLOAD_BYTES":
			return "1024"
		case "MAX_ATTACHMENTS_PER_REQUEST":
			return "2"
		case "MAX_REQUEST_BYTES":
			return "4096"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	users, err := auth.ParseUsers(testUsers)
	if err != nil {
		t.Fatalf("users: %v", err)
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return &harness{
		t: t,
		handler: New(Deps{
			Store: st, Users: users, Config: cfg, Version: "test",
			Now:        func() time.Time { return fixedNow },
			LoginDelay: 0, // no artificial delay in tests
		}),
		cfg: cfg,
	}
}

// do sends a request and returns the recorder.
func (h *harness) do(method, path string, body io.Reader, cookie string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, body)
	if cookie != "" {
		req.Header.Set("Cookie", SessionCookie+"="+cookie)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) doJSON(method, path, body, cookie string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", SessionCookie+"="+cookie)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

// login returns a session token for a username.
func (h *harness) login(username, password string) string {
	h.t.Helper()
	rec := h.doJSON(http.MethodPost, "/api/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), "")
	if rec.Code != http.StatusOK {
		h.t.Fatalf("login as %s: %d %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			return c.Value
		}
	}
	h.t.Fatalf("login as %s set no session cookie", username)
	return ""
}

func (h *harness) admin() string  { return h.login("admin", "tajne123") }
func (h *harness) editor() string { return h.login("jan", "heslo1") }

// decode unmarshals a response body.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[apiError](t, rec).Error.Code
}

// createProblem is a convenience for tests that need a problem to exist.
func (h *harness) createProblem(token, title string) int64 {
	h.t.Helper()
	rec := h.doJSON(http.MethodPost, "/api/problems", fmt.Sprintf(`{"title":%q}`, title), token)
	if rec.Code != http.StatusCreated {
		h.t.Fatalf("create problem: %d %s", rec.Code, rec.Body.String())
	}
	return decode[problemDetailDTO](h.t, rec).ID
}

func (h *harness) createMeeting(token, date string) meetingDetailDTO {
	h.t.Helper()
	rec := h.doJSON(http.MethodPost, "/api/meetings", fmt.Sprintf(`{"meeting_date":%q}`, date), token)
	if rec.Code != http.StatusCreated {
		h.t.Fatalf("create meeting: %d %s", rec.Code, rec.Body.String())
	}
	return decode[meetingDetailDTO](h.t, rec)
}

func TestHealth(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/health", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	got := decode[map[string]string](t, rec)
	if got["status"] != "ok" || got["version"] != "test" {
		t.Errorf("health = %v", got)
	}
}

// Every read endpoint must work without a session, and none may redirect.
func TestReadsArePublic(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	id := h.createProblem(admin, "Veřejný problém")
	m := h.createMeeting(admin, "2026-09-18")

	for _, path := range []string{
		"/api/problems",
		fmt.Sprintf("/api/problems/%d", id),
		"/api/meetings",
		"/api/meetings/" + m.Slug,
		"/api/auth/me",
		"/api/health",
	} {
		rec := h.do(http.MethodGet, path, nil, "")
		if rec.Code != http.StatusOK {
			t.Errorf("anonymous GET %s = %d, want 200", path, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Errorf("anonymous GET %s redirected to %s; it must never do that", path, loc)
		}
	}
}

func TestAuthMe(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/auth/me", nil, "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "null" {
		t.Errorf("anonymous /auth/me = %d %q, want 200 null", rec.Code, rec.Body.String())
	}

	token := h.editor()
	rec = h.do(http.MethodGet, "/api/auth/me", nil, token)
	u := decode[userDTO](t, rec)
	if u.Username != "jan" || u.DisplayName != "Jan Novák" || u.Role != "editor" {
		t.Errorf("/auth/me = %+v", u)
	}
}

func TestLoginFailure(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode[apiError](t, rec)
	if body.Error.Code != CodeInvalidCredentials || body.Error.Message != "Nesprávné jméno nebo heslo." {
		t.Errorf("error = %+v", body.Error)
	}
	// The message must not say which half was wrong.
	if strings.Contains(strings.ToLower(body.Error.Message), "heslo je") {
		t.Errorf("message is too specific: %q", body.Error.Message)
	}
}

// Using a session must slide it and re-send the cookie, or an actively used
// session would be dropped by the browser on day 30.
func TestSessionSlidesAndReissuesCookie(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.do(http.MethodGet, "/api/auth/me", nil, token)

	var found *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			found = c
		}
	}
	if found == nil {
		t.Fatal("an authenticated request did not re-send the session cookie")
	}
	if found.MaxAge != int(h.cfg.SessionTTL/time.Second) {
		t.Errorf("Max-Age = %d, want %d", found.MaxAge, int(h.cfg.SessionTTL/time.Second))
	}
	if !found.HttpOnly {
		t.Error("the session cookie must be HttpOnly")
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", found.SameSite)
	}
}

func TestLogout(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.do(http.MethodPost, "/api/auth/logout", nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	// The token must no longer authenticate.
	rec = h.do(http.MethodGet, "/api/auth/me", nil, token)
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Errorf("a logged-out token still resolves: %s", rec.Body.String())
	}
	// Logging out without a session is a documented no-op.
	if rec := h.do(http.MethodPost, "/api/auth/logout", nil, ""); rec.Code != http.StatusNoContent {
		t.Errorf("anonymous logout = %d, want 204", rec.Code)
	}
}

// The role table: who may do what.
//
// Each case builds its own harness. The cases mutate shared records — the
// delete case removes the very problem the done case needs — so sharing one
// database between them would make the table order-dependent.
func TestAuthorization(t *testing.T) {
	type fixture struct {
		method, path, body string
	}
	cases := []struct {
		name          string
		build         func(h *harness, admin string) fixture
		anon, ed, adm int
	}{
		{
			name: "create problem",
			build: func(h *harness, admin string) fixture {
				return fixture{http.MethodPost, "/api/problems", `{"title":"x"}`}
			},
			anon: http.StatusUnauthorized, ed: http.StatusCreated, adm: http.StatusCreated,
		},
		{
			name: "edit any problem",
			build: func(h *harness, admin string) fixture {
				id := h.createProblem(admin, "cizí problém")
				return fixture{http.MethodPatch, fmt.Sprintf("/api/problems/%d", id), `{"title":"y"}`}
			},
			anon: http.StatusUnauthorized, ed: http.StatusOK, adm: http.StatusOK,
		},
		{
			name: "delete problem",
			build: func(h *harness, admin string) fixture {
				id := h.createProblem(admin, "ke smazání")
				return fixture{http.MethodDelete, fmt.Sprintf("/api/problems/%d", id), ""}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusNoContent,
		},
		{
			name: "mark done",
			build: func(h *harness, admin string) fixture {
				id := h.createProblem(admin, "k vyřešení")
				return fixture{http.MethodPost, fmt.Sprintf("/api/problems/%d/done", id), ""}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusOK,
		},
		{
			name: "create meeting",
			build: func(h *harness, admin string) fixture {
				return fixture{http.MethodPost, "/api/meetings", `{"meeting_date":"2026-10-02"}`}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusCreated,
		},
		{
			name: "edit meeting",
			build: func(h *harness, admin string) fixture {
				m := h.createMeeting(admin, "2026-09-18")
				return fixture{http.MethodPatch, "/api/meetings/" + m.Slug, `{"note":"n"}`}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusOK,
		},
		{
			name: "delete meeting",
			build: func(h *harness, admin string) fixture {
				m := h.createMeeting(admin, "2026-09-18")
				return fixture{http.MethodDelete, "/api/meetings/" + m.Slug, ""}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusNoContent,
		},
		{
			name: "add agenda items",
			build: func(h *harness, admin string) fixture {
				m := h.createMeeting(admin, "2026-09-18")
				id := h.createProblem(admin, "na program")
				return fixture{http.MethodPost, "/api/meetings/" + m.Slug + "/items",
					fmt.Sprintf(`{"problem_ids":[%d]}`, id)}
			},
			anon: http.StatusUnauthorized, ed: http.StatusForbidden, adm: http.StatusCreated,
		},
	}

	for _, c := range cases {
		for _, role := range []string{"anonymous", "editor", "admin"} {
			t.Run(c.name+" ("+role+")", func(t *testing.T) {
				h := newHarness(t)
				admin := h.admin()
				f := c.build(h, admin)

				var token string
				var want int
				switch role {
				case "anonymous":
					token, want = "", c.anon
				case "editor":
					token, want = h.editor(), c.ed
				default:
					token, want = admin, c.adm
				}
				if rec := h.doJSON(f.method, f.path, f.body, token); rec.Code != want {
					t.Errorf("= %d, want %d: %s", rec.Code, want, rec.Body.String())
				}
			})
		}
	}
}

func TestProblemValidation(t *testing.T) {
	h := newHarness(t)
	token := h.editor()

	cases := []struct {
		name, body, wantField string
	}{
		{"empty title", `{"title":""}`, "title"},
		{"whitespace title", `{"title":"   "}`, "title"},
		{"title too long", fmt.Sprintf(`{"title":%q}`, strings.Repeat("a", 201)), "title"},
		{"javascript link", `{"title":"x","link":"javascript:alert(1)"}`, "link"},
		{"data link", `{"title":"x","link":"data:text/html,<script>"}`, "link"},
		{"relative link", `{"title":"x","link":"/local/path"}`, "link"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := h.doJSON(http.MethodPost, "/api/problems", c.body, token)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("= %d, want 400: %s", rec.Code, rec.Body.String())
			}
			body := decode[apiError](t, rec)
			if body.Error.Code != CodeValidationFailed {
				t.Errorf("code = %q", body.Error.Code)
			}
			if _, ok := body.Error.Details[c.wantField]; !ok {
				t.Errorf("details %v do not mention %q", body.Error.Details, c.wantField)
			}
		})
	}

	// The empty string is the documented "no link" value and must be accepted.
	if rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"x","link":""}`, token); rec.Code != http.StatusCreated {
		t.Errorf("an empty link should be accepted, got %d: %s", rec.Code, rec.Body.String())
	}
	// http and https are the only accepted schemes.
	for _, link := range []string{"http://jira.local/x", "https://jira.local/x", "HTTPS://JIRA.LOCAL/x"} {
		body := fmt.Sprintf(`{"title":"x","link":%q}`, link)
		if rec := h.doJSON(http.MethodPost, "/api/problems", body, token); rec.Code != http.StatusCreated {
			t.Errorf("link %q was rejected: %s", link, rec.Body.String())
		}
	}
}

// The admin files a problem under someone else's name; nobody else can.
//
// Problems are routinely reported by people who never sign in, so the admin
// enters them — but the Autor column has to name the reporter, not the typist.
func TestCreateProblemOnBehalfOfAnotherUser(t *testing.T) {
	h := newHarness(t)
	admin, editor := h.admin(), h.editor()

	t.Run("admin names another user", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Nahlásila Eva","created_by":"eva"}`, admin)
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		if got := decode[problemDetailDTO](t, rec).CreatedBy; got != "eva" {
			t.Errorf("created_by = %q, want eva", got)
		}
	})

	t.Run("omitted created_by stays the session", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Vlastní"}`, admin)
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		if got := decode[problemDetailDTO](t, rec).CreatedBy; got != "admin" {
			t.Errorf("created_by = %q, want admin", got)
		}
	})

	// A blank value counts as omitting the field, which the contract promises so
	// that a client always serializing the property still gets the session. The
	// editor is the caller here on purpose: if this ever stopped being treated
	// as "omitted" it would fall through to the admin gate and 403 every such
	// submit, which is the regression these two pin.
	t.Run("an empty created_by is the session, not a refusal", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Prázdné","created_by":""}`, editor)
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		if got := decode[problemDetailDTO](t, rec).CreatedBy; got != "jan" {
			t.Errorf("created_by = %q, want jan", got)
		}
	})

	t.Run("a blank-once-trimmed created_by is the session too", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Mezery","created_by":"   "}`, editor)
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		if got := decode[problemDetailDTO](t, rec).CreatedBy; got != "jan" {
			t.Errorf("created_by = %q, want jan", got)
		}
	})

	// Attachments carry the author too: they are part of the same report, and a
	// mismatch would make the detail page name two different people for one
	// submission.
	t.Run("multipart carries the author onto the attachments", func(t *testing.T) {
		rec := h.postMultipart("/api/problems", admin,
			map[string]string{"title": "Se snímkem", "created_by": "eva"},
			map[string][]byte{"snimek.png": []byte("PNG")}, "image/png")
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		got := decode[problemDetailDTO](t, rec)
		if got.CreatedBy != "eva" {
			t.Errorf("created_by = %q, want eva", got.CreatedBy)
		}
		if len(got.Attachments) != 1 || got.Attachments[0].CreatedBy != "eva" {
			t.Errorf("attachment author = %+v", got.Attachments)
		}
	})

	// Naming themselves is not "on behalf of anyone" and needs no role.
	t.Run("editor may name themselves", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Sám za sebe","created_by":"jan"}`, editor)
		if rec.Code != http.StatusCreated {
			t.Fatalf("= %d %s", rec.Code, rec.Body.String())
		}
		if got := decode[problemDetailDTO](t, rec).CreatedBy; got != "jan" {
			t.Errorf("created_by = %q, want jan", got)
		}
	})

	// Refused, not silently filed under the editor: a dropped author would be
	// invisible until someone noticed the wrong name in the column.
	t.Run("editor may not name anyone else", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Za Evu","created_by":"eva"}`, editor)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("= %d, want 403: %s", rec.Code, rec.Body.String())
		}
		if code := errorCode(t, rec); code != CodeForbidden {
			t.Errorf("code = %q", code)
		}
	})

	t.Run("the author must be a configured account", func(t *testing.T) {
		rec := h.doJSON(http.MethodPost, "/api/problems", `{"title":"Za ducha","created_by":"nikdo"}`, admin)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("= %d, want 400: %s", rec.Code, rec.Body.String())
		}
		body := decode[apiError](t, rec)
		if body.Error.Code != CodeValidationFailed {
			t.Errorf("code = %q", body.Error.Code)
		}
		if _, ok := body.Error.Details["created_by"]; !ok {
			t.Errorf("details %v do not mention created_by", body.Error.Details)
		}
	})

	// A refused author is refused before the bytes are committed, like every
	// other rejection on this endpoint.
	t.Run("a refused author leaves no files behind", func(t *testing.T) {
		before := h.countDir(h.cfg.AttachDir)
		rec := h.postMultipart("/api/problems", editor,
			map[string]string{"title": "Za Evu", "created_by": "eva"},
			map[string][]byte{"snimek.png": []byte("PNG")}, "image/png")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("= %d, want 403: %s", rec.Code, rec.Body.String())
		}
		if got := h.countDir(h.cfg.AttachDir); got != before {
			t.Errorf("attachments directory grew from %d to %d", before, got)
		}
		if h.countDir(h.cfg.TmpDir) != 0 {
			t.Errorf("staging was left dirty")
		}
	})
}

// link = "" clears the link; an omitted link leaves it alone.
func TestPatchDistinguishesEmptyFromOmitted(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.doJSON(http.MethodPost, "/api/problems",
		`{"title":"x","link":"https://jira.local/1","description":"popis"}`, token)
	id := decode[problemDetailDTO](t, rec).ID

	// Omitting the link must leave it in place.
	rec = h.doJSON(http.MethodPatch, fmt.Sprintf("/api/problems/%d", id), `{"title":"y"}`, token)
	got := decode[problemDetailDTO](t, rec)
	if got.Link != "https://jira.local/1" || got.Description != "popis" {
		t.Errorf("an omitted field was changed: link=%q description=%q", got.Link, got.Description)
	}
	// An explicit empty string must clear it.
	rec = h.doJSON(http.MethodPatch, fmt.Sprintf("/api/problems/%d", id), `{"link":""}`, token)
	if got := decode[problemDetailDTO](t, rec); got.Link != "" {
		t.Errorf("link = %q, want it cleared", got.Link)
	}
}

// done has its own admin-only endpoint and must not be settable through PATCH.
func TestPatchCannotSetDone(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	id := h.createProblem(token, "x")
	rec := h.doJSON(http.MethodPatch, fmt.Sprintf("/api/problems/%d", id), `{"done":true}`, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH with done = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestBucketFilters(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	open1 := h.createProblem(admin, "Náhled rozdílů nezobrazuje diakritiku správně")
	h.createProblem(admin, "Import nad 5 000 řádků spadne na timeout")
	doneID := h.createProblem(admin, "Duplicitní záznamy po opakovaném importu")
	if rec := h.do(http.MethodPost, fmt.Sprintf("/api/problems/%d/done", doneID), nil, admin); rec.Code != http.StatusOK {
		t.Fatalf("mark done: %d", rec.Code)
	}

	count := func(query string) int {
		rec := h.do(http.MethodGet, "/api/problems"+query, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", query, rec.Code)
		}
		return len(decode[[]problemDTO](t, rec))
	}

	if n := count("?done=false"); n != 2 {
		t.Errorf("the bucket holds %d, want 2", n)
	}
	if n := count("?done=true"); n != 1 {
		t.Errorf("done filter returned %d, want 1", n)
	}
	if n := count(""); n != 3 {
		t.Errorf("no filter returned %d, want 3", n)
	}
	// Diacritics- and case-insensitive text filter.
	if n := count("?q=rozdilu"); n != 1 {
		t.Errorf("q=rozdilu matched %d, want 1", n)
	}
	if n := count("?q=SPRAVNE"); n != 1 {
		t.Errorf("q=SPRAVNE matched %d, want 1", n)
	}
	if n := count("?q=NEEXISTUJE"); n != 0 {
		t.Errorf("q=NEEXISTUJE matched %d, want 0", n)
	}
	// A bad enum or boolean is a 400, not a silent default.
	if rec := h.do(http.MethodGet, "/api/problems?sort=nonsense", nil, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("sort=nonsense = %d, want 400", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/problems?done=maybe", nil, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("done=maybe = %d, want 400", rec.Code)
	}
	_ = open1
}

func TestProblemDetailCarriesTheTimeline(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	id := h.createProblem(admin, "Opakovaný problém")
	early := h.createMeeting(admin, "2026-08-21")
	late := h.createMeeting(admin, "2026-09-18")

	for _, m := range []meetingDetailDTO{late, early} { // added out of order on purpose
		rec := h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items",
			fmt.Sprintf(`{"problem_ids":[%d]}`, id), admin)
		if rec.Code != http.StatusCreated {
			t.Fatalf("add to %s: %d %s", m.Slug, rec.Code, rec.Body.String())
		}
		items := decode[[]meetingItemDTO](t, rec)
		note := "poznámka pro " + m.Slug
		rec = h.doJSON(http.MethodPatch,
			fmt.Sprintf("/api/meetings/%s/items/%d", m.Slug, items[0].ID),
			fmt.Sprintf(`{"prep_note":%q}`, note), admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("set note: %d %s", rec.Code, rec.Body.String())
		}
	}

	rec := h.do(http.MethodGet, fmt.Sprintf("/api/problems/%d", id), nil, "")
	detail := decode[problemDetailDTO](t, rec)
	if len(detail.Meetings) != 2 {
		t.Fatalf("timeline has %d entries, want 2", len(detail.Meetings))
	}
	// Oldest first, each keeping its own note.
	if detail.Meetings[0].Slug != early.Slug || detail.Meetings[1].Slug != late.Slug {
		t.Errorf("timeline order = %s, %s; want oldest first", detail.Meetings[0].Slug, detail.Meetings[1].Slug)
	}
	if detail.Meetings[0].PrepNote != "poznámka pro "+early.Slug {
		t.Errorf("the earlier note was overwritten: %q", detail.Meetings[0].PrepNote)
	}
	if detail.MeetingCount != 2 {
		t.Errorf("meeting_count = %d, want 2", detail.MeetingCount)
	}
}

func TestMeetingSlugsAndDerivedFields(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()

	cases := []struct {
		date           string
		slug           string
		isoYear, isoWk int
	}{
		{"2026-09-18", "2026-w38", 2026, 38},
		{"2027-01-01", "2026-w53", 2026, 53}, // ISO year trails the calendar year
		{"2025-12-29", "2026-w01", 2026, 1},  // ISO year leads it
	}
	for _, c := range cases {
		m := h.createMeeting(admin, c.date)
		if m.Slug != c.slug || m.ISOYear != c.isoYear || m.ISOWeek != c.isoWk {
			t.Errorf("%s -> slug %s, iso %d-%d; want %s, %d-%d",
				c.date, m.Slug, m.ISOYear, m.ISOWeek, c.slug, c.isoYear, c.isoWk)
		}
	}
	// A second meeting in the same ISO week takes the -2 suffix.
	if m := h.createMeeting(admin, "2026-09-17"); m.Slug != "2026-w38-2" {
		t.Errorf("second meeting slug = %s, want 2026-w38-2", m.Slug)
	}
	// An unparseable date is a 400 with a field detail.
	rec := h.doJSON(http.MethodPost, "/api/meetings", `{"meeting_date":"18.9.2026"}`, admin)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad date = %d, want 400", rec.Code)
	}
}

// Changing the date moves the label but never the URL.
func TestMeetingDateChangeKeepsSlug(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	m := h.createMeeting(admin, "2026-09-18")

	rec := h.doJSON(http.MethodPatch, "/api/meetings/"+m.Slug, `{"meeting_date":"2026-09-25"}`, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch = %d %s", rec.Code, rec.Body.String())
	}
	got := decode[meetingDetailDTO](t, rec)
	if got.Slug != "2026-w38" {
		t.Errorf("slug changed to %q; shared links would break", got.Slug)
	}
	if got.ISOWeek != 39 {
		t.Errorf("iso_week = %d, want 39 — the label follows the new date", got.ISOWeek)
	}
	// The old URL must still resolve.
	if rec := h.do(http.MethodGet, "/api/meetings/2026-w38", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("the frozen slug stopped resolving: %d", rec.Code)
	}
}

// The default list is one-sided: future meetings are never filtered out.
func TestMeetingArchiveFilter(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	future := h.createMeeting(admin, "2026-09-25") // after fixedNow
	current := h.createMeeting(admin, "2026-09-11")
	old := h.createMeeting(admin, "2024-09-12") // more than 365 days before fixedNow

	rec := h.do(http.MethodGet, "/api/meetings", nil, "")
	def := decode[[]meetingDTO](t, rec)
	slugs := map[string]bool{}
	for _, m := range def {
		slugs[m.Slug] = true
		if m.Archived {
			t.Errorf("%s is archived but appeared in the default list", m.Slug)
		}
	}
	if !slugs[future.Slug] {
		t.Error("the future meeting was filtered out; it is the one the list exists to surface")
	}
	if !slugs[current.Slug] {
		t.Error("the current meeting is missing")
	}
	if slugs[old.Slug] {
		t.Error("the archived meeting appeared in the default list")
	}
	// Newest first.
	if len(def) >= 2 && def[0].Slug != future.Slug {
		t.Errorf("first row is %s, want the future meeting %s", def[0].Slug, future.Slug)
	}

	rec = h.do(http.MethodGet, "/api/meetings?include_archived=true", nil, "")
	all := decode[[]meetingDTO](t, rec)
	if len(all) != 3 {
		t.Errorf("include_archived returned %d, want 3", len(all))
	}
	// Archived meetings stay reachable at their URL forever.
	if rec := h.do(http.MethodGet, "/api/meetings/"+old.Slug, nil, ""); rec.Code != http.StatusOK {
		t.Errorf("an archived meeting's URL stopped working: %d", rec.Code)
	}
}

func TestAgendaRejections(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	m := h.createMeeting(admin, "2026-09-18")
	p1 := h.createProblem(admin, "první")

	rec := h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items",
		fmt.Sprintf(`{"problem_ids":[%d]}`, p1), admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}

	// A repeated id within one request is a 400 ...
	rec = h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items", `{"problem_ids":[2,2]}`, admin)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != CodeValidationFailed {
		t.Errorf("duplicate id = %d %s, want 400 validation_failed", rec.Code, rec.Body.String())
	}
	// ... while a problem already on the agenda is the separate 409.
	rec = h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items",
		fmt.Sprintf(`{"problem_ids":[%d]}`, p1), admin)
	if rec.Code != http.StatusConflict || errorCode(t, rec) != CodeAlreadyOnAgenda {
		t.Errorf("re-add = %d %s, want 409 already_on_agenda", rec.Code, rec.Body.String())
	}
	// An empty list is a 400.
	rec = h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items", `{"problem_ids":[]}`, admin)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty list = %d, want 400", rec.Code)
	}
	// An unknown problem is a 400, not a 404 that would read as "no such meeting".
	rec = h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items", `{"problem_ids":[9999]}`, admin)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown problem = %d, want 400", rec.Code)
	}
	// An unknown meeting is a 404.
	rec = h.doJSON(http.MethodPost, "/api/meetings/2026-w12/items", `{"problem_ids":[1]}`, admin)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown meeting = %d, want 404", rec.Code)
	}
}

func TestReorderAndRenumber(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	m := h.createMeeting(admin, "2026-09-18")

	var ids []int64
	for i := 0; i < 4; i++ {
		ids = append(ids, h.createProblem(admin, fmt.Sprintf("problém %d", i)))
	}
	body, _ := json.Marshal(map[string][]int64{"problem_ids": ids})
	rec := h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items", string(body), admin)
	items := decode[[]meetingItemDTO](t, rec)

	order := []int64{items[3].ID, items[0].ID, items[2].ID, items[1].ID}
	ob, _ := json.Marshal(map[string][]int64{"item_ids": order})
	rec = h.doJSON(http.MethodPut, "/api/meetings/"+m.Slug+"/items/order", string(ob), admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder = %d %s", rec.Code, rec.Body.String())
	}
	got := decode[[]meetingItemDTO](t, rec)
	for i, it := range got {
		if it.ID != order[i] || it.Position != i {
			t.Errorf("slot %d holds item %d at position %d, want item %d", i, it.ID, it.Position, order[i])
		}
	}

	// A partial list is a 400 and must change nothing.
	rec = h.doJSON(http.MethodPut, "/api/meetings/"+m.Slug+"/items/order", `{"item_ids":[1,2]}`, admin)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != CodeInvalidOrder {
		t.Errorf("partial order = %d %s, want 400 invalid_order", rec.Code, rec.Body.String())
	}

	// Removing one renumbers the rest contiguously.
	rec = h.do(http.MethodDelete, fmt.Sprintf("/api/meetings/%s/items/%d", m.Slug, order[1]), nil, admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove item = %d", rec.Code)
	}
	rec = h.do(http.MethodGet, "/api/meetings/"+m.Slug, nil, "")
	after := decode[meetingDetailDTO](t, rec)
	for i, it := range after.Items {
		if it.Position != i {
			t.Errorf("after removal, slot %d has position %d; the agenda would render with a gap", i, it.Position)
		}
	}
}

// An item id is global, so both note edits and removals must be scoped to the
// meeting in the path.
func TestItemOperationsAreScopedToTheirMeeting(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	week31 := h.createMeeting(admin, "2026-07-31")
	week38 := h.createMeeting(admin, "2026-09-18")

	foreignProblem := h.createProblem(admin, "cizí")
	rec := h.doJSON(http.MethodPost, "/api/meetings/"+week31.Slug+"/items",
		fmt.Sprintf(`{"problem_ids":[%d]}`, foreignProblem), admin)
	foreignItem := decode[[]meetingItemDTO](t, rec)[0]

	var ids []int64
	for i := 0; i < 3; i++ {
		ids = append(ids, h.createProblem(admin, fmt.Sprintf("vlastní %d", i)))
	}
	body, _ := json.Marshal(map[string][]int64{"problem_ids": ids})
	h.doJSON(http.MethodPost, "/api/meetings/"+week38.Slug+"/items", string(body), admin)

	// Editing week 31's item through week 38's URL must 404 ...
	rec = h.doJSON(http.MethodPatch,
		fmt.Sprintf("/api/meetings/%s/items/%d", week38.Slug, foreignItem.ID), `{"prep_note":"nope"}`, admin)
	if rec.Code != http.StatusNotFound {
		t.Errorf("cross-meeting PATCH = %d, want 404", rec.Code)
	}
	// ... and deleting it likewise, without renumbering the wrong agenda.
	rec = h.do(http.MethodDelete,
		fmt.Sprintf("/api/meetings/%s/items/%d", week38.Slug, foreignItem.ID), nil, admin)
	if rec.Code != http.StatusNotFound {
		t.Errorf("cross-meeting DELETE = %d, want 404", rec.Code)
	}

	rec = h.do(http.MethodGet, "/api/meetings/"+week31.Slug, nil, "")
	w31 := decode[meetingDetailDTO](t, rec)
	if len(w31.Items) != 1 || w31.Items[0].PrepNote != "" {
		t.Errorf("week 31 was modified by a request aimed at week 38: %+v", w31.Items)
	}
	rec = h.do(http.MethodGet, "/api/meetings/"+week38.Slug, nil, "")
	if w38 := decode[meetingDetailDTO](t, rec); len(w38.Items) != 3 {
		t.Errorf("week 38 has %d items, want 3", len(w38.Items))
	}
}

// Deleting a problem strips it from every agenda and renumbers each.
func TestDeleteProblemRenumbersAgendas(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	m := h.createMeeting(admin, "2026-09-18")

	var ids []int64
	for i := 0; i < 3; i++ {
		ids = append(ids, h.createProblem(admin, fmt.Sprintf("p%d", i)))
	}
	body, _ := json.Marshal(map[string][]int64{"problem_ids": ids})
	h.doJSON(http.MethodPost, "/api/meetings/"+m.Slug+"/items", string(body), admin)

	if rec := h.do(http.MethodDelete, fmt.Sprintf("/api/problems/%d", ids[0]), nil, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	rec := h.do(http.MethodGet, "/api/meetings/"+m.Slug, nil, "")
	got := decode[meetingDetailDTO](t, rec)
	if len(got.Items) != 2 {
		t.Fatalf("agenda has %d items, want 2", len(got.Items))
	}
	for i, it := range got.Items {
		if it.Position != i {
			t.Errorf("slot %d has position %d; the agenda would render as 1, 3", i, it.Position)
		}
	}
}

// Marking done twice must not rewrite done_at/done_by.
func TestDoneIsIdempotent(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	id := h.createProblem(admin, "x")

	rec := h.do(http.MethodPost, fmt.Sprintf("/api/problems/%d/done", id), nil, admin)
	first := decode[problemDetailDTO](t, rec)
	if !first.Done || first.DoneBy == nil || *first.DoneBy != "admin" {
		t.Fatalf("after done: %+v", first)
	}
	rec = h.do(http.MethodPost, fmt.Sprintf("/api/problems/%d/done", id), nil, admin)
	again := decode[problemDetailDTO](t, rec)
	if *again.DoneAt != *first.DoneAt {
		t.Errorf("a repeated done rewrote done_at: %v -> %v", *first.DoneAt, *again.DoneAt)
	}

	rec = h.do(http.MethodDelete, fmt.Sprintf("/api/problems/%d/done", id), nil, admin)
	reopened := decode[problemDetailDTO](t, rec)
	if reopened.Done || reopened.DoneAt != nil || reopened.DoneBy != nil {
		t.Errorf("after reopening: %+v", reopened)
	}
}

// ---------- uploads ----------

func multipartBody(t *testing.T, fields map[string]string, files map[string][]byte, ctype string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range files {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + escapeQuotes(name) + `"`}
		if ctype != "" {
			h["Content-Type"] = []string{ctype}
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// escapeQuotes mirrors what mime/multipart does when a browser sends a file
// name containing a quote or a backslash.
func escapeQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(s)
}

func (h *harness) postMultipart(path, token string, fields map[string]string, files map[string][]byte, ctype string) *httptest.ResponseRecorder {
	h.t.Helper()
	body, contentType := multipartBody(h.t, fields, files, ctype)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Cookie", SessionCookie+"="+token)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) countDir(dir string) int {
	h.t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		h.t.Fatal(err)
	}
	return len(entries)
}

// A problem and its files are created in one request.
func TestCreateProblemWithAttachments(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token,
		map[string]string{"title": "Konflikt se neuloží", "description": "popis"},
		map[string][]byte{"snimek.png": []byte("\x89PNG\r\n\x1a\nfake")}, "image/png")
	if rec.Code != http.StatusCreated {
		t.Fatalf("= %d %s", rec.Code, rec.Body.String())
	}
	got := decode[problemDetailDTO](t, rec)
	if got.Title != "Konflikt se neuloží" {
		t.Errorf("title = %q", got.Title)
	}
	if len(got.Attachments) != 1 || got.AttachmentCount != 1 {
		t.Fatalf("attachments = %d", len(got.Attachments))
	}
	a := got.Attachments[0]
	if a.Filename != "snimek.png" || a.ContentType != "image/png" {
		t.Errorf("attachment = %+v", a)
	}
	if a.URL != fmt.Sprintf("/api/attachments/%d/content", a.ID) {
		t.Errorf("url = %q; it must be root-relative so it resolves on the page's own origin", a.URL)
	}
	if h.countDir(h.cfg.AttachDir) != 1 {
		t.Errorf("the file did not reach the attachments directory")
	}
	if h.countDir(h.cfg.TmpDir) != 0 {
		t.Errorf("staging was left dirty")
	}
}

// Each of the three caps rejects the request whole: no problem, no row, no file.
func TestUploadCapsRejectWholeRequest(t *testing.T) {
	cases := []struct {
		name  string
		files map[string][]byte
	}{
		{"one file over the per-file cap", map[string][]byte{"big.bin": bytes.Repeat([]byte("x"), 2000)}},
		{"more files than allowed", map[string][]byte{
			"a.bin": []byte("a"), "b.bin": []byte("b"), "c.bin": []byte("c"),
		}},
		{"sum over the request cap", map[string][]byte{
			"a.bin": bytes.Repeat([]byte("a"), 1000),
			"b.bin": bytes.Repeat([]byte("b"), 1000),
			"c.bin": bytes.Repeat([]byte("c"), 1000),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			token := h.editor()
			rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"}, c.files, "")
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("= %d %s, want 413", rec.Code, rec.Body.String())
			}
			if code := errorCode(t, rec); code != CodeFileTooLarge {
				t.Errorf("code = %q, want %q", code, CodeFileTooLarge)
			}
			// Nothing may survive a rejected submit.
			list := h.do(http.MethodGet, "/api/problems", nil, "")
			if n := len(decode[[]problemDTO](t, list)); n != 0 {
				t.Errorf("%d problems were created by a rejected request", n)
			}
			if n := h.countDir(h.cfg.AttachDir); n != 0 {
				t.Errorf("%d files reached the attachments directory", n)
			}
			if n := h.countDir(h.cfg.TmpDir); n != 0 {
				t.Errorf("%d files were left in staging", n)
			}
		})
	}
}

// A validation failure after the files are staged must also leave nothing.
func TestRejectedValidationLeavesNoFiles(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token,
		map[string]string{"title": ""}, // rejected
		map[string][]byte{"a.png": []byte("data")}, "image/png")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("= %d %s, want 400", rec.Code, rec.Body.String())
	}
	if n := h.countDir(h.cfg.AttachDir); n != 0 {
		t.Errorf("%d files survived a rejected submit", n)
	}
	if n := h.countDir(h.cfg.TmpDir); n != 0 {
		t.Errorf("%d files were left in staging", n)
	}
}

// PATCH with multipart appends files without touching the existing ones.
func TestPatchAppendsAttachments(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"},
		map[string][]byte{"first.png": []byte("1")}, "image/png")
	id := decode[problemDetailDTO](t, rec).ID

	body, contentType := multipartBody(t, map[string]string{"description": "doplněno"},
		map[string][]byte{"second.png": []byte("2")}, "image/png")
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/problems/%d", id), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cookie", SessionCookie+"="+token)
	rec = httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("= %d %s", rec.Code, rec.Body.String())
	}
	got := decode[problemDetailDTO](t, rec)
	if len(got.Attachments) != 2 {
		t.Errorf("attachments = %d, want 2", len(got.Attachments))
	}
	if got.Description != "doplněno" || got.Title != "x" {
		t.Errorf("fields = %q / %q", got.Title, got.Description)
	}
}

// The submitted filename must never reach the filesystem.
func TestUploadIgnoresSubmittedPath(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"},
		map[string][]byte{`..\..\app.db`: []byte("payload")}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("= %d %s", rec.Code, rec.Body.String())
	}
	got := decode[problemDetailDTO](t, rec)
	if got.Attachments[0].Filename != "app.db" {
		t.Errorf("metadata filename = %q, want the last segment only", got.Attachments[0].Filename)
	}
	entries, _ := os.ReadDir(h.cfg.AttachDir)
	for _, e := range entries {
		if !storageNamePattern.MatchString(e.Name()) {
			t.Errorf("stored file %q is not <uuid>.<ext>", e.Name())
		}
	}
	// Nothing may have been written outside the attachments directory.
	if _, err := os.Stat(filepath.Join(h.cfg.DataDir, "app.db-traversed")); err == nil {
		t.Error("a file escaped the attachments directory")
	}
}

func TestAttachmentServing(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("d", 40))
	rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"},
		map[string][]byte{"řešení \"x\".png": png}, "image/png")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	att := decode[problemDetailDTO](t, rec).Attachments[0]
	url := fmt.Sprintf("/api/attachments/%d/content", att.ID)

	// An image is served inline, without nosniff.
	rec = h.do(http.MethodGet, url, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET content = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q", ct)
	}
	disp := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disp, "inline;") {
		t.Errorf("disposition = %q, want inline", disp)
	}
	// The accented name and the quote must be percent-encoded, and no raw
	// quote may survive in the ASCII fallback.
	if !strings.Contains(disp, "filename*=UTF-8''") || !strings.Contains(disp, "%C5%99") {
		t.Errorf("RFC 5987 encoding missing: %q", disp)
	}
	if strings.Count(disp, `"`) != 2 {
		t.Errorf("the ASCII fallback is not a single quoted string: %q", disp)
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("Accept-Ranges is missing; video scrubbing depends on it")
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("ETag is missing")
	}

	// ?download=1 forces the attachment disposition even for an image.
	rec = h.do(http.MethodGet, url+"?download=1", nil, "")
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "attachment;") {
		t.Errorf("?download=1 disposition = %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("a download must carry nosniff")
	}

	// A range request is answered with 206 and Content-Range.
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=0-9")
	r2 := httptest.NewRecorder()
	h.handler.ServeHTTP(r2, req)
	if r2.Code != http.StatusPartialContent {
		t.Fatalf("range = %d, want 206", r2.Code)
	}
	if cr := r2.Header().Get("Content-Range"); cr != fmt.Sprintf("bytes 0-9/%d", len(png)) {
		t.Errorf("Content-Range = %q", cr)
	}
	if r2.Body.Len() != 10 {
		t.Errorf("range body = %d bytes, want 10", r2.Body.Len())
	}

	// A range past the end is a 416 with the JSON envelope.
	req = httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=9999-")
	r3 := httptest.NewRecorder()
	h.handler.ServeHTTP(r3, req)
	if r3.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("out-of-range = %d, want 416", r3.Code)
	}
	if cr := r3.Header().Get("Content-Range"); cr != fmt.Sprintf("bytes */%d", len(png)) {
		t.Errorf("416 Content-Range = %q", cr)
	}
	if code := errorCode(t, r3); code != CodeRangeNotSatisfiable {
		t.Errorf("416 code = %q", code)
	}
}

// An SVG is a download, never a preview: nosniff cannot stop a document whose
// declared type genuinely is image/svg+xml.
func TestSVGIsAlwaysADownload(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"},
		map[string][]byte{"evil.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`)},
		"image/svg+xml")
	att := decode[problemDetailDTO](t, rec).Attachments[0]

	rec = h.do(http.MethodGet, fmt.Sprintf("/api/attachments/%d/content", att.ID), nil, "")
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "attachment;") {
		t.Errorf("SVG disposition = %q, want attachment", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("an SVG must carry nosniff")
	}
}

func TestDeleteAttachmentRemovesTheFile(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	rec := h.postMultipart("/api/problems", token, map[string]string{"title": "x"},
		map[string][]byte{"a.png": []byte("data")}, "image/png")
	att := decode[problemDetailDTO](t, rec).Attachments[0]
	if h.countDir(h.cfg.AttachDir) != 1 {
		t.Fatal("setup: expected one file")
	}

	rec = h.do(http.MethodDelete, fmt.Sprintf("/api/attachments/%d", att.ID), nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if n := h.countDir(h.cfg.AttachDir); n != 0 {
		t.Errorf("%d files left on disk after deleting the attachment", n)
	}
	if rec := h.do(http.MethodGet, fmt.Sprintf("/api/attachments/%d/content", att.ID), nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("content after delete = %d, want 404", rec.Code)
	}
}

// Deleting a problem removes its files too; SQLite's cascade cannot.
func TestDeleteProblemRemovesAttachmentFiles(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	rec := h.postMultipart("/api/problems", admin, map[string]string{"title": "x"},
		map[string][]byte{"a.png": []byte("1"), "b.png": []byte("2")}, "image/png")
	id := decode[problemDetailDTO](t, rec).ID
	if h.countDir(h.cfg.AttachDir) != 2 {
		t.Fatal("setup: expected two files")
	}
	if rec := h.do(http.MethodDelete, fmt.Sprintf("/api/problems/%d", id), nil, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if n := h.countDir(h.cfg.AttachDir); n != 0 {
		t.Errorf("%d attachment files were orphaned", n)
	}
}

// An unknown path under /api must answer with JSON, never the SPA's HTML.
func TestUnknownAPIPathReturnsJSON(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/does-not-exist", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("= %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
	if code := errorCode(t, rec); code != CodeNotFound {
		t.Errorf("code = %q", code)
	}
}

func TestNotFoundForUnknownRecords(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{
		"/api/problems/9999",
		"/api/meetings/2026-w12",
		"/api/attachments/9999/content",
	} {
		if rec := h.do(http.MethodGet, path, nil, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
	// A non-numeric id is a 404, not a 500.
	if rec := h.do(http.MethodGet, "/api/problems/abc", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/problems/abc = %d, want 404", rec.Code)
	}
}

// Empty collections must serialise as [] rather than null, or the frontend has
// to guard every map.
func TestEmptyCollectionsAreArrays(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	if body := strings.TrimSpace(h.do(http.MethodGet, "/api/problems", nil, "").Body.String()); body != "[]" {
		t.Errorf("empty bucket = %s, want []", body)
	}
	if body := strings.TrimSpace(h.do(http.MethodGet, "/api/meetings", nil, "").Body.String()); body != "[]" {
		t.Errorf("empty meeting list = %s, want []", body)
	}
	m := h.createMeeting(admin, "2026-09-18")
	detail := decode[map[string]any](t, h.do(http.MethodGet, "/api/meetings/"+m.Slug, nil, ""))
	if items, ok := detail["items"].([]any); !ok || items == nil {
		t.Errorf("items = %v, want an empty array", detail["items"])
	}
	id := h.createProblem(admin, "x")
	pd := decode[map[string]any](t, h.do(http.MethodGet, fmt.Sprintf("/api/problems/%d", id), nil, ""))
	for _, key := range []string{"attachments", "meetings"} {
		if v, ok := pd[key].([]any); !ok || v == nil {
			t.Errorf("%s = %v, want an empty array", key, pd[key])
		}
	}
}

func TestCORS(t *testing.T) {
	h := newHarness(t)
	// Empty CORS_ORIGINS emits no headers at all.
	plain := CORS(nil, h.handler)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://elsewhere:9999")
	rec := httptest.NewRecorder()
	plain.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no CORS should be emitted by default, got %q", got)
	}

	wrapped := CORS([]string{"http://vyvoj-srv:9999"}, h.handler)

	// An allowed origin is echoed with credentials and Vary.
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://vyvoj-srv:9999")
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://vyvoj-srv:9999" {
		t.Errorf("allow-origin = %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("credentials must be allowed, or the session cookie is useless")
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Error("Vary: Origin is missing; a cache could cross-serve responses")
	}

	// An unlisted origin gets nothing.
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://attacker:9999")
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unlisted origin was allowed: %q", got)
	}

	// The preflight must be answered here: ServeMux has no OPTIONS route and
	// would reply 405, which kills it.
	req = httptest.NewRequest(http.MethodOptions, "/api/problems", nil)
	req.Header.Set("Origin", "http://vyvoj-srv:9999")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight = %d, want 204", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
		t.Errorf("allowed methods = %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestMalformedJSON(t *testing.T) {
	h := newHarness(t)
	token := h.editor()
	for _, body := range []string{`{`, `{"title":}`, `{"title":"x"} trailing`, `{"nope":1}`} {
		rec := h.doJSON(http.MethodPost, "/api/problems", body, token)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q = %d, want 400", body, rec.Code)
		}
	}
}
