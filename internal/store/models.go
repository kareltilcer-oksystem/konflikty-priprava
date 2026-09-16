package store

// Problem is a row of the bucket. iso_* and archived-style derived values live
// in the HTTP layer; this struct is storage shaped.
type Problem struct {
	ID              int64
	Title           string
	Description     string
	Link            string
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
}

// ProblemPatch is a partial update. Every field is a pointer because the empty
// string is a meaningful value: link = "" clears the link, while an omitted
// link leaves it untouched.
type ProblemPatch struct {
	Title       *string
	Description *string
	Link        *string
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
