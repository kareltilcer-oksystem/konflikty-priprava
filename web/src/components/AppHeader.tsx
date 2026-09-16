import { Link, useLocation } from 'react-router-dom'
import { cs } from '../i18n/cs'
import { cx } from './ui'
import type { User } from '../api/types'

/**
 * The app header: name, two links, and the account slot.
 *
 * The display name is the only account UI in the product — it is how someone
 * knows which of the three accounts is signed in.
 *
 * It carries print-hide: Ctrl+P on the agenda must produce the agenda alone.
 */
export function AppHeader({
  user,
  onSignIn,
  onSignOut,
}: {
  user: User | null | undefined
  onSignIn: () => void
  onSignOut: () => void
}) {
  return (
    <header className="print-hide flex h-[56px] flex-none items-center gap-7 border-b border-line bg-white px-8">
      <Link
        to="/"
        className="whitespace-nowrap text-[15px] font-semibold leading-none tracking-[-0.01em] text-ink no-underline"
      >
        {cs.app.name}
      </Link>

      <nav className="flex h-full items-center gap-1">
        <HeaderLink to="/" label={cs.app.nav.problems} section="problems" />
        <HeaderLink to="/porady" label={cs.app.nav.meetings} section="meetings" />
      </nav>

      <div className="flex-1" />

      {user ? (
        <div className="flex items-center gap-[14px]">
          <span className="text-[13px] font-medium leading-none text-ink">{user.display_name}</span>
          <button
            type="button"
            onClick={onSignOut}
            className="h-[28px] rounded-[6px] border border-field bg-white px-[10px] text-[12.5px] leading-none text-secondary transition-colors hover:bg-surface-hover hover:text-ink focus-ring"
          >
            {cs.app.signOut}
          </button>
        </div>
      ) : (
        <button
          type="button"
          onClick={onSignIn}
          className="h-[30px] rounded-[6px] border border-field bg-white px-[12px] text-[13px] font-medium leading-none text-ink transition-colors hover:bg-surface-hover focus-ring"
        >
          {cs.app.signIn}
        </button>
      )}
    </header>
  )
}

type Section = 'problems' | 'meetings'

/** Which nav tab a path belongs to. */
function sectionFor(pathname: string): Section | null {
  if (pathname === '/' || pathname.startsWith('/problem/')) return 'problems'
  if (pathname === '/porady' || pathname.startsWith('/porada/')) return 'meetings'
  return null
}

function HeaderLink({ to, label, section }: { to: string; label: string; section: Section }) {
  // A plain NavLink would not light Porady on /porada/{slug}: the shareable
  // meeting URL is a different path from the list it belongs to.
  const active = sectionFor(useLocation().pathname) === section

  return (
    <Link
      to={to}
      aria-current={active ? 'page' : undefined}
      className={cx(
        'flex h-full items-center px-[10px] text-[13.5px] leading-none no-underline transition-colors',
        active
          ? 'font-medium text-ink shadow-[inset_0_-2px_0_var(--color-primary)]'
          : 'text-secondary hover:text-ink',
      )}
    >
      {label}
    </Link>
  )
}
