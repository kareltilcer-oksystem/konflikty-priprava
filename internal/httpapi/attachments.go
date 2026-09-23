package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/uploads"
)

// storageNamePattern is what the uploads package generates. Re-checking it
// before opening the file is defence in depth: nothing should ever be able to
// put a path into that column, and if something did, this refuses to follow it.
var storageNamePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.[a-z0-9]+$`)

// readUpload parses a multipart request under the configured caps.
func (a *API) readUpload(w http.ResponseWriter, r *http.Request) (*uploads.Result, error) {
	return uploads.Read(w, r, a.cfg.TmpDir, uploads.Limits{
		MaxFileBytes:    a.cfg.MaxUploadBytes,
		MaxFiles:        a.cfg.MaxAttachmentsPerRequest,
		MaxRequestBytes: a.cfg.MaxRequestBytes,
	})
}

// writeUploadError turns a staging rejection into its Czech message.
//
// The three caps get three different messages on purpose: they fail for
// genuinely different reasons and the fix differs — remove one huge file,
// remove some files, or split the submit.
func writeUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, uploads.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge,
			"Soubor je větší než povolených 100 MB.")
	case errors.Is(err, uploads.ErrTooManyFiles):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge,
			"Najednou lze nahrát nejvýš 20 souborů.")
	case errors.Is(err, uploads.ErrBodyTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge,
			"Přílohy dohromady přesahují povolených 512 MB.")
	case errors.Is(err, uploads.ErrFieldTooLarge):
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgValidationFailed)
	case errors.Is(err, uploads.ErrFieldRepeated):
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgFieldRepeated)
	case errors.Is(err, uploads.ErrNotMultipart):
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgNotMultipart)
	default:
		slog.Error("read upload", "err", err)
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgValidationFailed)
	}
}

// uploadAttachment adds a single file to an existing problem.
func (a *API) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	problemID, ok := pathInt64(r, "problemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	account, _ := accountFrom(r.Context())

	staged, err := a.readUpload(w, r)
	if err != nil {
		writeUploadError(w, err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			staged.Rollback()
		}
	}()

	if len(staged.Files) != 1 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "Odešlete právě jeden soubor.")
		return
	}
	if err := staged.Commit(a.cfg.AttachDir); err != nil {
		slog.Error("commit upload", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}
	saved, err := a.store.AddAttachment(r.Context(), problemID, stagedAttachments(staged)[0], account.Username, a.now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	committed = true

	dto := toAttachment(saved)
	w.Header().Set("Location", dto.URL)
	writeJSON(w, http.StatusCreated, dto)
}

// deleteAttachment removes the metadata row and the file on disk.
func (a *API) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "attachmentId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	storageName, err := a.store.DeleteAttachment(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	a.removeFiles([]string{storageName})
	w.WriteHeader(http.StatusNoContent)
}

// attachmentContent streams an attachment's bytes.
func (a *API) attachmentContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "attachmentId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	att, err := a.store.GetAttachment(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !storageNamePattern.MatchString(att.StorageName) {
		slog.Error("attachment has an unexpected storage name", "id", id, "name", att.StorageName)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}

	path := filepath.Join(a.cfg.AttachDir, att.StorageName)
	f, err := os.Open(path)
	if err != nil {
		// The row survived its file: report it as missing rather than 500, but
		// log it, because it means something bypassed the delete path.
		slog.Warn("attachment file is missing", "id", id, "file", att.StorageName, "err", err)
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		slog.Error("stat attachment", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}

	// image/* and video/* render in place; everything else, SVG included, is a
	// download with nosniff, so an uploaded .html or .svg can never execute
	// inside the app's own origin.
	inline := uploads.IsInline(att.ContentType) && r.URL.Query().Get("download") != "1"

	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", uploads.ContentDisposition(inline, att.Filename))
	if !inline {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	// The bytes behind an id never change, so this can be cached hard.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", fmt.Sprintf(`"%d-%d"`, att.ID, info.Size()))

	// ServeContent answers an unsatisfiable range with plain text; the contract
	// documents a JSON body, so that one case is handled here first.
	if rangeUnsatisfiable(r.Header.Get("Range"), info.Size()) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size()))
		writeError(w, http.StatusRequestedRangeNotSatisfiable, CodeRangeNotSatisfiable, msgRangeNotSatisfiable)
		return
	}

	// The empty name is deliberate: a non-empty one would send ServeContent
	// into mime.TypeByExtension, which on Windows reads the registry. The
	// Content-Type is already set above. ServeContent handles Accept-Ranges,
	// 206, Content-Range and If-Range — the whole reason attachments are files
	// on disk rather than BLOBs.
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// rangeUnsatisfiable reports whether every range in the header lies past the
// end of the file. A malformed header is left to ServeContent, which ignores it
// and serves the whole body, as RFC 7233 requires.
func rangeUnsatisfiable(header string, size int64) bool {
	const prefix = "bytes="
	if header == "" || !strings.HasPrefix(header, prefix) {
		return false
	}
	specs := strings.Split(strings.TrimPrefix(header, prefix), ",")
	sawOne := false
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		start, end, found := strings.Cut(spec, "-")
		if !found {
			return false // malformed: not ours to reject
		}
		start, end = strings.TrimSpace(start), strings.TrimSpace(end)
		if start == "" {
			// A suffix range ("-500") is unsatisfiable only when it asks for
			// nothing at all.
			n, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return false
			}
			sawOne = true
			if n != 0 {
				return false
			}
			continue
		}
		from, err := strconv.ParseInt(start, 10, 64)
		if err != nil {
			return false
		}
		sawOne = true
		if from < size {
			return false // at least one range is satisfiable
		}
	}
	return sawOne
}
