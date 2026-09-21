import { useEffect, useId, useRef, useState, type ClipboardEvent, type DragEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCreateProblem, useUpdateProblem } from '../../api/hooks'
import { ApiError } from '../../api/client'
import type { ProblemDetail } from '../../api/types'
import { cs } from '../../i18n/cs'
import { formatSize } from '../../lib/format'
import { Drawer, ErrorBox, FieldError, btn, cx, input, label, textarea } from '../../components/ui'
import { FileIcon, ImageIcon, VideoIcon, CloseIcon } from '../../components/Icons'

// The three caps, mirrored from the server so the form can explain a rejection
// before spending minutes uploading. The server enforces them too and its
// message wins if one ever slips through.
const MAX_FILE_BYTES = 104_857_600 // 100 MB
const MAX_FILES = 20
const MAX_REQUEST_BYTES = 536_870_912 // 512 MB
const MAX_TITLE = 200

type Staged = { id: string; file: File }

// The staged id only has to be unique inside this list (React key, removal).
// A counter does that everywhere; crypto.randomUUID() is missing outside a
// secure context, so over plain HTTP it would throw on the first attachment.
let stagedSeq = 0
const stagedId = (file: File) => `${file.name}-${file.size}-${++stagedSeq}`

/**
 * The problem form (A4), used for both create and edit.
 *
 * Everything is saved in one request: the problem and its files go together, so
 * a cancelled or rejected submit leaves nothing behind (FR-P1). There is no
 * "save first, then attach" step — screenshots arrive by Ctrl+V and must not
 * require the problem to exist.
 */
export function ProblemFormDrawer({
  open,
  onClose,
  existing,
}: {
  open: boolean
  onClose: () => void
  existing?: ProblemDetail
}) {
  const navigate = useNavigate()
  const create = useCreateProblem()
  const update = useUpdateProblem(existing?.id ?? 0)
  const mutation = existing ? update : create

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [link, setLink] = useState('')
  const [files, setFiles] = useState<Staged[]>([])
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [capError, setCapError] = useState<{ title: string; hint: string } | null>(null)
  const [dragging, setDragging] = useState(false)

  const fileInput = useRef<HTMLInputElement>(null)
  const ids = useId()

  // Reset when the drawer opens, so a cancelled edit never leaks into the next.
  useEffect(() => {
    if (!open) return
    setTitle(existing?.title ?? '')
    setDescription(existing?.description ?? '')
    setLink(existing?.link ?? '')
    setFiles([])
    setFieldErrors({})
    setCapError(null)
    mutation.reset()
    // `mutation` is recreated on every render; depending on it would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, existing])

  // Ctrl+V anywhere in the drawer attaches the clipboard image. This is the
  // dominant way screenshots arrive, so it is bound to the panel rather than to
  // a focused field.
  function onPaste(e: ClipboardEvent) {
    const pasted = Array.from(e.clipboardData.files)
    if (pasted.length > 0) {
      e.preventDefault()
      addFiles(pasted)
    }
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragging(false)
    addFiles(Array.from(e.dataTransfer.files))
  }

  function addFiles(incoming: File[]) {
    if (incoming.length === 0) return
    setCapError(null)
    setFiles((prev) => [
      ...prev,
      ...incoming.map((file) => ({ id: stagedId(file), file })),
    ])
  }

  function removeFile(id: string) {
    setCapError(null)
    setFiles((prev) => prev.filter((f) => f.id !== id))
  }

  /** Checks the three caps. Each fails for a different reason, so each says so. */
  function checkCaps(): { title: string; hint: string } | null {
    const oversized = files.find((f) => f.file.size > MAX_FILE_BYTES)
    if (oversized) {
      return {
        title: cs.errors.fileTooLarge(oversized.file.name, formatSize(oversized.file.size)),
        hint: cs.errors.fileTooLargeHint,
      }
    }
    if (files.length > MAX_FILES) {
      return {
        title: cs.errors.tooManyFiles(files.length, MAX_FILES),
        hint: cs.errors.tooManyFilesHint(files.length - MAX_FILES),
      }
    }
    const total = files.reduce((sum, f) => sum + f.file.size, 0)
    if (total > MAX_REQUEST_BYTES) {
      return {
        title: cs.errors.requestTooLarge(formatSize(total), formatSize(MAX_REQUEST_BYTES)),
        hint: cs.errors.requestTooLargeHint,
      }
    }
    return null
  }

  function submit() {
    const errors: Record<string, string> = {}
    const trimmedTitle = title.trim()
    if (!trimmedTitle) errors.title = cs.errors.titleRequired
    else if (trimmedTitle.length > MAX_TITLE) errors.title = cs.errors.titleTooLong

    const trimmedLink = link.trim()
    if (trimmedLink && !/^https?:\/\//i.test(trimmedLink)) errors.link = cs.errors.linkScheme

    setFieldErrors(errors)
    if (Object.keys(errors).length > 0) return

    const cap = checkCaps()
    setCapError(cap)
    if (cap) return

    const body = new FormData()
    body.set('title', trimmedTitle)
    body.set('description', description)
    body.set('link', trimmedLink)
    for (const f of files) body.append('file', f.file)

    mutation.mutate(body as never, {
      onSuccess: (created) => {
        onClose()
        // A newly created problem opens straight away; an edit stays put.
        if (!existing && created) navigate(`/problem/${(created as ProblemDetail).id}`)
      },
    })
  }

  const serverError = mutation.error instanceof ApiError ? mutation.error : null
  const totalBytes = files.reduce((sum, f) => sum + f.file.size, 0)

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={existing ? cs.form.editTitle : cs.form.createTitle}
      footer={
        <>
          <span className="max-w-[230px] text-[11.5px] leading-[1.4] text-muted">{cs.form.atomicHint}</span>
          <span className="flex gap-[9px]">
            <button type="button" onClick={onClose} className={btn.secondary}>
              {cs.form.cancel}
            </button>
            <button type="button" onClick={submit} className={btn.primary} disabled={mutation.isPending}>
              {mutation.isPending ? cs.form.saving : cs.form.save}
            </button>
          </span>
        </>
      }
    >
      <div className="flex flex-col gap-[18px]" onPaste={onPaste}>
        {/* Title */}
        <div className="flex flex-col gap-[6px]">
          <div className="flex items-baseline justify-between">
            <label className={label} htmlFor={`${ids}-title`}>
              {cs.form.title} <span className="text-danger-strong">*</span>
            </label>
            <span className={cx('font-mono text-[11.5px] leading-none', title.length > MAX_TITLE ? 'text-danger' : 'text-faint')}>
              {title.length} / {MAX_TITLE}
            </span>
          </div>
          <input
            id={`${ids}-title`}
            className={cx(input, fieldErrors.title && 'border-danger-strong bg-danger-bg')}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            aria-invalid={Boolean(fieldErrors.title)}
          />
          {fieldErrors.title && <FieldError>{fieldErrors.title}</FieldError>}
        </div>

        {/* Description */}
        <div className="flex flex-col gap-[6px]">
          <label className={label} htmlFor={`${ids}-description`}>
            {cs.form.description}
          </label>
          <textarea
            id={`${ids}-description`}
            className={cx(textarea, 'h-[104px]')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <span className="text-[11.5px] leading-none text-faint">{cs.form.descriptionHint}</span>
        </div>

        {/* Link */}
        <div className="flex flex-col gap-[6px]">
          <label className={label} htmlFor={`${ids}-link`}>
            {cs.form.link} <span className="font-normal text-faint">{cs.form.optional}</span>
          </label>
          <input
            id={`${ids}-link`}
            className={cx(input, 'font-mono text-[13px]', fieldErrors.link && 'border-danger-strong bg-danger-bg')}
            value={link}
            onChange={(e) => setLink(e.target.value)}
            placeholder="https://"
            aria-invalid={Boolean(fieldErrors.link)}
          />
          {fieldErrors.link && <FieldError>{fieldErrors.link}</FieldError>}
        </div>

        {/* Attachments */}
        <div className="flex flex-col gap-[9px]">
          <label className={label}>{cs.form.attachments}</label>

          <div
            onDragOver={(e) => {
              e.preventDefault()
              setDragging(true)
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={onDrop}
            className={cx(
              'rounded-[8px] border-[1.5px] border-dashed p-[18px] text-center transition-colors',
              dragging ? 'border-primary bg-primary-tint' : 'border-check bg-surface-sunken',
            )}
          >
            <div className="text-[13.5px] font-medium leading-[1.4] text-ink">
              {dragging ? cs.form.dropActive : cs.form.dropzone}
            </div>
            <div className="mt-[5px] text-[11.5px] leading-[1.5] text-muted">{cs.form.dropzoneHint}</div>
            <button
              type="button"
              onClick={() => fileInput.current?.click()}
              className="mt-[11px] h-[29px] rounded-[6px] border border-field bg-white px-[11px] text-[12.5px] leading-none text-ink hover:bg-surface-hover focus-ring"
            >
              {cs.form.pick}
            </button>
            <input
              ref={fileInput}
              type="file"
              multiple
              className="hidden"
              onChange={(e) => {
                addFiles(Array.from(e.target.files ?? []))
                e.target.value = ''
              }}
            />
          </div>

          {files.length > 0 && (
            <div className="flex flex-col gap-[7px]">
              {files.map(({ id, file }) => (
                <StagedFileRow
                  key={id}
                  file={file}
                  over={file.size > MAX_FILE_BYTES}
                  onRemove={() => removeFile(id)}
                />
              ))}
              <div className="flex justify-between px-1 text-[11.5px] leading-none text-faint">
                <span>
                  {files.length} / {MAX_FILES}
                </span>
                <span className={cx('font-mono', totalBytes > MAX_REQUEST_BYTES && 'text-danger')}>
                  {formatSize(totalBytes)} / {formatSize(MAX_REQUEST_BYTES)}
                </span>
              </div>
            </div>
          )}
        </div>

        {capError && <ErrorBox title={capError.title} hint={capError.hint} />}
        {serverError && <ErrorBox title={serverError.message} hint={cs.errors.fileTooLargeHint} />}
      </div>
    </Drawer>
  )
}

function StagedFileRow({ file, over, onRemove }: { file: File; over: boolean; onRemove: () => void }) {
  const isImage = file.type.startsWith('image/') && file.type !== 'image/svg+xml'
  const isVideo = file.type.startsWith('video/')

  return (
    <div className={cx('flex items-center gap-[11px] rounded-[7px] border p-2', over ? 'border-danger-box bg-danger-bg' : 'border-line')}>
      <span className="flex h-10 w-10 flex-none items-center justify-center rounded-[4px] bg-surface-tile text-muted">
        {isImage ? <ImageIcon /> : isVideo ? <VideoIcon /> : <FileIcon size={16} />}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13px] leading-[1.3] text-ink" title={file.name}>
          {file.name}
        </span>
        <span className={cx('mt-[3px] block text-[11.5px] leading-none', over ? 'font-mono text-danger' : 'text-muted')}>
          {formatSize(file.size)}
        </span>
      </span>
      <button
        type="button"
        onClick={onRemove}
        aria-label={`${cs.form.remove}: ${file.name}`}
        className={cx(btn.icon, 'h-[26px] w-[26px] flex-none')}
      >
        <CloseIcon size={14} />
      </button>
    </div>
  )
}
