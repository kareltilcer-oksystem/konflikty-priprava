import { useEffect, useMemo, useState } from 'react'
import { useAddItems, useDisplayName, useProblems } from '../../api/hooks'
import { ApiError } from '../../api/client'
import type { MeetingDetail, SortKey } from '../../api/types'
import { cs } from '../../i18n/cs'
import { meetingTitle, relativeAge } from '../../lib/format'
import { CheckIcon, CloseIcon, SearchIcon } from '../../components/Icons'
import { ErrorBox, LabelChips, MeetingBadge, Modal, btn, cx } from '../../components/ui'
import { SortSelect } from '../problems/BucketPage'

/**
 * The agenda picker (A10).
 *
 * A multi-select over the bucket: several problems are added in one action, and
 * problems already on this agenda appear disabled rather than hidden, so the
 * admin can see they are handled. Done problems are not offered at all — a
 * solved problem has no business on a future agenda.
 *
 * `nejčastěji na poradě` leads the sort here rather than trailing it: this is
 * the screen where the deferral signal earns its keep.
 */
export function AddProblemsPicker({
  meeting,
  open,
  onClose,
}: {
  meeting: MeetingDetail
  open: boolean
  onClose: () => void
}) {
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<SortKey>('meetings_desc')
  const [picked, setPicked] = useState<Set<number>>(new Set())

  const displayName = useDisplayName()
  const addItems = useAddItems(meeting.slug)
  const { data: problems } = useProblems({ done: false, ...(query.trim() ? { q: query.trim() } : {}), sort })

  useEffect(() => {
    if (!open) return
    setPicked(new Set())
    setQuery('')
    addItems.reset()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const onAgenda = useMemo(() => new Set(meeting.items.map((i) => i.problem_id)), [meeting.items])
  const rows = problems ?? []

  function toggle(id: number) {
    setPicked((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function submit() {
    if (picked.size === 0) return
    // The order the admin ticked them in is the order they are appended.
    const ids = rows.filter((p) => picked.has(p.id)).map((p) => p.id)
    addItems.mutate(ids, { onSuccess: onClose })
  }

  const message = addItems.error instanceof ApiError ? addItems.error.message : null

  return (
    <Modal open={open} onClose={onClose} width={900} labelledBy="picker-title">
      <div className="flex flex-none items-start justify-between px-6 pt-5">
        <div>
          <div id="picker-title" className="text-[17px] font-semibold leading-[1.3]">
            {cs.picker.title}
          </div>
          <div className="mt-[5px] text-[12.5px] leading-none text-muted">
            {meetingTitle(meeting.iso_week, meeting.iso_year)} · {cs.picker.subtitle}
          </div>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label={cs.picker.cancel}
          className={cx(btn.icon, 'h-[30px] w-[30px]')}
        >
          <CloseIcon />
        </button>
      </div>

      <div className="flex flex-none items-center gap-3 px-6 py-4">
        <div className="relative w-[280px]">
          <SearchIcon className="pointer-events-none absolute left-[10px] top-[9px] text-muted" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={cs.bucket.search}
            aria-label={cs.bucket.search}
            className="h-[33px] w-full rounded-[6px] border border-field bg-white pl-[31px] pr-[10px] text-[13px] text-ink outline-none focus-ring"
          />
        </div>
        <div className="flex-1" />
        <span className="text-[12.5px] leading-none text-muted">{cs.bucket.sortLabel}</span>
        <SortSelect value={sort} onChange={setSort} />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6">
        {rows.length === 0 && (
          <div className="py-12 text-center text-[13.5px] text-muted">{cs.picker.empty}</div>
        )}

        {rows.map((p) => {
          const already = onAgenda.has(p.id)
          const selected = picked.has(p.id)
          return (
            <div
              key={p.id}
              className="grid grid-cols-[22px_minmax(0,1fr)_140px_100px_52px] items-center gap-[14px] border-t border-rule py-3"
            >
              {already ? (
                <span className="block h-[17px] w-[17px] rounded-[4px] border-[1.5px] border-line bg-surface-track" />
              ) : (
                <button
                  type="button"
                  onClick={() => toggle(p.id)}
                  role="checkbox"
                  aria-checked={selected}
                  aria-label={p.title}
                  className={cx(
                    'flex h-[17px] w-[17px] items-center justify-center rounded-[4px] p-0 transition-colors focus-ring',
                    selected
                      ? 'bg-primary text-white'
                      : 'border-[1.5px] border-check bg-white hover:border-primary',
                  )}
                >
                  {selected && <CheckIcon size={11} />}
                </button>
              )}

              <div className="flex min-w-0 items-center gap-[9px]">
                <span
                  className={cx(
                    'truncate text-[14px] leading-[1.35]',
                    already ? 'text-faint' : 'font-medium text-ink',
                  )}
                  title={p.title}
                >
                  {p.title}
                </span>
                <LabelChips labels={p.labels} />
                {already && (
                  <span className="flex-none rounded-[4px] border border-line bg-surface-hover px-[7px] py-[2px] text-[10.5px] font-medium uppercase leading-[1.4] tracking-[0.04em] text-muted">
                    {cs.picker.onAgenda}
                  </span>
                )}
              </div>

              <span className="truncate text-[12.5px] leading-none text-muted">{displayName(p.created_by)}</span>
              <span className="text-[12.5px] leading-none text-faint">{relativeAge(p.created_at)}</span>
              <span className="flex justify-end">
                <MeetingBadge count={p.meeting_count} />
              </span>
            </div>
          )
        })}
      </div>

      {message && (
        <div className="flex-none px-6 pb-3">
          <ErrorBox title={message} />
        </div>
      )}

      <div className="flex flex-none items-center justify-between gap-4 border-t border-line bg-surface-sunken px-6 py-[14px]">
        <span className="text-[12px] leading-[1.4] text-muted">{cs.picker.doneHint}</span>
        <span className="flex items-center gap-[9px]">
          <button type="button" onClick={onClose} className={btn.secondary}>
            {cs.picker.cancel}
          </button>
          <button
            type="button"
            onClick={submit}
            disabled={picked.size === 0 || addItems.isPending}
            className={btn.primary}
          >
            {picked.size === 0 ? cs.picker.submitEmpty : cs.picker.submit(picked.size)}
          </button>
        </span>
      </div>
    </Modal>
  )
}
