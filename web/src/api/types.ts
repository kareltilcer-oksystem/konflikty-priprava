// Wire types, mirroring api/openapi.yaml. Field names stay snake_case so the
// mapping is one-to-one and a schema change is visible here.

export type Role = 'admin' | 'editor'

/**
 * A problem's tag. The vocabulary is closed and defined by the server; the
 * Czech names it is shown under live in the string table (i18n/cs.ts).
 */
export type Label = 'ux' | 'analysis'

/** The vocabulary in the order the server returns it and every screen shows it. */
export const LABELS: Label[] = ['ux', 'analysis']

export interface User {
  username: string
  display_name: string
  role: Role
}

export interface Attachment {
  id: number
  problem_id: number
  filename: string
  content_type: string
  size_bytes: number
  /** Root-relative and ready for a src/href; resolves on the page's own origin. */
  url: string
  created_at: string
  created_by: string
}

export interface Problem {
  id: number
  title: string
  description: string
  link: string
  /** Canonical order, possibly empty — never null. */
  labels: Label[]
  created_at: string
  created_by: string
  updated_at: string
  done: boolean
  done_at: string | null
  done_by: string | null
  attachment_count: number
  /** How many agendas this problem has appeared on — the deferral signal. */
  meeting_count: number
}

export interface ProblemWithAttachments extends Problem {
  attachments: Attachment[]
}

/** One entry of a problem's cross-meeting timeline. */
export interface ProblemMeetingRef {
  item_id: number
  slug: string
  meeting_date: string
  iso_year: number
  iso_week: number
  /** 0-based; displayed as position + 1. */
  position: number
  prep_note: string
  action_note: string
}

export interface ProblemDetail extends ProblemWithAttachments {
  /** Oldest first. */
  meetings: ProblemMeetingRef[]
}

export interface Meeting {
  id: number
  slug: string
  meeting_date: string
  /** Derived from meeting_date on every read, never stored. */
  iso_year: number
  iso_week: number
  note: string
  /** Derived: more than ARCHIVE_AFTER_DAYS in the past. */
  archived: boolean
  item_count: number
  created_at: string
  created_by: string
}

export interface MeetingItem {
  id: number
  meeting_id: number
  problem_id: number
  position: number
  prep_note: string
  action_note: string
  problem: ProblemWithAttachments
}

export interface MeetingDetail extends Meeting {
  items: MeetingItem[]
}

export type SortKey = 'created_desc' | 'created_asc' | 'meetings_desc'
export type DoneFilter = 'open' | 'done' | 'all'

export interface ProblemQuery {
  done?: boolean
  q?: string
  scheduled?: boolean
  sort?: SortKey
}

export interface ProblemPatch {
  title?: string
  description?: string
  link?: string
  /** Replaces the whole set; `[]` clears it, omitting it leaves it alone. */
  labels?: Label[]
}
