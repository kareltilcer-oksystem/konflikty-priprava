import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCreateMeeting } from '../../api/hooks'
import { ApiError } from '../../api/client'
import { cs } from '../../i18n/cs'
import { previewSlug } from '../../lib/format'
import { ErrorBox, Modal, btn, input, label, textarea } from '../../components/ui'

/** Suggests the coming Friday — the weekly slot the meeting actually occupies. */
function nextFriday(): string {
  const d = new Date()
  const delta = (5 - d.getDay() + 7) % 7 || 7
  d.setDate(d.getDate() + delta)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/**
 * The new-meeting dialog (B2).
 *
 * The date is the only required input. The URL slug is derived server-side and
 * is never an input field — it is shown as a preview so the admin can see the
 * link the meeting will live at.
 */
export function NewMeetingDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate()
  const create = useCreateMeeting()
  const [date, setDate] = useState(nextFriday())
  const [note, setNote] = useState('')

  useEffect(() => {
    if (!open) return
    setDate(nextFriday())
    setNote('')
    create.reset()
    // create is recreated each render; depending on it would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  function submit(e: FormEvent) {
    e.preventDefault()
    create.mutate(
      { meeting_date: date, note },
      {
        onSuccess: (meeting) => {
          onClose()
          navigate(`/porada/${meeting.slug}`)
        },
      },
    )
  }

  const slug = previewSlug(date)
  const message = create.error instanceof ApiError ? create.error.message : null

  return (
    <Modal open={open} onClose={onClose} width={436} labelledBy="new-meeting-title">
      <form onSubmit={submit} className="p-[22px]">
        <div id="new-meeting-title" className="text-[16.5px] font-semibold leading-none">
          {cs.newMeeting.title}
        </div>

        <div className="mt-[18px] flex flex-col gap-[14px]">
          <div className="flex flex-col gap-[6px]">
            <label className={label} htmlFor="meeting-date">
              {cs.newMeeting.date}
            </label>
            <input
              id="meeting-date"
              type="date"
              className={input}
              value={date}
              onChange={(e) => setDate(e.target.value)}
              required
            />
            {slug && (
              <span className="text-[11.5px] leading-[1.5] text-faint">
                {cs.newMeeting.slugHint} <span className="font-mono">/porada/{slug}</span>
              </span>
            )}
          </div>

          <div className="flex flex-col gap-[6px]">
            <label className={label} htmlFor="meeting-note">
              {cs.newMeeting.note} <span className="font-normal text-faint">{cs.form.optional}</span>
            </label>
            <textarea
              id="meeting-note"
              className={`${textarea} h-[72px]`}
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder={cs.newMeeting.notePlaceholder}
            />
          </div>
        </div>

        {message && (
          <div className="mt-[14px]">
            <ErrorBox title={message} />
          </div>
        )}

        <div className="mt-5 flex justify-end gap-[9px]">
          <button type="button" onClick={onClose} className={btn.secondary}>
            {cs.newMeeting.cancel}
          </button>
          <button type="submit" className={btn.primary} disabled={create.isPending}>
            {cs.newMeeting.submit}
          </button>
        </div>
      </form>
    </Modal>
  )
}
