import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useDeleteAttachment, useDeleteProblem, useDisplayName, useProblem, useSetDone } from '../../api/hooks'
import { ApiError } from '../../api/client'
import type { ProblemMeetingRef, User } from '../../api/types'
import { cs } from '../../i18n/cs'
import { formatDate, safeLink, displayLink, weekLabel } from '../../lib/format'
import { ChevronLeftIcon, CheckIcon, ExternalLinkIcon } from '../../components/Icons'
import { AttachmentGrid } from '../../components/Attachments'
import { DeleteProblemDialog } from './DeleteProblemDialog'
import { ProblemFormDrawer } from './ProblemForm'
import { NotFound } from '../../components/NotFoundPage'
import { DoneBadge, LabelChips, Spinner, btn, cx, sectionLabel } from '../../components/ui'

/** The problem detail (A5): every field, the gallery, and the meeting timeline. */
export function ProblemDetailPage({ user }: { user: User | null }) {
  const { id: rawId } = useParams()
  const id = Number(rawId)
  const navigate = useNavigate()

  const { data: problem, isPending, error } = useProblem(id)
  const setDone = useSetDone()
  const deleteProblem = useDeleteProblem()
  const deleteAttachment = useDeleteAttachment()
  const displayName = useDisplayName()

  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)

  if (isPending) return <Spinner />
  if (error instanceof ApiError && error.isNotFound) {
    return <NotFound detail={cs.errors.notFoundProblem(rawId ?? '')} to="/" linkLabel={cs.errors.toProblems} />
  }
  if (!problem) return <NotFound to="/" linkLabel={cs.errors.toProblems} />

  const isAdmin = user?.role === 'admin'
  const link = safeLink(problem.link)

  return (
    <div className="px-8 py-6">
      <div className="max-w-[1000px]">
        <Link
          to="/"
          className="inline-flex items-center gap-[6px] text-[13px] leading-none text-secondary no-underline hover:text-primary"
        >
          <ChevronLeftIcon />
          {cs.problem.back}
        </Link>

        <div className="mt-3 flex items-start justify-between gap-8">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-[10px]">
              <h1
                className={cx(
                  'm-0 text-[23px] leading-[1.3] tracking-[-0.01em]',
                  problem.done ? 'font-normal text-muted line-through' : 'font-semibold',
                )}
              >
                {problem.title}
              </h1>
              <LabelChips labels={problem.labels} />
              {problem.done && <DoneBadge />}
            </div>
            <div className="mt-2 text-[13px] leading-none text-muted">
              {cs.problem.createdBy} <span className="text-secondary">{displayName(problem.created_by)}</span>
              {' · '}
              {formatDate(problem.created_at)}
              {problem.updated_at !== problem.created_at && (
                <> · {cs.problem.updatedAt} {formatDate(problem.updated_at)}</>
              )}
            </div>
          </div>

          {user && (
            <div className="flex flex-none items-center gap-2">
              {/* Only the admin may flip done; both editors may edit. */}
              {isAdmin && (
                <button
                  type="button"
                  onClick={() => setDone.mutate({ id: problem.id, done: !problem.done })}
                  className={problem.done ? btn.secondary : btn.done}
                >
                  {!problem.done && <CheckIcon size={14} />}
                  {problem.done ? cs.problem.markOpen : cs.problem.markDone}
                </button>
              )}
              <button type="button" onClick={() => setEditing(true)} className={btn.secondary}>
                {cs.problem.edit}
              </button>
              {isAdmin && (
                <button type="button" onClick={() => setConfirming(true)} className={btn.dangerOutline}>
                  {cs.problem.delete}
                </button>
              )}
            </div>
          )}
        </div>

        <div className="mt-[26px] grid items-start gap-11 lg:grid-cols-[minmax(0,1fr)_360px]">
          <div>
            {problem.description && (
              // Line breaks are preserved; the field is plain text by decision,
              // so nothing here interprets markup.
              <p className="m-0 whitespace-pre-line text-[14.5px] leading-[1.7] text-prose">{problem.description}</p>
            )}

            {link && (
              <a
                href={link}
                target="_blank"
                rel="noreferrer noopener"
                className="mt-4 inline-flex items-center gap-[7px] font-mono text-[13.5px] leading-none text-primary no-underline hover:underline"
              >
                <ExternalLinkIcon />
                {displayLink(link)}
              </a>
            )}

            <div className="mt-7">
              <div className="flex items-center justify-between">
                <span className={sectionLabel}>
                  {cs.problem.attachments}
                  {problem.attachments.length > 0 && ` — ${problem.attachments.length}`}
                </span>
                {user && (
                  <button
                    type="button"
                    onClick={() => setEditing(true)}
                    className="h-[29px] rounded-[6px] border border-field bg-white px-[11px] text-[12.5px] leading-none text-ink hover:bg-surface-hover focus-ring"
                  >
                    {cs.problem.addAttachments}
                  </button>
                )}
              </div>

              {problem.attachments.length > 0 ? (
                <div className="mt-3">
                  <AttachmentGrid
                    attachments={problem.attachments}
                    // This is the only screen where a saved attachment can be
                    // removed, so a wrongly pasted screenshot has nowhere else
                    // to go.
                    onRemove={user ? (a) => deleteAttachment.mutate(a.id) : undefined}
                  />
                </div>
              ) : (
                <div className="mt-3 text-[13px] text-muted">Žádné přílohy.</div>
              )}
            </div>
          </div>

          <Timeline entries={problem.meetings} />
        </div>
      </div>

      {user && (
        <ProblemFormDrawer open={editing} onClose={() => setEditing(false)} existing={problem} />
      )}
      {isAdmin && (
        <DeleteProblemDialog
          problem={problem}
          open={confirming}
          onClose={() => setConfirming(false)}
          onConfirm={() =>
            deleteProblem.mutate(problem.id, {
              onSuccess: () => navigate('/'),
            })
          }
          pending={deleteProblem.isPending}
        />
      )}
    </div>
  )
}

/**
 * The cross-meeting timeline — the payoff of storing the notes per agenda
 * entry rather than per problem.
 *
 * A problem on its fourth meeting shows four entries, and reading down the
 * column tells the story of what was said each week and what came of it.
 */
function Timeline({ entries }: { entries: ProblemMeetingRef[] }) {
  if (entries.length === 0) {
    return (
      <div>
        <div className={sectionLabel}>{cs.problem.onMeetings}</div>
        <div className="mt-3 text-[13px] leading-[1.5] text-muted">{cs.problem.timelineEmpty}</div>
      </div>
    )
  }

  return (
    <div>
      <div className={sectionLabel}>
        {cs.problem.onMeetings} — {entries.length}×
      </div>
      <div className="mt-[14px] flex flex-col">
        {entries.map((entry, i) => {
          const last = i === entries.length - 1
          return (
            <div key={entry.item_id} className="grid grid-cols-[14px_minmax(0,1fr)] gap-[14px]">
              <div className="flex flex-col items-center">
                {/* The most recent appearance is the live one. */}
                <span className={cx('mt-[5px] block h-[9px] w-[9px] rounded-full', last ? 'bg-primary' : 'bg-check')} />
                {!last && <span className="block w-px flex-1 bg-line" />}
              </div>
              <div className={last ? '' : 'pb-[22px]'}>
                <div className="flex flex-wrap items-center gap-[9px]">
                  <Link
                    to={`/porada/${entry.slug}`}
                    className="text-[13.5px] font-medium leading-none text-primary no-underline hover:underline"
                  >
                    {weekLabel(entry.iso_week)}
                  </Link>
                  <span className="text-[12.5px] leading-none text-muted">{formatDate(entry.meeting_date)}</span>
                  <span
                    className={cx(
                      'rounded-[4px] px-[6px] py-[2px] font-mono text-[11px] font-medium leading-[1.4]',
                      last ? 'bg-primary-chip text-primary-ink' : 'bg-surface-track text-secondary',
                    )}
                  >
                    {cs.problem.itemNumber} {entry.position + 1}
                  </span>
                </div>

                {entry.prep_note && (
                  <div className="mt-[9px] whitespace-pre-line text-[13.5px] leading-[1.6] text-prose">
                    {entry.prep_note}
                  </div>
                )}

                {/* The action note is often empty; that case must not look broken. */}
                {entry.action_note ? (
                  <div className="mt-[9px] border-l-2 border-rule-strong pl-[11px] text-[13px] leading-[1.6] text-secondary">
                    <span className="mb-1 block text-[11px] font-medium uppercase leading-none tracking-[0.06em] text-faint">
                      {cs.problem.noAction.split(' — ')[0]}
                    </span>
                    <span className="whitespace-pre-line">{entry.action_note}</span>
                  </div>
                ) : (
                  <div className="mt-[9px] text-[12.5px] leading-[1.5] text-faint">{cs.problem.noAction}</div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
