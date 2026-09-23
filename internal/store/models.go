package store

import "slices"

// Label is one of the tags a problem can carry. The vocabulary is closed on
// purpose: these two say which discipline the problem belongs to, and a free
// text field would hold "UX", "ux " and "uix" within a month — three tags for
// one thing, and a filter that finds none of them.
//
// The stored values are English like the rest of the schema; their Czech names
// ("UX", "Analýza") live in the frontend's string table.
type Label string

const (
	LabelUX       Label = "ux"
	LabelAnalysis Label = "analysis"
)

// LabelOrder is the vocabulary in the order every screen shows it, and the
// order a set is stored and returned in, so one pair of labels never renders
// two ways.
var LabelOrder = []Label{LabelUX, LabelAnalysis}

// Valid reports whether l is one of the documented labels. LabelOrder is the
// only list of them, so a new label cannot be valid yet missing from the order.
func (l Label) Valid() bool { return slices.Contains(LabelOrder, l) }

// NormalizeLabels validates a requested set and returns it in canonical order
// with duplicates collapsed, so {analysis, ux} and {ux, ux, analysis} are the
// same set and are stored the same way.
//
// Values must match exactly — no trimming, no case folding — so both request
// shapes and the contract's enum accept the same strings.
//
// ok is false when a value is not in the vocabulary; the refusal itself is left
// to the caller, which is the only side that knows the field name and speaks
// Czech.
func NormalizeLabels(in []string) ([]string, bool) {
	for _, raw := range in {
		if !Label(raw).Valid() {
			return nil, false
		}
	}
	return canonicalLabels(in), true
}

// canonicalLabels returns the labels of LabelOrder that occur in values, once
// each and in that order. Anything outside the vocabulary is left out.
func canonicalLabels(values []string) []string {
	out := make([]string, 0, len(LabelOrder))
	for _, l := range LabelOrder {
		if slices.Contains(values, string(l)) {
			out = append(out, string(l))
		}
	}
	return out
}

// Problem is a row of the bucket. iso_* and archived-style derived values live
// in the HTTP layer; this struct is storage shaped.
type Problem struct {
	ID              int64
	Title           string
	Description     string
	Link            string
	Labels          []string
	CreatedAt       string
	CreatedBy       string
	UpdatedAt       string
	Done            bool
	DoneAt          *string
	DoneBy          *string
	AttachmentCount int
	MeetingCount    int
}

// Attachment is a file belonging to a problem. The bytes live on disk under
// StorageName; only this metadata is in SQLite.
type Attachment struct {
	ID          int64
	ProblemID   int64
	Filename    string
	ContentType string
	SizeBytes   int64
	StorageName string
	CreatedAt   string
	CreatedBy   string
}

// NewAttachment is an attachment about to be inserted, after its bytes have
// been staged on disk.
type NewAttachment struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	StorageName string
}

// Meeting is one weekly session. iso_year, iso_week and archived are derived
// from MeetingDate on read and are deliberately not stored.
type Meeting struct {
	ID          int64
	Slug        string
	MeetingDate string
	Note        string
	ItemCount   int
	CreatedAt   string
	CreatedBy   string
}

// MeetingItem is an agenda entry: the link between a problem and a meeting, and
// the carrier of the admin's per-meeting notes.
type MeetingItem struct {
	ID         int64
	MeetingID  int64
	ProblemID  int64
	Position   int
	PrepNote   string
	ActionNote string
	CreatedAt  string
}

// ProblemMeetingRef is one entry of a problem's cross-meeting timeline.
type ProblemMeetingRef struct {
	ItemID      int64
	MeetingID   int64
	Slug        string
	MeetingDate string
	Position    int
	PrepNote    string
	ActionNote  string
}

// SortKey selects the bucket's ordering.
type SortKey string

const (
	SortCreatedDesc  SortKey = "created_desc"
	SortCreatedAsc   SortKey = "created_asc"
	SortMeetingsDesc SortKey = "meetings_desc"
)

// Valid reports whether k is one of the three documented sort keys.
func (k SortKey) Valid() bool {
	switch k {
	case SortCreatedDesc, SortCreatedAsc, SortMeetingsDesc:
		return true
	}
	return false
}

// ProblemFilter carries the bucket's query parameters that are resolved in SQL.
// The text filter `q` is deliberately absent: it cannot be done in SQLite for
// Czech and is applied in Go afterwards (see internal/search).
type ProblemFilter struct {
	Done      *bool
	Scheduled *bool
	Sort      SortKey
}

// ProblemInput is the payload for creating a problem.
type ProblemInput struct {
	Title       string
	Description string
	Link        string
	Labels      []string
}

// ProblemPatch is a partial update. Every field is a pointer because the empty
// value is a meaningful one: link = "" clears the link and labels = [] clears
// every label, while omitting either leaves it untouched.
type ProblemPatch struct {
	Title       *string
	Description *string
	Link        *string
	Labels      *[]string
}

// MeetingPatch is a partial update of a meeting.
type MeetingPatch struct {
	MeetingDate *string
	Note        *string
}

// ItemPatch is a partial update of an agenda entry's notes.
type ItemPatch struct {
	PrepNote   *string
	ActionNote *string
}
