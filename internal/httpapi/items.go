package httpapi

import (
	"net/http"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

type addItemsRequest struct {
	ProblemIDs []int64 `json:"problem_ids"`
}

type reorderRequest struct {
	ItemIDs []int64 `json:"item_ids"`
}

type itemUpdateRequest struct {
	PrepNote   *string `json:"prep_note"`
	ActionNote *string `json:"action_note"`
}

// addItems puts one or more problems on the agenda, appended in the given
// order. The picker sends several at once.
func (a *API) addItems(w http.ResponseWriter, r *http.Request) {
	var req addItemsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}
	if len(req.ProblemIDs) == 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgProblemIDsRequired)
		return
	}
	// A repeated id within one request is a 400; a problem already on the
	// agenda is the separate 409 the store raises.
	seen := make(map[int64]bool, len(req.ProblemIDs))
	for _, id := range req.ProblemIDs {
		if id < 1 {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, msgUnknownProblem)
			return
		}
		if seen[id] {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, msgDuplicateProblem)
			return
		}
		seen[id] = true
	}

	meetingSlug := r.PathValue("slug")
	created, err := a.store.AddItems(r.Context(), meetingSlug, req.ProblemIDs, a.now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	dtos, err := a.itemDTOs(r, created)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dtos)
}

// reorderItems rewrites the whole agenda order. This is what drag-and-drop
// calls on drop.
func (a *API) reorderItems(w http.ResponseWriter, r *http.Request) {
	var req reorderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}
	items, err := a.store.ReorderItems(r.Context(), r.PathValue("slug"), req.ItemIDs)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	dtos, err := a.itemDTOs(r, items)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dtos)
}

// updateItem edits an agenda entry's notes. Both belong to this occurrence of
// the problem, so a later meeting discussing the same problem gets its own pair.
func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathInt64(r, "itemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	var req itemUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}
	item, err := a.store.UpdateItem(r.Context(), r.PathValue("slug"), itemID,
		store.ItemPatch{PrepNote: req.PrepNote, ActionNote: req.ActionNote})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	dtos, err := a.itemDTOs(r, []store.MeetingItem{item})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dtos[0])
}

// deleteItem removes an entry from the agenda. The problem is untouched and
// stays in the bucket.
func (a *API) deleteItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathInt64(r, "itemId")
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
		return
	}
	if err := a.store.DeleteItem(r.Context(), r.PathValue("slug"), itemID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// itemDTOs hydrates agenda items with their problems and attachments.
//
// The embedded shape is ProblemWithAttachments, never ProblemDetail: the
// latter's timeline would repeat the very meeting being rendered inside every
// item, and would make a drag-and-drop drop re-download each problem's whole
// history through the reorder response.
func (a *API) itemDTOs(r *http.Request, items []store.MeetingItem) ([]meetingItemDTO, error) {
	ctx := r.Context()
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ProblemID)
	}
	problems, err := a.store.ProblemsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	attachments, err := a.store.AttachmentsByProblems(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]meetingItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, meetingItemDTO{
			ID: it.ID, MeetingID: it.MeetingID, ProblemID: it.ProblemID,
			Position: it.Position, PrepNote: it.PrepNote, ActionNote: it.ActionNote,
			Problem: toProblemWithAttachments(problems[it.ProblemID], attachments[it.ProblemID]),
		})
	}
	return out, nil
}
