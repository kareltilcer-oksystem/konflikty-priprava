package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

// Stable machine-readable error codes (api/openapi.yaml).
const (
	CodeValidationFailed    = "validation_failed"
	CodeInvalidCredentials  = "invalid_credentials"
	CodeUnauthorized        = "unauthorized"
	CodeForbidden           = "forbidden"
	CodeNotFound            = "not_found"
	CodeFileTooLarge        = "file_too_large"
	CodeAlreadyOnAgenda     = "already_on_agenda"
	CodeInvalidOrder        = "invalid_order"
	CodeRangeNotSatisfiable = "range_not_satisfiable"
	CodeInternal            = "internal_error"
)

// Czech messages. The API emits display-ready Czech (PRD 9.3): the frontend
// prints error.message as it comes rather than maintaining a second copy of
// these strings keyed by code.
const (
	msgValidationFailed    = "Neplatný požadavek."
	msgTitleRequired       = "Vyplňte název problému."
	msgTitleTooLong        = "Název může mít nejvýše 200 znaků."
	msgLinkScheme          = "Odkaz musí začínat http:// nebo https://."
	msgAuthorForbidden     = "Vložit problém za jiného uživatele může pouze správce."
	msgUnknownAuthor       = "Vybraný autor neexistuje."
	msgUnknownLabel        = "Vybraný štítek neexistuje."
	msgFieldRepeated       = "Každé pole formuláře lze odeslat jen jednou."
	msgInvalidCredentials  = "Nesprávné jméno nebo heslo."
	msgUnauthorized        = "Pro tuto akci se musíte přihlásit."
	msgForbidden           = "Tuto akci může provést pouze správce."
	msgNotFound            = "Záznam nebyl nalezen."
	msgAlreadyOnAgenda     = "Problém už je na programu této porady."
	msgDuplicateProblem    = "Každý problém lze přidat jen jednou."
	msgUnknownProblem      = "Některý z vybraných problémů neexistuje."
	msgInvalidOrder        = "Seznam pořadí neodpovídá bodům této porady."
	msgRangeNotSatisfiable = "Požadovaná část souboru neexistuje."
	msgInternal            = "Na serveru došlo k chybě."
	msgBadJSON             = "Tělo požadavku není platný JSON."
	msgBadDate             = "Datum porady je neplatné."
	msgBadSort             = "Neplatné řazení."
	msgBadBool             = "Neplatná hodnota filtru."
	msgProblemIDsRequired  = "Vyberte alespoň jeden problém."
	msgNotMultipart        = "Přílohy je nutné odeslat jako multipart/form-data."
)

// apiError is the single error shape every failing endpoint returns.
type apiError struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// writeError sends the JSON error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeErrorDetails(w, status, code, message, nil)
}

func writeErrorDetails(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiError{Error: errorBody{
		Code: code, Message: message, Details: details,
	}}); err != nil {
		slog.Error("write error response", "err", err)
	}
}

// writeStoreError maps a store error onto its status. Anything unrecognised is
// a 500 whose detail is logged rather than returned, so an internal message
// never reaches a page every anonymous visitor can open.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, msgNotFound)
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, CodeAlreadyOnAgenda, msgAlreadyOnAgenda)
	case errors.Is(err, store.ErrInvalidOrder):
		writeError(w, http.StatusBadRequest, CodeInvalidOrder, msgInvalidOrder)
	case errors.Is(err, store.ErrUnknownProblem):
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgUnknownProblem)
	default:
		slog.Error("unhandled store error", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
	}
}

// writeJSON sends a successful JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "err", err)
	}
}
