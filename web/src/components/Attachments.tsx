import { useCallback, useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Attachment } from '../api/types'
import { attachmentKind, formatSize } from '../lib/format'
import { cs } from '../i18n/cs'
import { cx } from './ui'
import { ChevronLeftIcon, ChevronRightIcon, CloseIcon, DownloadIcon, FileIcon, PlayIcon, CloseIcon as XIcon } from './Icons'

/**
 * The attachment gallery.
 *
 * Three renderings of one grid: images open in a lightbox, videos play inline,
 * and everything else — SVG included — is a download tile. The SVG carve-out is
 * deliberate and matches what the API will serve: it is a scriptable document,
 * never a picture.
 */
export function AttachmentGrid({
  attachments,
  onRemove,
  columns = 3,
}: {
  attachments: Attachment[]
  onRemove?: (a: Attachment) => void
  columns?: number
}) {
  const [lightbox, setLightbox] = useState<number | null>(null)
  if (attachments.length === 0) return null

  // Only images and videos take part in the lightbox's prev/next.
  const viewable = attachments.filter((a) => attachmentKind(a.content_type) !== 'file')

  return (
    <>
      <div
        className="grid gap-3"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {attachments.map((a) => (
          <AttachmentTile
            key={a.id}
            attachment={a}
            onOpen={() => {
              const i = viewable.findIndex((v) => v.id === a.id)
              if (i >= 0) setLightbox(i)
            }}
            onRemove={onRemove ? () => onRemove(a) : undefined}
          />
        ))}
      </div>

      {lightbox !== null && viewable.length > 0 && (
        <Lightbox
          items={viewable}
          index={lightbox}
          onIndex={setLightbox}
          onClose={() => setLightbox(null)}
        />
      )}
    </>
  )
}

function AttachmentTile({
  attachment: a,
  onOpen,
  onRemove,
}: {
  attachment: Attachment
  onOpen: () => void
  onRemove?: () => void
}) {
  const kind = attachmentKind(a.content_type)
  const caption = `${a.filename} · ${formatSize(a.size_bytes)}`

  return (
    <div className="relative overflow-hidden rounded-[7px] border border-line print-item">
      {kind === 'image' && (
        <button
          type="button"
          onClick={onOpen}
          className="flex h-[128px] w-full items-center justify-center bg-surface-tile focus-ring"
          aria-label={a.filename}
        >
          <img src={a.url} alt={a.filename} className="h-full w-full object-cover" loading="lazy" />
        </button>
      )}

      {kind === 'video' && (
        // Metadata only: an agenda with six videos must not pull six files.
        <video src={a.url} controls preload="metadata" className="h-[128px] w-full bg-video object-contain" />
      )}

      {kind === 'file' && (
        <a
          href={`${a.url}?download=1`}
          download={a.filename}
          className="flex h-[128px] flex-col items-center justify-center gap-[9px] bg-surface-sunken text-muted no-underline focus-ring"
        >
          <FileIcon size={26} />
          <span className="inline-flex items-center gap-[6px] text-[12px] text-primary">
            <DownloadIcon />
            {cs.problem.download}
          </span>
        </a>
      )}

      <div
        className="truncate border-t border-line px-[9px] py-[7px] text-[11.5px] leading-[1.3] text-secondary"
        title={caption}
      >
        {caption}
      </div>

      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          title={cs.problem.removeAttachment}
          aria-label={`${cs.problem.removeAttachment}: ${a.filename}`}
          className={cx(
            'print-hide absolute right-[7px] top-[7px] flex h-[24px] w-[24px] items-center justify-center rounded-[5px] focus-ring',
            kind === 'file' ? 'bg-surface-tile text-secondary' : 'bg-[rgb(21_24_28/0.6)] text-white',
          )}
        >
          <XIcon size={13} />
        </button>
      )}
    </div>
  )
}

/** The image and video lightbox. */
function Lightbox({
  items,
  index,
  onIndex,
  onClose,
}: {
  items: Attachment[]
  index: number
  onIndex: (i: number) => void
  onClose: () => void
}) {
  const current = items[index]

  const go = useCallback(
    (delta: number) => onIndex((index + delta + items.length) % items.length),
    [index, items.length, onIndex],
  )

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      if (e.key === 'ArrowLeft') go(-1)
      if (e.key === 'ArrowRight') go(1)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [go, onClose])

  if (!current) return null
  const kind = attachmentKind(current.content_type)

  return createPortal(
    <div
      className="print-hide fixed inset-0 z-[60] flex flex-col bg-ink"
      role="dialog"
      aria-modal="true"
      aria-label={current.filename}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className="flex flex-none items-center justify-between px-[18px] py-[14px]">
        <div className="min-w-0">
          <div className="truncate text-[13.5px] font-medium leading-none text-white">{current.filename}</div>
          <div className="mt-[6px] text-[11.5px] leading-none text-white/55">
            {formatSize(current.size_bytes)} · {cs.lightbox.counter(index + 1, items.length)}
          </div>
        </div>
        <div className="flex flex-none items-center gap-[9px]">
          <a
            href={`${current.url}?download=1`}
            download={current.filename}
            className="inline-flex h-[30px] items-center gap-[6px] rounded-[6px] border border-white/28 px-[11px] text-[12.5px] leading-none text-white no-underline hover:bg-white/10 focus-ring"
          >
            <DownloadIcon />
            {cs.lightbox.download}
          </a>
          <button
            type="button"
            onClick={onClose}
            aria-label={cs.lightbox.close}
            className="flex h-[30px] w-[30px] items-center justify-center rounded-[6px] border border-white/28 text-white hover:bg-white/10 focus-ring"
          >
            <CloseIcon size={15} />
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 items-center gap-[18px] px-[18px] pb-[18px]">
        {items.length > 1 && (
          <LightboxNav label={cs.lightbox.prev} onClick={() => go(-1)}>
            <ChevronLeftIcon size={18} />
          </LightboxNav>
        )}

        <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden rounded-[4px] bg-viewport">
          {kind === 'image' ? (
            <img src={current.url} alt={current.filename} className="max-h-full max-w-full object-contain" />
          ) : (
            <video src={current.url} controls autoPlay className="max-h-full max-w-full" />
          )}
        </div>

        {items.length > 1 && (
          <LightboxNav label={cs.lightbox.next} onClick={() => go(1)}>
            <ChevronRightIcon size={18} />
          </LightboxNav>
        )}
      </div>
    </div>,
    document.body,
  )
}

function LightboxNav({
  label,
  onClick,
  children,
}: {
  label: string
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="flex h-[38px] w-[38px] flex-none items-center justify-center rounded-full border border-white/28 text-white hover:bg-white/10 focus-ring"
    >
      {children}
    </button>
  )
}

/**
 * The compact attachment strip on an agenda item.
 *
 * The agenda is read, not browsed, so the tiles are smaller here and a video is
 * a poster rather than a player — the admin is looking at the whole programme.
 */
export function AttachmentStrip({ attachments }: { attachments: Attachment[] }) {
  if (attachments.length === 0) return null
  return (
    <div className="mt-[11px] flex flex-wrap gap-2">
      {attachments.map((a) => {
        const kind = attachmentKind(a.content_type)
        return (
          <a
            key={a.id}
            href={kind === 'file' ? `${a.url}?download=1` : a.url}
            target={kind === 'file' ? undefined : '_blank'}
            rel="noreferrer"
            download={kind === 'file' ? a.filename : undefined}
            className="w-[132px] overflow-hidden rounded-[6px] border border-line no-underline focus-ring"
            title={`${a.filename} · ${formatSize(a.size_bytes)}`}
          >
            {kind === 'image' && (
              <img src={a.url} alt={a.filename} className="h-[74px] w-full bg-surface-tile object-cover" loading="lazy" />
            )}
            {kind === 'video' && (
              <span className="flex h-[74px] w-full items-center justify-center bg-video">
                <span className="flex h-[26px] w-[26px] items-center justify-center rounded-full bg-white/90 text-video">
                  <PlayIcon />
                </span>
              </span>
            )}
            {kind === 'file' && (
              <span className="flex h-[74px] w-full items-center justify-center bg-surface-sunken text-muted">
                <FileIcon size={20} />
              </span>
            )}
            <span className="block truncate border-t border-line px-[7px] py-[5px] text-[10.5px] leading-[1.3] text-secondary">
              {a.filename}
            </span>
          </a>
        )
      })}
    </div>
  )
}

/** The button label under an attachment count, used by the bucket row. */
export function AttachmentCount({ count }: { count: number }) {
  if (count === 0) return <span className="text-ghost">—</span>
  return <>{count}</>
}
