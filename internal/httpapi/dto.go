package httpapi

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/auth"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/slug"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/timeutil"
)

// userDTO mirrors the User schema.
type userDTO struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

func toUser(a auth.Account) userDTO {
	return userDTO{Username: a.Username, DisplayName: a.DisplayName, Role: string(a.Role)}
}

// problemDTO mirrors the Problem schema.
type problemDTO struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	// Canonical order, never null: an empty set is [] like every other
	// collection on the wire.
	Labels          []string `json:"labels"`
	CreatedAt       string   `json:"created_at"`
	CreatedBy       string   `json:"created_by"`
	UpdatedAt       string   `json:"updated_at"`
	Done            bool     `json:"done"`
	DoneAt          *string  `json:"done_at"`
	DoneBy          *string  `json:"done_by"`
	AttachmentCount int      `json:"attachment_count"`
	MeetingCount    int      `json:"meeting_count"`
}

// problemWithAttachmentsDTO mirrors ProblemWithAttachments: a problem plus its
// files, without the cross-meeting timeline. This is what an agenda entry
// embeds.
type problemWithAttachmentsDTO struct {
	problemDTO
	Attachments []attachmentDTO `json:"attachments"`
}

// problemDetailDTO mirrors ProblemDetail: the above plus the timeline.
type problemDetailDTO struct {
	problemWithAttachmentsDTO
	Meetings []problemMeetingRefDTO `json:"meetings"`
}

type problemMeetingRefDTO struct {
	ItemID      int64  `json:"item_id"`
	Slug        string `json:"slug"`
	MeetingDate string `json:"meeting_date"`
	ISOYear     int    `json:"iso_year"`
	ISOWeek     int    `json:"iso_week"`
	Position    int    `json:"position"`
	PrepNote    string `json:"prep_note"`
	ActionNote  string `json:"action_note"`
}

type attachmentDTO struct {
	ID          int64  `json:"id"`
	ProblemID   int64  `json:"problem_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	URL         string `json:"url"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

type meetingDTO struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	MeetingDate string `json:"meeting_date"`
	ISOYear     int    `json:"iso_year"`
	ISOWeek     int    `json:"iso_week"`
	Note        string `json:"note"`
	Archived    bool   `json:"archived"`
	ItemCount   int    `json:"item_count"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

type meetingDetailDTO struct {
	meetingDTO
	Items []meetingItemDTO `json:"items"`
}

type meetingItemDTO struct {
	ID         int64                     `json:"id"`
	MeetingID  int64                     `json:"meeting_id"`
	ProblemID  int64                     `json:"problem_id"`
	Position   int                       `json:"position"`
	PrepNote   string                    `json:"prep_note"`
	ActionNote string                    `json:"action_note"`
	Problem    problemWithAttachmentsDTO `json:"problem"`
	// The problem's non-empty action notes from earlier agendas, oldest first;
	// [] when there are none.
	PreviousActions []previousActionDTO `json:"previous_actions"`
}

// previousActionDTO is one earlier meeting's outcome for an agenda item's
// problem: just the note and enough of its meeting to label and link it.
type previousActionDTO struct {
	ItemID      int64  `json:"item_id"`
	Slug        string `json:"slug"`
	MeetingDate string `json:"meeting_date"`
	ISOYear     int    `json:"iso_year"`
	ISOWeek     int    `json:"iso_week"`
	ActionNote  string `json:"action_note"`
}

func toProblem(p store.Problem) problemDTO {
	labels := p.Labels
	if labels == nil {
		labels = []string{}
	}
	return problemDTO{
		ID: p.ID, Title: p.Title, Description: p.Description, Link: p.Link, Labels: labels,
		CreatedAt: p.CreatedAt, CreatedBy: p.CreatedBy, UpdatedAt: p.UpdatedAt,
		Done: p.Done, DoneAt: p.DoneAt, DoneBy: p.DoneBy,
		AttachmentCount: p.AttachmentCount, MeetingCount: p.MeetingCount,
	}
}

func toProblems(ps []store.Problem) []problemDTO {
	out := make([]problemDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProblem(p))
	}
	return out
}

// attachmentURL is the root-relative URL of an attachment's bytes.
//
// It resolves against the page's own origin, which serves /api in both
// environments — proxied by Vite in development, routed by the SPA listener to
// the same in-process handler in production. An absolute URL here would break
// the moment the app is reached by a different hostname.
func attachmentURL(id int64) string {
	return fmt.Sprintf("/api/attachments/%d/content", id)
}

func toAttachment(a store.Attachment) attachmentDTO {
	return attachmentDTO{
		ID: a.ID, ProblemID: a.ProblemID, Filename: a.Filename,
		ContentType: a.ContentType, SizeBytes: a.SizeBytes, URL: attachmentURL(a.ID),
		CreatedAt: a.CreatedAt, CreatedBy: a.CreatedBy,
	}
}

func toAttachments(as []store.Attachment) []attachmentDTO {
	out := make([]attachmentDTO, 0, len(as))
	for _, a := range as {
		out = append(out, toAttachment(a))
	}
	return out
}

func toProblemWithAttachments(p store.Problem, as []store.Attachment) problemWithAttachmentsDTO {
	return problemWithAttachmentsDTO{problemDTO: toProblem(p), Attachments: toAttachments(as)}
}

// toMeeting derives iso_year, iso_week and archived from meeting_date.
//
// None of the three is stored. The slug keeps a frozen copy of the creation
// week, so after a date change the label follows the new date while the URL
// does not — the single, deliberate divergence.
func toMeeting(m store.Meeting, now time.Time, archiveAfterDays int) (meetingDTO, error) {
	date, err := timeutil.ParseDate(m.MeetingDate)
	if err != nil {
		return meetingDTO{}, fmt.Errorf("meeting %s has an unreadable date: %w", m.Slug, err)
	}
	year, week := slug.ISOWeek(date)
	return meetingDTO{
		ID: m.ID, Slug: m.Slug, MeetingDate: m.MeetingDate,
		ISOYear: year, ISOWeek: week, Note: m.Note,
		Archived:  timeutil.IsArchived(date, now, archiveAfterDays),
		ItemCount: m.ItemCount, CreatedAt: m.CreatedAt, CreatedBy: m.CreatedBy,
	}, nil
}

// meetingWeek derives the ISO year and week of a stored meeting date, for the
// references to a meeting that carry its label but not the whole meeting.
func meetingWeek(meetingSlug, meetingDate string) (year, week int, err error) {
	date, err := timeutil.ParseDate(meetingDate)
	if err != nil {
		return 0, 0, fmt.Errorf("meeting %s has an unreadable date: %w", meetingSlug, err)
	}
	year, week = slug.ISOWeek(date)
	return year, week, nil
}

func toProblemMeetingRef(r store.ProblemMeetingRef) (problemMeetingRefDTO, error) {
	year, week, err := meetingWeek(r.Slug, r.MeetingDate)
	if err != nil {
		return problemMeetingRefDTO{}, err
	}
	return problemMeetingRefDTO{
		ItemID: r.ItemID, Slug: r.Slug, MeetingDate: r.MeetingDate,
		ISOYear: year, ISOWeek: week, Position: r.Position,
		PrepNote: r.PrepNote, ActionNote: r.ActionNote,
	}, nil
}

// toPreviousActions renders an agenda item's earlier action notes.
//
// A note whose meeting has an unreadable date is logged and left out rather
// than failed: it belongs to another agenda, and must not take this one down.
func toPreviousActions(actions []store.PreviousAction) []previousActionDTO {
	out := make([]previousActionDTO, 0, len(actions))
	for _, a := range actions {
		year, week, err := meetingWeek(a.Slug, a.MeetingDate)
		if err != nil {
			slog.Error("skip previous action note", "item", a.ItemID, "err", err)
			continue
		}
		out = append(out, previousActionDTO{
			ItemID: a.ItemID, Slug: a.Slug, MeetingDate: a.MeetingDate,
			ISOYear: year, ISOWeek: week, ActionNote: a.ActionNote,
		})
	}
	return out
}
