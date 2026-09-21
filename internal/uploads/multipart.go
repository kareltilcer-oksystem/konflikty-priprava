// Package uploads stages multipart uploads on disk so a problem and its files
// are created in one atomic step.
//
// Nothing here uses Request.ParseMultipartForm. That call buffers part of the
// body in memory and spills the rest into os.TempDir() with a lifecycle we do
// not control, which neither the ~30 MB memory budget nor the all-or-nothing
// guarantee can live with. The body is streamed part by part instead, so peak
// memory is one copy buffer regardless of a 512 MB request.
package uploads

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Upload limits. All three are needed: the per-file cap alone bounds nothing,
// because an unlimited number of 99 MB parts is still a multi-gigabyte body
// that the atomicity guarantee forces the server to stage before it commits.
type Limits struct {
	MaxFileBytes    int64
	MaxFiles        int
	MaxRequestBytes int64
}

// Rejections. Each maps onto a distinct message in the UI, because the three
// caps fail for genuinely different reasons and the fix differs.
var (
	ErrNotMultipart  = errors.New("request is not multipart/form-data")
	ErrFileTooLarge  = errors.New("a file exceeds the per-file limit")
	ErrTooManyFiles  = errors.New("too many files in one request")
	ErrBodyTooLarge  = errors.New("the whole request exceeds the size limit")
	ErrFieldTooLarge = errors.New("a text field is too large")
	ErrMalformed     = errors.New("malformed multipart body")
)

// maxFieldBytes bounds a single non-file part. Titles and notes are small; a
// megabyte is generous and stops a text field being used as an upload channel.
const maxFieldBytes = 1 << 20

// Staged is one uploaded file whose bytes are already on disk but not yet in
// their final location.
type Staged struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	StorageName string

	tmpPath   string
	finalPath string
}

// Result is a parsed multipart request.
type Result struct {
	// Fields holds the non-file parts. A key's presence means the client sent
	// that field, which is the multipart equivalent of a non-nil pointer in the
	// JSON shape: it distinguishes "clear the link" from "leave it alone".
	Fields map[string]string
	Files  []Staged
}

// Has reports whether the client sent a field at all.
func (r *Result) Has(name string) bool {
	_, ok := r.Fields[name]
	return ok
}

// Field returns a field's value, or the empty string when it was not sent.
func (r *Result) Field(name string) string { return r.Fields[name] }

// Read consumes a multipart request, writing every file part into tmpDir.
//
// On any error it cleans up after itself and returns nothing staged, so the
// caller never has to roll back a failed Read. On success the caller must
// either Commit or Rollback.
func Read(w http.ResponseWriter, r *http.Request, tmpDir string, lim Limits) (*Result, error) {
	if !isMultipart(r) {
		return nil, ErrNotMultipart
	}
	// A declared length over the cap is refused before a single byte is read.
	if lim.MaxRequestBytes > 0 && r.ContentLength > lim.MaxRequestBytes {
		return nil, ErrBodyTooLarge
	}
	if lim.MaxRequestBytes > 0 {
		// Passing w matters: it marks the connection for close, so a client
		// still uploading cannot poison keep-alive.
		r.Body = http.MaxBytesReader(w, r.Body, lim.MaxRequestBytes)
	}

	mr, err := r.MultipartReader()
	if err != nil {
		return nil, ErrNotMultipart
	}

	res := &Result{Fields: map[string]string{}}
	ok := false
	defer func() {
		if !ok {
			res.Rollback()
		}
	}()

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, classify(err)
		}
		if part.FileName() == "" && part.FormName() != "file" {
			if err := readField(res, part); err != nil {
				part.Close()
				return nil, err
			}
			part.Close()
			continue
		}
		// The count is checked before any of this part's bytes are written.
		if lim.MaxFiles > 0 && len(res.Files) >= lim.MaxFiles {
			part.Close()
			return nil, ErrTooManyFiles
		}
		staged, err := stagePart(part, tmpDir, lim.MaxFileBytes)
		part.Close()
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, staged)
	}

	ok = true
	return res, nil
}

func isMultipart(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return false
	}
	base, _, err := mime.ParseMediaType(ct)
	return err == nil && strings.HasPrefix(base, "multipart/")
}

// classify turns a read error into the rejection it represents. The overflow is
// detected through MaxBytesError, never by matching the message text.
func classify(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return ErrBodyTooLarge
	}
	if errors.Is(err, ErrBodyTooLarge) || errors.Is(err, ErrFileTooLarge) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrMalformed, err)
}

func readField(res *Result, part *multipart.Part) error {
	buf, err := io.ReadAll(io.LimitReader(part, maxFieldBytes+1))
	if err != nil {
		return classify(err)
	}
	if int64(len(buf)) > maxFieldBytes {
		return ErrFieldTooLarge
	}
	res.Fields[part.FormName()] = string(buf)
	return nil
}

// stagePart streams one file part to disk under a generated name.
func stagePart(part *multipart.Part, tmpDir string, maxFileBytes int64) (Staged, error) {
	// Peek before deciding the type: the sniff needs the first bytes, and
	// bufio lets us look without consuming them.
	br := bufio.NewReaderSize(part, 512)
	head, err := br.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return Staged{}, classify(err)
	}
	contentType := ResolveContentType(part.Header.Get("Content-Type"), head)

	storageName, err := StorageName(contentType)
	if err != nil {
		return Staged{}, err
	}
	tmpPath := filepath.Join(tmpDir, storageName)
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return Staged{}, fmt.Errorf("stage upload: %w", err)
	}

	// The +1 reveals an over-limit file without reading the rest of it.
	limit := maxFileBytes
	if limit > 0 {
		limit++
	} else {
		limit = math.MaxInt64
	}
	n, copyErr := io.Copy(f, io.LimitReader(br, limit))
	closeErr := f.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return Staged{}, classify(copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return Staged{}, fmt.Errorf("stage upload: %w", closeErr)
	}
	if maxFileBytes > 0 && n > maxFileBytes {
		_ = os.Remove(tmpPath)
		return Staged{}, ErrFileTooLarge
	}

	return Staged{
		Filename:    SanitizeFilename(part.FileName()),
		ContentType: contentType,
		SizeBytes:   n,
		StorageName: storageName,
		tmpPath:     tmpPath,
	}, nil
}

// Commit moves every staged file into its final directory.
//
// It runs before the database transaction. Rename and insert have identical
// crash windows either way round, but doing the rename first keeps the store
// API free of commit callbacks, and the only residue a crash can leave is an
// orphan file that no row references — never a row pointing at a file that is
// not there, which would be a broken download.
func (r *Result) Commit(finalDir string) error {
	for i := range r.Files {
		f := &r.Files[i]
		dst := filepath.Join(finalDir, f.StorageName)
		if err := os.Rename(f.tmpPath, dst); err != nil {
			return fmt.Errorf("store upload %q: %w", f.Filename, err)
		}
		f.finalPath = dst
	}
	return nil
}

// Rollback removes everything this request staged. It is idempotent and works
// both before and after Commit, so the handler can defer it unconditionally and
// clear it only once the database transaction has succeeded.
func (r *Result) Rollback() {
	if r == nil {
		return
	}
	for i := range r.Files {
		f := &r.Files[i]
		if f.finalPath != "" {
			_ = os.Remove(f.finalPath)
			f.finalPath = ""
		}
		if f.tmpPath != "" {
			_ = os.Remove(f.tmpPath)
			f.tmpPath = ""
		}
	}
}
