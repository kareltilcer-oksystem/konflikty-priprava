import type { MeetingDetail } from '../../api/types'
import { cs } from '../../i18n/cs'
import { formatDate, weekLabel } from '../../lib/format'
import { Modal, btn } from '../../components/ui'

/**
 * Confirms deleting a meeting.
 *
 * It says explicitly that the problems survive and return to the bucket —
 * without that, the admin hesitates over whether deleting a meeting also throws
 * away the work it was about.
 */
export function DeleteMeetingDialog({
  meeting,
  open,
  onClose,
  onConfirm,
  pending,
}: {
  meeting: MeetingDetail
  open: boolean
  onClose: () => void
  onConfirm: () => void
  pending: boolean
}) {
  return (
    <Modal open={open} onClose={onClose} width={468} labelledBy="delete-meeting-title">
      <div className="p-[22px]">
        <div id="delete-meeting-title" className="text-[16.5px] font-semibold leading-[1.35]">
          {cs.confirm.deleteMeetingTitle(formatDate(meeting.meeting_date), weekLabel(meeting.iso_week))}
        </div>

        <div className="mt-[11px] text-[13.5px] leading-[1.6] text-secondary">
          {cs.confirm.deleteMeetingBody(meeting.items.length)}{' '}
          <span className="text-ink">{cs.confirm.deleteMeetingEmphasis}</span>
          {cs.confirm.deleteMeetingTail}
        </div>

        <div className="mt-[11px] text-[12.5px] leading-[1.5] text-muted">
          {cs.confirm.deleteMeetingLink(meeting.slug)}
        </div>

        <div className="mt-5 flex justify-end gap-[9px]">
          <button type="button" onClick={onClose} className={btn.secondary}>
            {cs.confirm.cancel}
          </button>
          <button type="button" onClick={onConfirm} className={btn.danger} disabled={pending}>
            {cs.confirm.deleteMeetingConfirm}
          </button>
        </div>
      </div>
    </Modal>
  )
}
