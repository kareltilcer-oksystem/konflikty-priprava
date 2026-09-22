import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { CloseIcon } from './Icons'
import { cs } from '../i18n/cs'

/** Joins class names, dropping anything falsy. */
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}

// Button styles, lifted from design/v1.
const btnBase =
  'inline-flex items-center justify-center gap-[7px] rounded-[6px] font-sans transition-colors ' +
  'disabled:cursor-not-allowed focus-ring'

export const btn = {
  primary: cx(
    btnBase,
    'h-[34px] px-[14px] text-[13.5px] font-medium leading-none text-white',
    'bg-primary hover:bg-primary-hover disabled:bg-surface-tile disabled:text-faint',
  ),
  secondary: cx(
    btnBase,
    'h-[34px] px-[14px] text-[13.5px] leading-none text-ink',
    'border border-field bg-white hover:bg-surface-track',
  ),
  small: cx(
    btnBase,
    'h-[30px] px-[11px] text-[13px] leading-none text-ink',
    'border border-field bg-white hover:bg-surface-hover',
  ),
  tiny: cx(
    btnBase,
    'h-[24px] px-[8px] rounded-[5px] text-[12px] leading-none text-secondary',
    'border border-field bg-white hover:bg-surface-hover hover:text-ink',
  ),
  danger: cx(
    btnBase,
    'h-[34px] px-[16px] text-[13.5px] font-medium leading-none text-white',
    'bg-danger hover:bg-danger-hover',
  ),
  dangerOutline: cx(
    btnBase,
    'h-[34px] px-[13px] text-[13px] leading-none text-danger',
    'border border-danger-line bg-white hover:bg-danger-tint',
  ),
  done: cx(
    btnBase,
    'h-[34px] px-[13px] text-[13px] font-medium leading-none text-done',
    'border border-done-line bg-done-tint hover:bg-done-hover',
  ),
  icon: 'inline-flex items-center justify-center rounded-[6px] bg-transparent text-secondary transition-colors hover:bg-surface-track focus-ring',
}

export const input =
  'h-[36px] w-full rounded-[6px] border border-field bg-white px-[11px] text-[14px] text-ink outline-none transition-colors focus-ring'

export const textarea =
  'w-full rounded-[6px] border border-field bg-white px-[11px] py-[9px] text-[13.5px] leading-[1.6] text-ink outline-none transition-colors resize-none focus-ring'

/**
 * What every dropdown shares. Deliberately sets no size, padding, type scale or
 * background position: the two dropdowns differ on exactly those, and a base
 * that declared them would be overridden by whichever copy of the property the
 * generated stylesheet happened to emit last, not by the order written at the
 * call site.
 *
 * `select-chevron` (src/styles/index.css) carries both `appearance-none` — what
 * makes the control match the text fields — and the arrow that stripping the
 * native appearance takes away. They travel as one class so a `<select>` can
 * never be styled into having no dropdown affordance at all.
 */
const selectBase =
  'select-chevron cursor-pointer rounded-[6px] border border-field bg-white bg-[length:12px] text-ink outline-none transition-colors focus-ring'

/** A full-width form select matching `input`'s height. */
export const select = cx(selectBase, 'h-[36px] w-full bg-[right_10px_center] pl-[11px] pr-[32px] text-[14px]')

/** The shorter, auto-width select used in toolbars beside the filter buttons. */
export const selectCompact = cx(selectBase, 'h-[33px] bg-[right_9px_center] pl-[10px] pr-[30px] text-[13px]')

export const label = 'text-[12.5px] font-medium leading-none text-ink'

export const columnHeader =
  'text-[11.5px] font-medium uppercase leading-none tracking-[0.06em] text-muted'

export const sectionLabel =
  'text-[12px] font-medium uppercase leading-none tracking-[0.08em] text-muted'

export const microLabel =
  'text-[11px] font-medium uppercase leading-none tracking-[0.08em] text-muted'

/**
 * A modal dialog.
 *
 * Rendered through a portal so a dialog opened from deep inside the agenda is
 * never clipped by an ancestor's overflow, and closed on Escape because that is
 * what every other dialog on the machine does.
 */
export function Modal({
  open,
  onClose,
  children,
  width = 468,
  labelledBy,
}: {
  open: boolean
  onClose: () => void
  children: ReactNode
  width?: number
  labelledBy?: string
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    // Stop the page behind the scrim scrolling under the dialog.
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    // Move focus in, so the keyboard lands somewhere useful.
    ref.current?.querySelector<HTMLElement>('input, textarea, button')?.focus()
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
    }
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div
      className="print-hide fixed inset-0 z-50 flex items-center justify-center bg-[rgb(21_24_28/0.32)] p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        className="flex max-h-[min(760px,90vh)] w-full flex-col overflow-hidden rounded-[10px] bg-white shadow-[0_18px_48px_rgb(21_24_28/0.25)]"
        style={{ maxWidth: width }}
      >
        {children}
      </div>
    </div>,
    document.body,
  )
}

/** A right-hand drawer — what the problem form uses. */
export function Drawer({
  open,
  onClose,
  title,
  children,
  footer,
}: {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer: ReactNode
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    ref.current?.querySelector<HTMLElement>('input, textarea')?.focus()
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
    }
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div className="print-hide fixed inset-0 z-50">
      <div
        className="absolute inset-0 bg-[rgb(21_24_28/0.28)]"
        onMouseDown={onClose}
        aria-hidden
      />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="absolute inset-y-0 right-0 flex w-full max-w-[560px] flex-col bg-white shadow-[-8px_0_28px_rgb(21_24_28/0.14)]"
      >
        <div className="flex h-[60px] flex-none items-center justify-between border-b border-line px-[22px]">
          <span className="text-[16.5px] font-semibold leading-none">{title}</span>
          <button type="button" onClick={onClose} className={cx(btn.icon, 'h-[30px] w-[30px]')} aria-label={cs.form.cancel}>
            <CloseIcon />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-[22px] py-5">{children}</div>
        <div className="flex flex-none items-center justify-between gap-4 border-t border-line bg-surface-sunken px-[22px] py-[14px]">
          {footer}
        </div>
      </div>
    </div>,
    document.body,
  )
}

/** The red box used for a failed submit and a login failure. */
export function ErrorBox({ title, hint, children }: { title: string; hint?: string; children?: ReactNode }) {
  return (
    <div role="alert" className="rounded-[8px] border border-danger-box bg-danger-bg px-[15px] py-[13px]">
      <div className="text-[13.5px] font-medium leading-[1.4] text-danger-ink">{title}</div>
      {hint && <div className="mt-[7px] text-[12.5px] leading-[1.6] text-secondary">{hint}</div>}
      {children}
    </div>
  )
}

/** An inline field error, with the icon from the design. */
export function FieldError({ children }: { children: ReactNode }) {
  return (
    <span className="flex items-center gap-[6px] text-[12.5px] leading-[1.4] text-danger">
      <AlertGlyph />
      {children}
    </span>
  )
}

function AlertGlyph() {
  return (
    <svg width={13} height={13} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" aria-hidden>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5.5M12 16.2v.3" />
    </svg>
  )
}

/** A centred empty state. */
export function EmptyState({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }) {
  return (
    <div className="py-16 text-center">
      <div className="text-[15px] leading-[1.5] text-ink">{title}</div>
      {hint && <div className="mt-[6px] text-[13px] leading-[1.5] text-muted">{hint}</div>}
      {action && <div className="mt-3">{action}</div>}
    </div>
  )
}

export function Spinner({ children = cs.common.loading }: { children?: string }) {
  return <div className="py-16 text-center text-[13.5px] text-muted">{children}</div>
}

/** The green `Vyřešeno` badge. */
export function DoneBadge() {
  return (
    <span className="flex-none rounded-[4px] border border-done-line bg-done-tint px-[7px] py-[3px] text-[10.5px] font-medium uppercase leading-none tracking-[0.04em] text-done">
      {cs.problem.doneBadge}
    </span>
  )
}

/**
 * The meeting-count badge — the deferral signal.
 *
 * Three states on purpose: zero stays quiet as a dash, one or two is a neutral
 * pill, and three or more turns amber. A problem on its fourth agenda is a
 * problem in trouble and has to be legible at a glance.
 */
export function MeetingBadge({ count }: { count: number }) {
  if (count === 0) return <span className="text-[13px] text-ghost">—</span>
  const amber = count >= 3
  return (
    <span
      className={cx(
        'rounded-[4px] px-[8px] py-[3px] font-mono text-[12px] font-medium leading-[1.2]',
        amber ? 'bg-defer-tint text-defer' : 'bg-surface-track text-secondary',
      )}
      title={`${count}×`}
    >
      {count}×
    </span>
  )
}
