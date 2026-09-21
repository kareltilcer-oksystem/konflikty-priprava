import { Link } from 'react-router-dom'
import { cs } from '../i18n/cs'

/** The 404 screen: an explanation and a way back, never a login prompt. */
export function NotFound({ detail, to, linkLabel }: { detail?: string; to: string; linkLabel: string }) {
  return (
    <div className="px-8 py-14 text-center">
      <div className="text-[17px] font-semibold leading-[1.4] text-ink">{cs.errors.notFoundTitle}</div>
      {detail && <div className="mt-[7px] text-[13px] leading-[1.5] text-muted">{detail}</div>}
      <Link to={to} className="mt-[14px] inline-block text-[13.5px] leading-none text-primary">
        {linkLabel}
      </Link>
    </div>
  )
}

export function NotFoundPage() {
  return <NotFound to="/" linkLabel={cs.errors.toProblems} />
}
