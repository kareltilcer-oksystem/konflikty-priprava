package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/timeutil"
)

// listMeetings returns meetings newest first.
//
// The default filter is one-sided: everything on or after today minus
// ARCHIVE_AFTER_DAYS. It is not a window — a future-dated meeting must never be
// filtered out, because the one being prepared for the coming week is always
// future-dated and is the row the list exists to surface.
func (a *API) listMeetings(w http.ResponseWriter, r *http.Request) {
	includeArchived, ok := queryBool(r, "include_archived")
	if !ok {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadBool)
		return
	}
	since := ""
	if includeArchived == nil || !*includeArchived {
		since = timeutil.FormatDate(timeutil.ArchiveCutoff(a.now(), a.cfg.ArchiveAfterDays))
	}

	meetings, err := a.store.ListMeetings(r.Context(), since)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]meetingDTO, 0, len(meetings))
	for _, m := range meetings {
		dto, err := toMeeting(m, a.now(), a.cfg.ArchiveAfterDays)
		if err != nil {
			slog.Error("render meeting", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
			return
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

type meetingCreateRequest struct {
	MeetingDate string  `json:"meeting_date"`
	Note        *string `json:"note"`
}

type meetingUpdateRequest struct {
	MeetingDate *string `json:"meeting_date"`
	Note        *string `json:"note"`
}

// createMeeting derives and freezes the slug from the date's ISO week.
func (a *API) createMeeting(w http.ResponseWriter, r *http.Request) {
	var req meetingCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}
	date, err := timeutil.ParseDate(req.MeetingDate)
	if err != nil {
		writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msgBadDate,
			map[string]string{"meeting_date": msgBadDate})
		return
	}
	note := ""
	if req.Note != nil {
		note = *req.Note
	}
	account, _ := accountFrom(r.Context())

	m, err := a.store.CreateMeeting(r.Context(), date, note, account.Username, a.now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	detail, err := a.meetingDetail(r, m.Slug)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/meetings/%s", m.Slug))
	writeJSON(w, http.StatusCreated, detail)
}

func (a *API) getMeeting(w http.ResponseWriter, r *http.Request) {
	detail, err := a.meetingDetail(r, r.PathValue("slug"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// updateMeeting changes the date or the general note.
//
// The slug is never regenerated: links shared earlier keep working, so after a
// date change /porada/2026-w38 can legitimately show Týden 39.
func (a *API) updateMeeting(w http.ResponseWriter, r *http.Request) {
	var req meetingUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}
	patch := store.MeetingPatch{Note: req.Note}
	if req.MeetingDate != nil {
		date, err := timeutil.ParseDate(*req.MeetingDate)
		if err != nil {
			writeErrorDetails(w, http.StatusBadRequest, CodeValidationFailed, msgBadDate,
				map[string]string{"meeting_date": msgBadDate})
			return
		}
		formatted := timeutil.FormatDate(date)
		patch.MeetingDate = &formatted
	}
	slugValue := r.PathValue("slug")
	if _, err := a.store.UpdateMeeting(r.Context(), slugValue, patch); err != nil {
		writeStoreError(w, err)
		return
	}
	detail, err := a.meetingDetail(r, slugValue)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// deleteMeeting removes the agenda and its notes; the problems stay in the
// bucket.
func (a *API) deleteMeeting(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteMeeting(r.Context(), r.PathValue("slug")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// meetingDetail assembles a meeting with its ordered agenda, each item carrying
// its problem and that problem's attachments, so the page renders in one round
// trip.
func (a *API) meetingDetail(r *http.Request, meetingSlug string) (meetingDetailDTO, error) {
	ctx := r.Context()
	m, err := a.store.GetMeetingBySlug(ctx, meetingSlug)
	if err != nil {
		return meetingDetailDTO{}, err
	}
	head, err := toMeeting(m, a.now(), a.cfg.ArchiveAfterDays)
	if err != nil {
		return meetingDetailDTO{}, err
	}
	items, err := a.store.ListItems(ctx, m.ID)
	if err != nil {
		return meetingDetailDTO{}, err
	}

	problemIDs := make([]int64, 0, len(items))
	for _, it := range items {
		problemIDs = append(problemIDs, it.ProblemID)
	}
	problems, err := a.store.ProblemsByIDs(ctx, problemIDs)
	if err != nil {
		return meetingDetailDTO{}, err
	}
	// One query for every item's files, rather than one per item.
	attachments, err := a.store.AttachmentsByProblems(ctx, problemIDs)
	if err != nil {
		return meetingDetailDTO{}, err
	}

	out := make([]meetingItemDTO, 0, len(items))
	for _, it := range items {
		p, ok := problems[it.ProblemID]
		if !ok {
			// A cascade would have removed the item along with the problem, so
			// this cannot happen; skip rather than render a hole.
			slog.Error("agenda item references a missing problem", "item", it.ID, "problem", it.ProblemID)
			continue
		}
		out = append(out, meetingItemDTO{
			ID: it.ID, MeetingID: it.MeetingID, ProblemID: it.ProblemID,
			Position: it.Position, PrepNote: it.PrepNote, ActionNote: it.ActionNote,
			Problem: toProblemWithAttachments(p, attachments[it.ProblemID]),
		})
	}
	return meetingDetailDTO{meetingDTO: head, Items: out}, nil
}
