import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMeetings } from '../../api/hooks'
import type { Meeting, User } from '../../api/types'
import { cs, itemCount } from '../../i18n/cs'
import { formatDate, parseDate, weekLabel, weekdayAndDate } from '../../lib/format'
import { PlusIcon, CheckIcon } from '../../components/Icons'
import { EmptyState, Spinner, btn, columnHeader, cx } from '../../components/ui'
import { NewMeetingDialog } from './NewMeetingDialog'

/** The meetings list (A6). */
export function MeetingsPage({ user }: { user: User | null }) {
  const [showArchive, setShowArchive] = useState(false)
  const [creating, setCreating] = useState(false)
  const { data: meetings, isPending } = useMeetings(showArchive)

  const rows = meetings ?? []
  const isAdmin = user?.role === 'admin'

  // The list is not a date window: the meeting being prepared is always
  // future-dated and is the row the admin clicks most, so it sits at the top
  // and says so.
  const next = useMemo(() => {
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    return rows
      .filter((m) => parseDate(m.meeting_date) >= today)
      .sort((a, b) => a.meeting_date.localeCompare(b.meeting_date))[0]
  }, [rows])

  return (
    <div className="px-8 pt-[26px]">
      <div className="flex items-end justify-between gap-6">
        <div>
          <h1 className="m-0 text-[20px] font-semibold leading-[1.2] tracking-[-0.01em]">{cs.meetings.title}</h1>
          <div className="mt-[5px] text-[13px] leading-none text-muted">
            {isPending
              ? cs.common.loading
              : next
                ? cs.meetings.nextMeeting(weekdayAndDate(next.meeting_date))
                : `${rows.length} ${rows.length === 1 ? 'porada' : rows.length >= 2 && rows.length <= 4 ? 'porady' : 'porad'}`}
          </div>
        </div>

        {isAdmin && (
          <button type="button" onClick={() => setCreating(true)} className={btn.primary}>
            <PlusIcon />
            {cs.meetings.create}
          </button>
        )}
      </div>

      <div className="mt-6">
        <div className="grid grid-cols-[140px_130px_minmax(0,1fr)_120px] items-center gap-4 border-b border-rule-strong pb-[9px]">
          <span className={columnHeader}>{cs.meetings.columns.date}</span>
          <span className={columnHeader}>{cs.meetings.columns.week}</span>
          <span className={columnHeader}>{cs.meetings.columns.agenda}</span>
          <span />
        </div>

        {isPending && <Spinner />}

        {!isPending &&
          rows.map((m) => <MeetingRow key={m.id} meeting={m} isNext={m.id === next?.id} />)}

        {!isPending && rows.length === 0 && <EmptyState title={cs.meetings.empty} />}
      </div>

      <div className="mt-[18px]">
        <button
          type="button"
          onClick={() => setShowArchive((v) => !v)}
          aria-pressed={showArchive}
          className="flex h-[30px] items-center gap-2 rounded-[6px] border border-field bg-white px-[11px] text-[13px] leading-none text-ink hover:bg-surface-hover focus-ring"
        >
          <span
            className={cx(
              'flex h-[14px] w-[14px] items-center justify-center rounded-[3px]',
              showArchive ? 'bg-primary text-white' : 'border-[1.5px] border-check',
            )}
          >
            {showArchive && <CheckIcon size={10} />}
          </span>
          {cs.meetings.showArchive}
        </button>
        <div className="mt-2 text-[12px] leading-[1.5] text-faint">{cs.meetings.archiveHint}</div>
      </div>

      {isAdmin && <NewMeetingDialog open={creating} onClose={() => setCreating(false)} />}
    </div>
  )
}

function MeetingRow({ meeting: m, isNext }: { meeting: Meeting; isNext: boolean }) {
  return (
    <div className="grid grid-cols-[140px_130px_minmax(0,1fr)_120px] items-center gap-4 border-b border-rule py-[13px] transition-colors hover:bg-surface-sunken">
      <Link
        to={`/porada/${m.slug}`}
        className="text-[14.5px] font-medium leading-none text-ink no-underline hover:text-primary"
      >
        {formatDate(m.meeting_date)}
      </Link>
      <span className="text-[13.5px] leading-none text-secondary">{weekLabel(m.iso_week)}</span>
      <span className="text-[13.5px] leading-none text-secondary">{itemCount(m.item_count)}</span>
      <span className="flex justify-end">
        {isNext && !m.archived && (
          <span className="rounded-[4px] border border-primary-line bg-primary-tint px-2 py-[3px] text-[11px] font-medium uppercase leading-[1.4] tracking-[0.04em] text-primary-ink">
            {cs.meetings.upcoming}
          </span>
        )}
        {m.archived && (
          <span className="rounded-[4px] border border-line bg-surface-hover px-2 py-[3px] text-[11px] font-medium uppercase leading-[1.4] tracking-[0.04em] text-muted">
            {cs.meetings.archived}
          </span>
        )}
      </span>
    </div>
  )
}
