import type { ProblemDetail } from '../../api/types'
import { cs } from '../../i18n/cs'
import { formatDate, weekLabel } from '../../lib/format'
import { Modal, btn } from '../../components/ui'

/**
 * Confirms deleting a problem.
 *
 * This is the one confirmation in the app that carries real information, so it
 * is designed as such rather than as a generic "Opravdu?": deleting a problem
 * strips it from every past agenda it appeared on, taking that week's prep and
 * action notes with it, and the admin cannot know which meetings those are
 * without being told.
 */
export function DeleteProblemDialog({
  problem,
  open,
  onClose,
  onConfirm,
  pending,
}: {
  problem: ProblemDetail
  open: boolean
  onClose: () => void
  onConfirm: () => void
  pending: boolean
}) {
  const meetings = problem.meetings
  const attachments = problem.attachments.length

  return (
    <Modal open={open} onClose={onClose} width={468} labelledBy="delete-problem-title">
      <div className="p-[22px]">
        <div id="delete-problem-title" className="text-[16.5px] font-semibold leading-[1.35]">
          {cs.confirm.deleteProblemTitle(problem.title)}
        </div>

        {meetings.length > 0 ? (
          <>
            <div className="mt-[11px] text-[13.5px] leading-[1.6] text-secondary">
              {cs.confirm.deleteProblemBody(meetings.length)}
            </div>
            <div className="mt-3 overflow-hidden rounded-[7px] border border-line">
              {meetings.map((m, i) => (
                <div
                  key={m.item_id}
                  className={`flex items-center justify-between px-3 py-[9px] ${
                    i < meetings.length - 1 ? 'border-b border-rule' : ''
                  }`}
                >
                  <span className="text-[13px] font-medium leading-none">{weekLabel(m.iso_week)}</span>
                  <span className="text-[12.5px] leading-none text-muted">
                    {formatDate(m.meeting_date)} · {cs.problem.itemNumber} {m.position + 1}
                  </span>
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className="mt-[11px] text-[13.5px] leading-[1.6] text-secondary">
            {cs.confirm.deleteProblemNoMeetings}
          </div>
        )}

        <div className="mt-[11px] text-[12.5px] leading-[1.5] text-muted">
          {cs.confirm.deleteProblemFinal(attachments)}
        </div>

        <div className="mt-5 flex justify-end gap-[9px]">
          <button type="button" onClick={onClose} className={btn.secondary}>
            {cs.confirm.cancel}
          </button>
          <button type="button" onClick={onConfirm} className={btn.danger} disabled={pending}>
            {cs.confirm.deleteProblemConfirm}
          </button>
        </div>
      </div>
    </Modal>
  )
}
