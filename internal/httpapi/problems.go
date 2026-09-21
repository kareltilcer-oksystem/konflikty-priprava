package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/search"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/uploads"
)

// maxTitleLen mirrors the contract's maxLength on Problem.title.
const maxTitleLen = 200

func (a *API) listProblems(w http.ResponseWriter, r *http.Request) {
	done, ok := queryBool(r, "done")
	if !ok {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadBool)
		return
	}
	scheduled, ok := queryBool(r, "scheduled")
	if !ok {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadBool)
		return
	}
	sort := store.SortKey(r.URL.Query().Get("sort"))
	if sort == "" {
		sort = store.SortCreatedDesc
	}
	if !sort.Valid() {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadSort)
		return
	}

	problems, err := a.store.ListProblems(r.Context(), store.ProblemFilter{
		Done: done, Scheduled: scheduled, Sort: sort,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// The text filter runs here rather than in SQL: SQLite folds ASCII only,
	// so it cannot match `reseni` against `řešení`. The candidate set is a few
	// hundred rows by design.
	problems = search.Filter(problems, r.URL.Query().Get("q"), func(p store.Problem) []string {
		return []string{p.Title, p.Description}
	})

	writeJSON(w, http.StatusOK, toProblems(problems))
}

func (a *API) getProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "problemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	detail, err := a.problemDetail(r, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// problemDetail assembles the full ProblemDetail: the problem, its files and
// its cross-meeting timeline.
func (a *API) problemDetail(r *http.Request, id int64) (problemDetailDTO, error) {
	ctx := r.Context()
	p, err := a.store.GetProblem(ctx, id)
	if err != nil {
		return problemDetailDTO{}, err
	}
	atts, err := a.store.AttachmentsByProblem(ctx, id)
	if err != nil {
		return problemDetailDTO{}, err
	}
	refs, err := a.store.ProblemMeetingRefs(ctx, id)
	if err != nil {
		return problemDetailDTO{}, err
	}
	timeline := make([]problemMeetingRefDTO, 0, len(refs))
	for _, ref := range refs {
		dto, err := toProblemMeetingRef(ref)
		if err != nil {
			return problemDetailDTO{}, err
		}
		timeline = append(timeline, dto)
	}
	return problemDetailDTO{
		problemWithAttachmentsDTO: toProblemWithAttachments(p, atts),
		Meetings:                  timeline,
	}, nil
}

type problemCreateRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Link        *string `json:"link"`
}

type problemUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Link        *string `json:"link"`
}

// createProblem accepts both shapes: JSON for a problem with no files, and
// multipart for a problem and its attachments in one atomic request.
func (a *API) createProblem(w http.ResponseWriter, r *http.Request) {
	account, _ := accountFrom(r.Context())

	var (
		input   store.ProblemInput
		staged  *uploads.Result
		details map[string]string
	)

	if isJSONRequest(r) {
		var req problemCreateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
			return
		}
		input.Title = strings.TrimSpace(req.Title)
		if req.Description != nil {
			input.Description = *req.Description
		}
		if req.Link != nil {
			input.Link = strings.TrimSpace(*req.Link)
		}
	} else {
		var err error
		staged, err = a.readUpload(w, r)
		if err != nil {
			writeUploadError(w, err)
			return
		}
		// Rolled back unless the whole request succeeds, so a rejected submit
		// leaves no orphan bytes on disk.
		committed := false
		defer func() {
			if !committed {
				staged.Rollback()
			}
		}()
		input.Title = strings.TrimSpace(staged.Field("title"))
		input.Description = staged.Field("description")
		input.Link = strings.TrimSpace(staged.Field("link"))

		if msg, field, bad := validateProblem(input.Title, input.Link); bad {
			details = map[string]string{field: msg}
			writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msg, details)
			return
		}
		if err := staged.Commit(a.cfg.AttachDir); err != nil {
			slog.Error("commit uploads", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
			return
		}
		id, err := a.store.CreateProblem(r.Context(), input, stagedAttachments(staged), account.Username, a.now())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		committed = true
		a.writeCreatedProblem(w, r, id)
		return
	}

	if msg, field, bad := validateProblem(input.Title, input.Link); bad {
		writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msg, map[string]string{field: msg})
		return
	}
	id, err := a.store.CreateProblem(r.Context(), input, nil, account.Username, a.now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	a.writeCreatedProblem(w, r, id)
}

func (a *API) writeCreatedProblem(w http.ResponseWriter, r *http.Request, id int64) {
	detail, err := a.problemDetail(r, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/problems/%d", id))
	writeJSON(w, http.StatusCreated, detail)
}

// updateProblem mirrors createProblem: JSON edits the fields alone, multipart
// edits them and appends newly pasted attachments in the same atomic request.
//
// done is deliberately not settable here — it has its own admin-only endpoint.
func (a *API) updateProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "problemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	account, _ := accountFrom(r.Context())

	var patch store.ProblemPatch
	var attachments []store.NewAttachment

	if isJSONRequest(r) {
		var req problemUpdateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
			return
		}
		patch = store.ProblemPatch{Title: req.Title, Description: req.Description, Link: req.Link}
		if patch.Title != nil {
			trimmed := strings.TrimSpace(*patch.Title)
			patch.Title = &trimmed
		}
		if patch.Link != nil {
			trimmed := strings.TrimSpace(*patch.Link)
			patch.Link = &trimmed
		}
		if msg, field, bad := validatePatch(patch); bad {
			writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msg, map[string]string{field: msg})
			return
		}
		if err := a.store.UpdateProblem(r.Context(), id, patch, nil, account.Username, a.now()); err != nil {
			writeStoreError(w, err)
			return
		}
		a.writeProblemDetail(w, r, id)
		return
	}

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

	// A key's presence is the multipart equivalent of a non-nil pointer: it
	// distinguishes "clear the link" from "leave it alone". An edit that only
	// adds screenshots sends no field parts at all, which is valid.
	if staged.Has("title") {
		v := strings.TrimSpace(staged.Field("title"))
		patch.Title = &v
	}
	if staged.Has("description") {
		v := staged.Field("description")
		patch.Description = &v
	}
	if staged.Has("link") {
		v := strings.TrimSpace(staged.Field("link"))
		patch.Link = &v
	}
	if msg, field, bad := validatePatch(patch); bad {
		writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msg, map[string]string{field: msg})
		return
	}
	if err := staged.Commit(a.cfg.AttachDir); err != nil {
		slog.Error("commit uploads", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}
	attachments = stagedAttachments(staged)
	if err := a.store.UpdateProblem(r.Context(), id, patch, attachments, account.Username, a.now()); err != nil {
		writeStoreError(w, err)
		return
	}
	committed = true
	a.writeProblemDetail(w, r, id)
}

func (a *API) writeProblemDetail(w http.ResponseWriter, r *http.Request, id int64) {
	detail, err := a.problemDetail(r, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// deleteProblem removes a problem, its files and every agenda entry for it.
func (a *API) deleteProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "problemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	storageNames, err := a.store.DeleteProblem(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// The files go only after the transaction has committed. SQLite and the
	// filesystem share no transaction, so the choice is between orphan bytes
	// (harmless, sweepable) and rows pointing at missing files (a broken
	// download). This order picks the former.
	a.removeFiles(storageNames)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) markDone(w http.ResponseWriter, r *http.Request)    { a.setDone(w, r, true) }
func (a *API) markNotDone(w http.ResponseWriter, r *http.Request) { a.setDone(w, r, false) }

func (a *API) setDone(w http.ResponseWriter, r *http.Request, done bool) {
	id, ok := pathInt64(r, "problemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	account, _ := accountFrom(r.Context())
	if err := a.store.SetProblemDone(r.Context(), id, done, account.Username, a.now()); err != nil {
		writeStoreError(w, err)
		return
	}
	a.writeProblemDetail(w, r, id)
}

// removeFiles unlinks committed attachment bytes, logging what it cannot.
func (a *API) removeFiles(storageNames []string) {
	for _, name := range storageNames {
		if name == "" {
			continue
		}
		path := filepath.Join(a.cfg.AttachDir, filepath.Base(name))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Error("remove attachment file", "file", name, "err", err)
		}
	}
}

func stagedAttachments(res *uploads.Result) []store.NewAttachment {
	out := make([]store.NewAttachment, 0, len(res.Files))
	for _, f := range res.Files {
		out = append(out, store.NewAttachment{
			Filename:    f.Filename,
			ContentType: f.ContentType,
			SizeBytes:   f.SizeBytes,
			StorageName: f.StorageName,
		})
	}
	return out
}

// validateProblem checks the fields a create must get right.
func validateProblem(title, link string) (msg, field string, bad bool) {
	if title == "" {
		return msgTitleRequired, "title", true
	}
	if len([]rune(title)) > maxTitleLen {
		return msgTitleTooLong, "title", true
	}
	if !validLink(link) {
		return msgLinkScheme, "link", true
	}
	return "", "", false
}

func validatePatch(p store.ProblemPatch) (msg, field string, bad bool) {
	if p.Title != nil {
		if *p.Title == "" {
			return msgTitleRequired, "title", true
		}
		if len([]rune(*p.Title)) > maxTitleLen {
			return msgTitleTooLong, "title", true
		}
	}
	if p.Link != nil && !validLink(*p.Link) {
		return msgLinkScheme, "link", true
	}
	return "", "", false
}

// validLink accepts http(s) URLs and the empty string, and nothing else.
//
// This value ends up as an href on a page every anonymous visitor can open, so
// javascript: and data: must never reach it. The empty string is the documented
// "no link" value, which is why the check is not a URL parse.
func validLink(link string) bool {
	if link == "" {
		return true
	}
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
