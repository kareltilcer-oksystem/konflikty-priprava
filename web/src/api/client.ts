import { cs } from '../i18n/cs'

/**
 * An error carrying the server's Czech message.
 *
 * The API emits display-ready Czech (PRD 9.3), so the UI prints `message` as it
 * arrives rather than keeping a second copy of these strings keyed by code.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: Record<string, string>

  constructor(status: number, code: string, message: string, details: Record<string, string> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }

  get isNotFound() {
    return this.status === 404
  }
  get isUnauthorized() {
    return this.status === 401
  }
  get isForbidden() {
    return this.status === 403
  }
}

interface ErrorEnvelope {
  error?: { code?: string; message?: string; details?: Record<string, string> }
}

/**
 * Sends a request to the API on the page's own origin.
 *
 * `/api` resolves through the Vite proxy in development and through the SPA
 * listener in production, so nothing here needs to know which it is talking to
 * and no CORS is involved.
 */
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      // The session lives in an HttpOnly cookie the client never handles.
      credentials: 'include',
      ...init,
    })
  } catch {
    // A network failure has no server message to show, so supply one.
    throw new ApiError(0, 'network_error', cs.errors.network)
  }

  if (res.status === 204) return undefined as T

  const isJSON = res.headers.get('Content-Type')?.includes('application/json') ?? false
  if (!res.ok) {
    let code: string = 'unknown'
    let message: string = cs.errors.unknown
    let details: Record<string, string> = {}
    if (isJSON) {
      const body = (await res.json().catch(() => ({}))) as ErrorEnvelope
      code = body.error?.code ?? code
      message = body.error?.message ?? message
      details = body.error?.details ?? {}
    }
    throw new ApiError(res.status, code, message, details)
  }

  if (!isJSON) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),

  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'POST',
      ...(body === undefined
        ? {}
        : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
    }),

  patch: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  put: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),

  /**
   * Sends a multipart body.
   *
   * Content-Type is deliberately not set: the browser has to add the multipart
   * boundary itself, and setting the header by hand strips it.
   */
  form: <T>(path: string, method: 'POST' | 'PATCH', body: FormData) =>
    request<T>(path, { method, body }),
}
