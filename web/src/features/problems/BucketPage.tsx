import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useDisplayName, useProblems, useSetDone } from '../../api/hooks'
import type { DoneFilter, Problem, SortKey, User } from '../../api/types'
import { bucketCount, cs } from '../../i18n/cs'
import { relativeAge } from '../../lib/format'
import { PaperclipIcon, PlusIcon, SearchIcon } from '../../components/Icons'
import {
  DoneBadge,
  LabelChips,
  MeetingBadge,
  EmptyState,
  Spinner,
  btn,
  columnHeader,
  cx,
  selectCompact,
} from '../../components/ui'
import { ProblemFormDrawer } from './ProblemForm'

/** The bucket (A1–A3): the landing screen and the busiest one. */
export function BucketPage({ user }: { user: User | null }) {
  const [filter, setFilter] = useState<DoneFilter>('open')
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<SortKey>('created_desc')
  const [formOpen, setFormOpen] = useState(false)

  const params = useMemo(
    () => ({
      ...(filter === 'all' ? {} : { done: filter === 'done' }),
      ...(query.trim() ? { q: query.trim() } : {}),
      sort,
    }),
    [filter, query, sort],
  )

  const { data: problems, isPending, isError } = useProblems(params)
  const setDone = useSetDone()
  const displayName = useDisplayName()

  const rows = problems ?? []
  const searching = query.trim().length > 0

  return (
    <div className="px-8 pt-[26px]">
      <div className="flex items-end justify-between gap-6">
        <div>
          <h1 className="m-0 text-[20px] font-semibold leading-[1.2] tracking-[-0.01em]">{cs.bucket.title}</h1>
          <div className="mt-[5px] text-[13px] leading-none text-muted">
            {isPending ? cs.common.loading : bucketCount(rows.length, filter)}
          </div>
        </div>

        {/* Write controls are absent for an anonymous visitor, not disabled:
            the read-only page has to look complete and intentional. */}
        {user && (
          <button type="button" onClick={() => setFormOpen(true)} className={btn.primary}>
            <PlusIcon />
            {cs.bucket.add}
          </button>
        )}
      </div>

      <div className="mt-[22px] flex flex-wrap items-center gap-[14px]">
        <SegmentedFilter value={filter} onChange={setFilter} />

        <div className="relative w-[300px]">
          <SearchIcon className="pointer-events-none absolute left-[10px] top-[9px] text-muted" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={cs.bucket.search}
            aria-label={cs.bucket.search}
            className="h-[33px] w-full rounded-[6px] border border-field bg-white pl-[31px] pr-[10px] text-[13px] text-ink outline-none transition-colors focus-ring"
          />
        </div>

        <div className="flex-1" />

        <label className="text-[12.5px] leading-none text-muted" htmlFor="bucket-sort">
          {cs.bucket.sortLabel}
        </label>
        <SortSelect id="bucket-sort" value={sort} onChange={setSort} />
      </div>

      <div className="mt-5">
        <div
          className={cx(
            'grid items-center gap-4 border-b border-rule-strong pb-[9px]',
            user ? 'grid-cols-[28px_minmax(0,1fr)_150px_104px_70px_56px]' : 'grid-cols-[minmax(0,1fr)_150px_104px_70px_56px]',
          )}
        >
          {user && <span />}
          <span className={columnHeader}>{cs.bucket.columns.problem}</span>
          <span className={columnHeader}>{cs.bucket.columns.author}</span>
          <span className={columnHeader}>{cs.bucket.columns.created}</span>
          <span className={cx(columnHeader, 'text-center')}>{cs.bucket.columns.attachments}</span>
          <span className={cx(columnHeader, 'text-right')}>{cs.bucket.columns.meetings}</span>
        </div>

        {isPending && <Spinner />}
        {isError && <EmptyState title={cs.errors.unknown} />}

        {!isPending &&
          rows.map((p) => (
            <ProblemRow
              key={p.id}
              problem={p}
              author={displayName(p.created_by)}
              canToggle={Boolean(user?.role === 'admin')}
              showToggleColumn={Boolean(user)}
              onToggle={() => setDone.mutate({ id: p.id, done: !p.done })}
            />
          ))}

        {!isPending && rows.length === 0 && searching && (
          <EmptyState
            title={cs.bucket.emptySearch}
            hint={cs.bucket.emptySearchHint}
            action={
              <button type="button" onClick={() => setQuery('')} className={btn.small}>
                {cs.bucket.clearSearch}
              </button>
            }
          />
        )}
        {!isPending && rows.length === 0 && !searching && (
          <EmptyState
            title={filter === 'done' ? 'Žádné vyřešené problémy.' : cs.bucket.emptyList}
            hint={filter === 'open' ? cs.bucket.emptyListHint : undefined}
          />
        )}
      </div>

      {user && <ProblemFormDrawer open={formOpen} onClose={() => setFormOpen(false)} />}
    </div>
  )
}

function ProblemRow({
  problem: p,
  author,
  canToggle,
  showToggleColumn,
  onToggle,
}: {
  problem: Problem
  author: string
  canToggle: boolean
  showToggleColumn: boolean
  onToggle: () => void
}) {
  return (
    <div
      className={cx(
        'grid items-center gap-4 border-b border-rule py-3 transition-colors hover:bg-surface-sunken',
        showToggleColumn
          ? 'grid-cols-[28px_minmax(0,1fr)_150px_104px_70px_56px]'
          : 'grid-cols-[minmax(0,1fr)_150px_104px_70px_56px]',
      )}
    >
      {showToggleColumn && (
        <span>
          {/* Ticking problems off is the admin's weekly routine, so the toggle
              lives on the list and not only on the detail page. */}
          {canToggle ? (
            <DoneToggle done={p.done} onToggle={onToggle} />
          ) : (
            <span
              className={cx(
                'block h-5 w-5 rounded-full border-[1.5px]',
                p.done ? 'border-done bg-done' : 'border-check-round',
              )}
            />
          )}
        </span>
      )}

      <div className="flex min-w-0 items-center gap-[10px]">
        <Link
          to={`/problem/${p.id}`}
          className={cx(
            'truncate text-[14.5px] leading-[1.35] no-underline',
            p.done ? 'text-muted line-through' : 'font-medium text-ink hover:text-primary',
          )}
          title={p.title}
        >
          {p.title}
        </Link>
        <LabelChips labels={p.labels} />
        {p.done && <DoneBadge />}
      </div>

      <span className="truncate text-[13px] leading-none text-secondary" title={author}>{author}</span>
      <span className="whitespace-nowrap text-[13px] leading-none text-muted">{relativeAge(p.created_at)}</span>

      <span className="flex items-center justify-center gap-[5px] text-[13px] leading-none text-secondary">
        {p.attachment_count > 0 ? (
          <>
            <PaperclipIcon className="text-muted" />
            {p.attachment_count}
          </>
        ) : (
          <span className="text-ghost">—</span>
        )}
      </span>

      <span className="flex justify-end">
        <MeetingBadge count={p.meeting_count} />
      </span>
    </div>
  )
}

/** The round done toggle from the design. */
export function DoneToggle({ done, onToggle }: { done: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      title={done ? cs.bucket.markOpen : cs.bucket.markDone}
      aria-label={done ? cs.bucket.markOpen : cs.bucket.markDone}
      aria-pressed={done}
      className={cx(
        'flex h-5 w-5 items-center justify-center rounded-full p-0 transition-colors focus-ring',
        done
          ? 'border border-done bg-done text-white'
          : 'border-[1.5px] border-check-round bg-white hover:border-done hover:bg-done-tint',
      )}
    >
      {done && (
        <svg width={12} height={12} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          <path d="m5 13 4 4L19 7" />
        </svg>
      )}
    </button>
  )
}

function SegmentedFilter({ value, onChange }: { value: DoneFilter; onChange: (v: DoneFilter) => void }) {
  const options: Array<[DoneFilter, string]> = [
    ['open', cs.bucket.filters.open],
    ['done', cs.bucket.filters.done],
    ['all', cs.bucket.filters.all],
  ]
  return (
    <div className="flex gap-[2px] rounded-[7px] bg-surface-track p-[3px]">
      {options.map(([key, text]) => (
        <button
          key={key}
          type="button"
          onClick={() => onChange(key)}
          aria-pressed={value === key}
          className={cx(
            'rounded-[5px] px-3 py-[6px] text-[13px] leading-none transition-colors focus-ring',
            value === key
              ? 'bg-white font-medium text-ink shadow-[0_1px_2px_rgb(21_24_28/0.1)]'
              : 'bg-transparent text-secondary hover:text-ink',
          )}
        >
          {text}
        </button>
      ))}
    </div>
  )
}

export function SortSelect({
  id,
  value,
  onChange,
}: {
  id?: string
  value: SortKey
  onChange: (v: SortKey) => void
}) {
  return (
    <select
      id={id}
      value={value}
      onChange={(e) => onChange(e.target.value as SortKey)}
      className={selectCompact}
    >
      <option value="created_desc">{cs.bucket.sorts.created_desc}</option>
      <option value="created_asc">{cs.bucket.sorts.created_asc}</option>
      <option value="meetings_desc">{cs.bucket.sorts.meetings_desc}</option>
    </select>
  )
}
