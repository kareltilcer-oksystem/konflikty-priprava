package httpapi

import "net/http"

// routes is the authorization table: every endpoint, with its role visible on
// the same line.
//
// Patterns carry the literal /api prefix because this handler is mounted
// unchanged on both listeners. Every GET is public; every mutating route
// requires a session, and the admin-only ones check the role as well.
func (a *API) routes() http.Handler {
	mux := http.NewServeMux()

	// system
	mux.HandleFunc("GET /api/health", a.health)

	// auth
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/auth/me", a.me)
	mux.HandleFunc("GET /api/users", a.listUsers)

	// problems
	mux.HandleFunc("GET /api/problems", a.listProblems)
	mux.HandleFunc("POST /api/problems", requireAuth(a.createProblem))
	mux.HandleFunc("GET /api/problems/{problemId}", a.getProblem)
	mux.HandleFunc("PATCH /api/problems/{problemId}", requireAuth(a.updateProblem))
	mux.HandleFunc("DELETE /api/problems/{problemId}", requireAdmin(a.deleteProblem))
	mux.HandleFunc("POST /api/problems/{problemId}/done", requireAdmin(a.markDone))
	mux.HandleFunc("DELETE /api/problems/{problemId}/done", requireAdmin(a.markNotDone))

	// attachments
	mux.HandleFunc("POST /api/problems/{problemId}/attachments", requireAuth(a.uploadAttachment))
	mux.HandleFunc("DELETE /api/attachments/{attachmentId}", requireAuth(a.deleteAttachment))
	mux.HandleFunc("GET /api/attachments/{attachmentId}/content", a.attachmentContent)

	// meetings
	mux.HandleFunc("GET /api/meetings", a.listMeetings)
	mux.HandleFunc("POST /api/meetings", requireAdmin(a.createMeeting))
	mux.HandleFunc("GET /api/meetings/{slug}", a.getMeeting)
	mux.HandleFunc("PATCH /api/meetings/{slug}", requireAdmin(a.updateMeeting))
	mux.HandleFunc("DELETE /api/meetings/{slug}", requireAdmin(a.deleteMeeting))

	// agenda
	mux.HandleFunc("POST /api/meetings/{slug}/items", requireAdmin(a.addItems))
	mux.HandleFunc("PUT /api/meetings/{slug}/items/order", requireAdmin(a.reorderItems))
	mux.HandleFunc("PATCH /api/meetings/{slug}/items/{itemId}", requireAdmin(a.updateItem))
	mux.HandleFunc("DELETE /api/meetings/{slug}/items/{itemId}", requireAdmin(a.deleteItem))

	// An unknown path under /api must answer with the JSON envelope. Without
	// this it would fall through to the SPA handler on the 9999 listener and
	// return index.html to a client expecting JSON.
	mux.HandleFunc("/api/", a.notFound)

	return a.withSession(mux)
}

func (a *API) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": a.version})
}
