import { plural } from '../i18n/cs'

/** Parses a YYYY-MM-DD date as local midnight. */
export function parseDate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1)
}

/** Czech date with spaces after the dots: `16. 9. 2026` (PRD §8). */
export function formatDate(iso: string): string {
  const d = iso.includes('T') ? new Date(iso) : parseDate(iso)
  return `${d.getDate()}. ${d.getMonth() + 1}. ${d.getFullYear()}`
}

/** `Týden 38` (PRD §8). */
export function weekLabel(isoWeek: number): string {
  return `Týden ${isoWeek}`
}

/** `Porada — týden 38 (2026)` (PRD §8). */
export function meetingTitle(isoWeek: number, isoYear: number): string {
  return `Porada — týden ${isoWeek} (${isoYear})`
}

const WEEKDAYS = ['v neděli', 'v pondělí', 'v úterý', 've středu', 've čtvrtek', 'v pátek', 'v sobotu']

/** `v pátek 25. 9. 2026`, for the meetings-list subtitle. */
export function weekdayAndDate(iso: string): string {
  const d = parseDate(iso)
  return `${WEEKDAYS[d.getDay()]} ${formatDate(iso)}`
}

/**
 * Relative age in Czech: `dnes`, `před 3 dny`, `před 2 týdny`, `před měsícem`.
 *
 * The bucket shows when a problem arrived, and an exact timestamp carries no
 * information anyone acts on — what matters is whether it has been sitting
 * there a day or a month.
 */
export function relativeAge(isoTimestamp: string, now: Date = new Date()): string {
  const then = new Date(isoTimestamp)
  const days = Math.floor((startOfDay(now).getTime() - startOfDay(then).getTime()) / 86_400_000)

  if (days <= 0) return 'dnes'
  if (days === 1) return 'včera'
  if (days < 7) return `před ${days} dny`
  if (days < 14) return 'před týdnem'
  if (days < 31) {
    const weeks = Math.floor(days / 7)
    return weeks === 1 ? 'před týdnem' : `před ${weeks} ${plural(weeks, 'týdnem', 'týdny', 'týdny')}`
  }
  const months = Math.floor(days / 30)
  if (months === 1) return 'před měsícem'
  if (months < 12) return `před ${months} ${plural(months, 'měsícem', 'měsíci', 'měsíci')}`
  const years = Math.floor(days / 365)
  return years === 1 ? 'před rokem' : `před ${years} ${plural(years, 'rokem', 'roky', 'lety')}`
}

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

/**
 * File size with a Czech decimal comma: `180 kB`, `24,3 MB`.
 *
 * Decimal units, not binary: this number sits next to a 100 MB limit the user
 * was told about, and 100 MB there means 100,000,000 bytes.
 */
export function formatSize(bytes: number): string {
  if (bytes < 1000) return `${bytes} B`
  if (bytes < 1_000_000) return `${Math.round(bytes / 1000)} kB`
  if (bytes < 1_000_000_000) {
    const mb = bytes / 1_000_000
    const rounded = mb < 10 ? mb.toFixed(1) : String(Math.round(mb))
    return `${rounded.replace('.', ',')} MB`
  }
  return `${(bytes / 1_000_000_000).toFixed(1).replace('.', ',')} GB`
}

/** How an attachment should be rendered. SVG is never a preview — see below. */
export type AttachmentKind = 'image' | 'video' | 'file'

/**
 * Classifies an attachment for display.
 *
 * SVG is deliberately a download tile rather than an image: it is a scriptable
 * document, and the API refuses to serve it inline for the same reason.
 */
export function attachmentKind(contentType: string): AttachmentKind {
  const base = contentType.split(';')[0]?.trim().toLowerCase() ?? ''
  if (base === 'image/svg+xml' || base === 'image/svg') return 'file'
  if (base.startsWith('image/')) return 'image'
  if (base.startsWith('video/')) return 'video'
  return 'file'
}

/**
 * Whether a link is safe to put in an href.
 *
 * The server rejects anything but http(s) on write, but this value is rendered
 * on a page every anonymous visitor can open, so the renderer refuses it too
 * rather than trusting that every row predates the check.
 */
export function safeLink(link: string): string | null {
  const trimmed = link.trim()
  if (!trimmed) return null
  const lower = trimmed.toLowerCase()
  return lower.startsWith('http://') || lower.startsWith('https://') ? trimmed : null
}

/** Strips the scheme so a long URL reads as a reference, not a wall of text. */
export function displayLink(link: string): string {
  return link.replace(/^https?:\/\//i, '')
}

/**
 * The ISO week-numbering year and week of a date.
 *
 * Both values come from the same calculation, and the year is the ISO
 * week-numbering year rather than the calendar year — they disagree at every
 * year boundary. 2027-01-01 is ISO 2026-W53; 2025-12-29 is ISO 2026-W01. The
 * server derives the slug the same way, so the preview in the new-meeting
 * dialog matches what gets created.
 */
export function isoWeek(date: Date): { year: number; week: number } {
  // Shift to the Thursday of this week: the ISO year is whichever year that
  // Thursday falls in.
  const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()))
  const dayNum = d.getUTCDay() || 7 // Monday = 1 … Sunday = 7
  d.setUTCDate(d.getUTCDate() + 4 - dayNum)
  const year = d.getUTCFullYear()
  const yearStart = new Date(Date.UTC(year, 0, 1))
  const week = Math.ceil(((d.getTime() - yearStart.getTime()) / 86_400_000 + 1) / 7)
  return { year, week }
}

/**
 * The slug a date would produce, for the preview in the new-meeting dialog.
 *
 * The week is always zero-padded, so a week has exactly one spelling. A second
 * meeting in the same week gets a -2 suffix the server allocates; the preview
 * shows the unsuffixed form, which is what the first meeting of a week gets.
 */
export function previewSlug(isoDate: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(isoDate)) return null
  const d = parseDate(isoDate)
  if (Number.isNaN(d.getTime())) return null
  const { year, week } = isoWeek(d)
  return `${String(year).padStart(4, '0')}-w${String(week).padStart(2, '0')}`
}
