package uploads

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var storageNamePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.[a-z0-9]+$`)

type filePart struct {
	field    string
	filename string
	ctype    string
	body     []byte
}

// buildRequest assembles a multipart request the way the browser form does.
func buildRequest(t *testing.T, fields map[string]string, files []filePart) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		h := make(map[string][]string)
		disp := `form-data; name="` + f.field + `"; filename="` + f.filename + `"`
		h["Content-Disposition"] = []string{disp}
		if f.ctype != "" {
			h["Content-Type"] = []string{f.ctype}
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/problems", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.ContentLength = int64(buf.Len())
	return req
}

func dirs(t *testing.T) (tmpDir, finalDir string) {
	t.Helper()
	root := t.TempDir()
	tmpDir = filepath.Join(root, "tmp")
	finalDir = filepath.Join(root, "attachments")
	for _, d := range []string{tmpDir, finalDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return tmpDir, finalDir
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// Tiny caps keep these tests in milliseconds while exercising exactly the same
// code paths as 100 MB / 20 files / 512 MB.
var tiny = Limits{MaxFileBytes: 1024, MaxFiles: 2, MaxRequestBytes: 4096}

func TestReadHappyPath(t *testing.T) {
	tmpDir, finalDir := dirs(t)
	req := buildRequest(t,
		map[string]string{"title": "Konflikt se neuloží", "description": "popis", "link": ""},
		[]filePart{
			{field: "file", filename: "snimek.png", ctype: "image/png", body: []byte("\x89PNG\r\n\x1a\nfake")},
			{field: "file", filename: "log.txt", ctype: "text/plain", body: []byte("hello")},
		})

	res, err := Read(httptest.NewRecorder(), req, tmpDir, tiny)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if res.Field("title") != "Konflikt se neuloží" {
		t.Errorf("title = %q", res.Field("title"))
	}
	// A sent-but-empty field must be distinguishable from an absent one.
	if !res.Has("link") {
		t.Error("link was sent as an empty string and should register as present")
	}
	if res.Has("nope") {
		t.Error("an unsent field should not register as present")
	}
	if len(res.Files) != 2 {
		t.Fatalf("staged %d files, want 2", len(res.Files))
	}
	if countFiles(t, tmpDir) != 2 || countFiles(t, finalDir) != 0 {
		t.Fatalf("before commit: tmp=%d final=%d, want 2 and 0", countFiles(t, tmpDir), countFiles(t, finalDir))
	}

	if err := res.Commit(finalDir); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if countFiles(t, tmpDir) != 0 || countFiles(t, finalDir) != 2 {
		t.Errorf("after commit: tmp=%d final=%d, want 0 and 2", countFiles(t, tmpDir), countFiles(t, finalDir))
	}
	for _, f := range res.Files {
		if _, err := os.Stat(filepath.Join(finalDir, f.StorageName)); err != nil {
			t.Errorf("committed file %s is missing: %v", f.StorageName, err)
		}
	}
}

// Each of the three caps must reject the whole request and leave nothing behind
// — no staged file, no committed file.
func TestReadRejectsEachCapAndStagesNothing(t *testing.T) {
	cases := []struct {
		name  string
		files []filePart
		want  error
	}{
		{
			name:  "one file over the per-file cap",
			files: []filePart{{field: "file", filename: "big.bin", body: bytes.Repeat([]byte("x"), 2000)}},
			want:  ErrFileTooLarge,
		},
		{
			name: "more files than allowed",
			files: []filePart{
				{field: "file", filename: "a.bin", body: []byte("a")},
				{field: "file", filename: "b.bin", body: []byte("b")},
				{field: "file", filename: "c.bin", body: []byte("c")},
			},
			want: ErrTooManyFiles,
		},
		{
			name: "no single file too large, but the sum is",
			files: []filePart{
				{field: "file", filename: "a.bin", body: bytes.Repeat([]byte("a"), 1000)},
				{field: "file", filename: "b.bin", body: bytes.Repeat([]byte("b"), 1000)},
			},
			want: ErrBodyTooLarge,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpDir, finalDir := dirs(t)
			lim := tiny
			if c.want == ErrBodyTooLarge {
				lim.MaxRequestBytes = 1500 // below the two 1000-byte parts together
			}
			req := buildRequest(t, map[string]string{"title": "x"}, c.files)
			res, err := Read(httptest.NewRecorder(), req, tmpDir, lim)
			if !errors.Is(err, c.want) {
				t.Fatalf("Read = %v, want %v", err, c.want)
			}
			if res != nil {
				t.Error("a rejected Read must not return a result")
			}
			if n := countFiles(t, tmpDir); n != 0 {
				t.Errorf("%d files left in staging; a rejected request must leave nothing", n)
			}
			if n := countFiles(t, finalDir); n != 0 {
				t.Errorf("%d files reached the attachments directory", n)
			}
		})
	}
}

// Rollback after Commit has to remove the renamed files too: the handler defers
// it and only clears it once the database transaction has succeeded.
func TestRollbackAfterCommitRemovesCommittedFiles(t *testing.T) {
	tmpDir, finalDir := dirs(t)
	req := buildRequest(t, map[string]string{"title": "x"}, []filePart{
		{field: "file", filename: "a.png", ctype: "image/png", body: []byte("a")},
		{field: "file", filename: "b.png", ctype: "image/png", body: []byte("b")},
	})
	res, err := Read(httptest.NewRecorder(), req, tmpDir, tiny)
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Commit(finalDir); err != nil {
		t.Fatal(err)
	}
	if countFiles(t, finalDir) != 2 {
		t.Fatalf("expected 2 committed files")
	}

	res.Rollback() // simulates the database insert failing

	if n := countFiles(t, finalDir); n != 0 {
		t.Errorf("%d files survived the rollback; they would be orphans", n)
	}
	res.Rollback() // must be idempotent
}

// The submitted file name must never influence the path written to.
func TestStorageNameIgnoresTheSubmittedFilename(t *testing.T) {
	tmpDir, _ := dirs(t)
	for _, evil := range []string{
		`..\..\..\app.db`,
		"../../../app.db",
		"x.a/../../../app.db",
		`C:\Windows\System32\drivers\etc\hosts`,
	} {
		req := buildRequest(t, nil, []filePart{{field: "file", filename: evil, body: []byte("payload")}})
		res, err := Read(httptest.NewRecorder(), req, tmpDir, tiny)
		if err != nil {
			t.Fatalf("Read(%q): %v", evil, err)
		}
		f := res.Files[0]
		if strings.ContainsAny(f.StorageName, `/\`) || strings.Contains(f.StorageName, "..") {
			t.Errorf("storage name %q derived from %q contains a path", f.StorageName, evil)
		}
		// The whole name is uuid + an extension from the resolved content
		// type; nothing the client sent can appear in it.
		if !storageNamePattern.MatchString(f.StorageName) {
			t.Errorf("storage name %q is not <uuid>.<ext>", f.StorageName)
		}
		// The metadata copy keeps only the last segment.
		if strings.ContainsAny(f.Filename, `/\`) {
			t.Errorf("metadata filename %q still contains a path", f.Filename)
		}
		// Everything staged must sit directly in tmpDir.
		if got := filepath.Dir(filepath.Join(tmpDir, f.StorageName)); got != filepath.Clean(tmpDir) {
			t.Errorf("file would be written to %q, outside %q", got, tmpDir)
		}
		res.Rollback()
	}
}

func TestResolveContentType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 16))
	cases := []struct {
		name     string
		declared string
		head     []byte
		want     string
	}{
		{"declared type wins", "image/png", nil, "image/png"},
		{"parameters are stripped", "text/plain; charset=utf-8", nil, "text/plain"},
		{"case is normalised", "IMAGE/PNG", nil, "image/png"},
		{"octet-stream falls back to sniffing", DefaultContentType, png, "image/png"},
		{"missing type falls back to sniffing", "", png, "image/png"},
		{"nothing to go on", "", nil, DefaultContentType},
	}
	for _, c := range cases {
		if got := ResolveContentType(c.declared, c.head); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExtFor(t *testing.T) {
	cases := map[string]string{
		"image/png":                "png",
		"image/jpeg":               "jpg",
		"video/mp4":                "mp4",
		"image/svg+xml":            "svg",
		"text/plain":               "txt",
		"application/pdf":          "pdf",
		"application/octet-stream": "bin",
		"application/x-made-up":    "bin",
		"":                         "bin",
	}
	for in, want := range cases {
		if got := ExtFor(in); got != want {
			t.Errorf("ExtFor(%q) = %q, want %q", in, got, want)
		}
	}
}

// SVG must not take the inline branch: nosniff cannot stop a document whose
// declared type genuinely is image/svg+xml.
func TestIsInline(t *testing.T) {
	inline := []string{"image/png", "image/jpeg", "image/webp", "video/mp4", "video/webm", "IMAGE/PNG"}
	download := []string{
		"image/svg+xml", "image/svg", "image/svg+xml; charset=utf-8",
		"text/html", "application/pdf", "application/octet-stream", "text/plain", "",
	}
	for _, ct := range inline {
		if !IsInline(ct) {
			t.Errorf("%q should be served inline", ct)
		}
	}
	for _, ct := range download {
		if IsInline(ct) {
			t.Errorf("%q must be served as a download", ct)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"snimek.png", "snimek.png"},
		{`..\..\app.db`, "app.db"},
		{"../../app.db", "app.db"},
		{"/etc/passwd", "passwd"},
		{"řešení.png", "řešení.png"},
		{"with\r\nnewline.txt", "withnewline.txt"},
		{"tab\there.txt", "tabhere.txt"},
		{"", FallbackFilename},
		{"   ", FallbackFilename},
		{"..", FallbackFilename},
		{".", FallbackFilename},
	}
	for _, c := range cases {
		if got := SanitizeFilename(c.in); got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := SanitizeFilename(strings.Repeat("a", 400) + ".png"); len(got) > MaxFilenameLen {
		t.Errorf("a long name was not truncated: %d bytes", len(got))
	}
}

// A quote or a CRLF in the file name must not be able to break the header.
func TestContentDisposition(t *testing.T) {
	got := ContentDisposition(false, `řešení "x".png`)
	if !strings.HasPrefix(got, "attachment; ") {
		t.Errorf("disposition = %q, want an attachment", got)
	}
	if strings.Contains(got, `"x"`) {
		t.Errorf("the ASCII fallback kept a raw quote: %q", got)
	}
	if !strings.Contains(got, "filename*=UTF-8''") {
		t.Errorf("no RFC 5987 form: %q", got)
	}
	// ř is C5 99 in UTF-8, and the quote must be percent-encoded.
	if !strings.Contains(got, "%C5%99") || !strings.Contains(got, "%22") {
		t.Errorf("encoding is wrong: %q", got)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("header contains a line break: %q", got)
	}

	if inline := ContentDisposition(true, "snimek.png"); !strings.HasPrefix(inline, "inline; ") {
		t.Errorf("inline disposition = %q", inline)
	}
	// A name with nothing ASCII left must still produce a usable fallback.
	if got := ContentDisposition(false, "řešení.png"); !strings.Contains(got, `filename="`) {
		t.Errorf("missing the ASCII fallback: %q", got)
	}
}

func TestReadRejectsNonMultipart(t *testing.T) {
	tmpDir, _ := dirs(t)
	req := httptest.NewRequest(http.MethodPost, "/api/problems", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	if _, err := Read(httptest.NewRecorder(), req, tmpDir, tiny); !errors.Is(err, ErrNotMultipart) {
		t.Errorf("Read = %v, want ErrNotMultipart", err)
	}
}

// An edit that only adds files sends no field parts at all; that is valid.
func TestReadAcceptsFilesWithoutFields(t *testing.T) {
	tmpDir, _ := dirs(t)
	req := buildRequest(t, nil, []filePart{{field: "file", filename: "a.png", ctype: "image/png", body: []byte("a")}})
	res, err := Read(httptest.NewRecorder(), req, tmpDir, tiny)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(res.Files) != 1 || len(res.Fields) != 0 {
		t.Errorf("got %d files and %d fields, want 1 and 0", len(res.Files), len(res.Fields))
	}
	res.Rollback()
}

func TestNewUUIDIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		id, err := newUUID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 36 || seen[id] {
			t.Fatalf("bad or duplicate uuid %q", id)
		}
		seen[id] = true
	}
}
