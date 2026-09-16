import { useState, type FormEvent } from 'react'
import { useLogin } from '../../api/hooks'
import { ApiError } from '../../api/client'
import { cs } from '../../i18n/cs'
import { Modal, btn, cx, input, label, ErrorBox } from '../../components/ui'
import { CloseIcon } from '../../components/Icons'

/**
 * The login dialog.
 *
 * A modal rather than a page, because everything behind it is already visible:
 * an anonymous visitor reads the whole app and only signs in to write
 * (FR-A2). Nothing here ever redirects.
 */
export function LoginDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const login = useLogin()

  function submit(e: FormEvent) {
    e.preventDefault()
    login.mutate(
      { username, password },
      {
        onSuccess: () => {
          setUsername('')
          setPassword('')
          onClose()
        },
      },
    )
  }

  // The server's Czech message is printed as it arrives.
  const message = login.error instanceof ApiError ? login.error.message : null

  return (
    <Modal open={open} onClose={onClose} width={388} labelledBy="login-title">
      <form onSubmit={submit} className="p-[22px]">
        <div className="flex items-center justify-between">
          <span id="login-title" className="text-[16.5px] font-semibold leading-none">
            {cs.login.title}
          </span>
          <button
            type="button"
            onClick={onClose}
            aria-label={cs.login.cancel}
            className={cx(btn.icon, 'h-[28px] w-[28px]')}
          >
            <CloseIcon size={15} />
          </button>
        </div>

        <div className="mt-[18px] flex flex-col gap-[13px]">
          <div className="flex flex-col gap-[6px]">
            <label className={label} htmlFor="login-username">
              {cs.login.username}
            </label>
            <input
              id="login-username"
              className={input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
          </div>
          <div className="flex flex-col gap-[6px]">
            <label className={label} htmlFor="login-password">
              {cs.login.password}
            </label>
            <input
              id="login-password"
              type="password"
              className={input}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
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
            {cs.login.cancel}
          </button>
          <button type="submit" className={btn.primary} disabled={login.isPending}>
            {cs.login.submit}
          </button>
        </div>
      </form>
    </Modal>
  )
}
