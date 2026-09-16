// The icon set used across the app, traced from design/v1. Inline SVG rather
// than an icon package: there are nine of them and they never change.

type Props = { className?: string; size?: number }

const base = {
  fill: 'none',
  stroke: 'currentColor',
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

export function PlusIcon({ size = 14, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2.4} {...base} className={className} aria-hidden>
      <path d="M12 5v14M5 12h14" />
    </svg>
  )
}

export function CheckIcon({ size = 12, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={3} {...base} className={className} aria-hidden>
      <path d="m5 13 4 4L19 7" />
    </svg>
  )
}

export function CloseIcon({ size = 16, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M6 6l12 12M18 6 6 18" />
    </svg>
  )
}

export function SearchIcon({ size = 15, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-4.3-4.3" />
    </svg>
  )
}

export function ChevronLeftIcon({ size = 13, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M15 6l-6 6 6 6" />
    </svg>
  )
}

export function ChevronRightIcon({ size = 13, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M9 6l6 6-6 6" />
    </svg>
  )
}

export function PaperclipIcon({ size = 13, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M21.4 11.05 12.25 20.2a5.5 5.5 0 0 1-7.78-7.78l8.49-8.48a3.67 3.67 0 0 1 5.18 5.18l-8.49 8.49a1.83 1.83 0 0 1-2.59-2.6l7.78-7.77" />
    </svg>
  )
}

export function FileIcon({ size = 20, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={1.6} {...base} className={className} aria-hidden>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
    </svg>
  )
}

export function ImageIcon({ size = 16, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={1.8} {...base} className={className} aria-hidden>
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <circle cx="8.5" cy="9.5" r="1.5" />
      <path d="m4 18 5-5 4 4 3-3 4 4" />
    </svg>
  )
}

export function VideoIcon({ size = 16, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={1.8} {...base} className={className} aria-hidden>
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <path d="m10.5 9.5 4.5 2.5-4.5 2.5z" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function PlayIcon({ size = 11, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden>
      <path d="M8 5l12 7-12 7z" />
    </svg>
  )
}

export function DownloadIcon({ size = 13, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M12 4v11m0 0 4-4m-4 4-4-4M5 20h14" />
    </svg>
  )
}

export function ExternalLinkIcon({ size = 14, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M10 14 21 3m0 0h-6m6 0v6M19 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h5" />
    </svg>
  )
}

export function AlertIcon({ size = 13, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5.5M12 16.2v.3" />
    </svg>
  )
}

export function SortArrowsIcon({ size = 14, className }: Props) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeWidth={2} {...base} className={className} aria-hidden>
      <path d="M12 5v14M8 9l4-4 4 4M8 15l4 4 4-4" />
    </svg>
  )
}

/** The six-dot drag handle. Filled rather than stroked, as in the design. */
export function DragHandleIcon({ className }: Props) {
  return (
    <svg width={12} height={18} viewBox="0 0 12 18" fill="currentColor" className={className} aria-hidden>
      <circle cx="3" cy="3" r="1.5" />
      <circle cx="9" cy="3" r="1.5" />
      <circle cx="3" cy="9" r="1.5" />
      <circle cx="9" cy="9" r="1.5" />
      <circle cx="3" cy="15" r="1.5" />
      <circle cx="9" cy="15" r="1.5" />
    </svg>
  )
}
