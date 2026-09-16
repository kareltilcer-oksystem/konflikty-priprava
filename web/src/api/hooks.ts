import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api } from './client'
import type {
  Attachment,
  Meeting,
  MeetingDetail,
  MeetingItem,
  Problem,
  ProblemDetail,
  ProblemPatch,
  ProblemQuery,
} from './types'

export const keys = {
  me: ['me'] as const,
  users: ['users'] as const,
  problems: (q: ProblemQuery) => ['problems', q] as const,
  problem: (id: number) => ['problem', id] as const,
  meetings: (includeArchived: boolean) => ['meetings', includeArchived] as const,
  meeting: (slug: string) => ['meeting', slug] as const,
}

/**
 * Invalidates everything a write can affect.
 *
 * The relationships here are dense — marking a problem done changes the bucket,
 * that problem's detail and every agenda it sits on — and the dataset is tiny,
 * so refetching broadly is both correct and cheap. Narrowing it would only
 * create ways for one of the four screens to go stale.
 */
function invalidateAll(qc: QueryClient) {
  return Promise.all([
    qc.invalidateQueries({ queryKey: ['problems'] }),
    qc.invalidateQueries({ queryKey: ['problem'] }),
    qc.invalidateQueries({ queryKey: ['meetings'] }),
    qc.invalidateQueries({ queryKey: ['meeting'] }),
  ])
}

function problemSearch(q: ProblemQuery): string {
  const params = new URLSearchParams()
  if (q.done !== undefined) params.set('done', String(q.done))
  if (q.q) params.set('q', q.q)
  if (q.scheduled !== undefined) params.set('scheduled', String(q.scheduled))
  if (q.sort) params.set('sort', q.sort)
  const s = params.toString()
  return s ? `?${s}` : ''
}

// ---------- auth ----------

export function useMe() {
  return useQuery({
    queryKey: keys.me,
    // null is a valid answer — an anonymous visitor, not a failure.
    queryFn: () => api.get<import('./types').User | null>('/auth/me'),
    staleTime: 60_000,
  })
}

/**
 * The configured accounts, for resolving an author's display name.
 *
 * Records store the username; every screen that names an author shows the
 * display name. The set is three rows fixed at start-up, so it is fetched once
 * and never goes stale within a session.
 */
export function useUsers() {
  return useQuery({
    queryKey: keys.users,
    queryFn: () => api.get<import('./types').User[]>('/users'),
    staleTime: Infinity,
  })
}

/** Maps a username to its display name, falling back to the username itself. */
export function useDisplayName(): (username: string) => string {
  const { data } = useUsers()
  return (username: string) =>
    data?.find((u) => u.username === username)?.display_name ?? username
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { username: string; password: string }) =>
      api.post<import('./types').User>('/auth/login', body),
    onSuccess: (user) => {
      qc.setQueryData(keys.me, user)
      return invalidateAll(qc)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<void>('/auth/logout'),
    onSuccess: () => {
      qc.setQueryData(keys.me, null)
      return invalidateAll(qc)
    },
  })
}

// ---------- problems ----------

export function useProblems(q: ProblemQuery) {
  return useQuery({
    queryKey: keys.problems(q),
    queryFn: () => api.get<Problem[]>(`/problems${problemSearch(q)}`),
    // Keeps the previous rows on screen while a new filter loads, so typing in
    // the search box does not blank the list on every keystroke.
    placeholderData: (prev) => prev,
  })
}

export function useProblem(id: number) {
  return useQuery({
    queryKey: keys.problem(id),
    queryFn: () => api.get<ProblemDetail>(`/problems/${id}`),
    enabled: Number.isFinite(id) && id > 0,
  })
}

/**
 * Creates a problem.
 *
 * A FormData body sends the problem and its files in one atomic request, so a
 * rejected upload leaves nothing behind (FR-P1). Plain fields go as JSON.
 */
export function useCreateProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: FormData) => api.form<ProblemDetail>('/problems', 'POST', body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useUpdateProblem(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: FormData | ProblemPatch) =>
      body instanceof FormData
        ? api.form<ProblemDetail>(`/problems/${id}`, 'PATCH', body)
        : api.patch<ProblemDetail>(`/problems/${id}`, body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useDeleteProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete<void>(`/problems/${id}`),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useSetDone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, done }: { id: number; done: boolean }) =>
      done
        ? api.post<ProblemDetail>(`/problems/${id}/done`)
        : api.delete<ProblemDetail>(`/problems/${id}/done`),
    onSuccess: () => invalidateAll(qc),
  })
}

// ---------- attachments ----------

export function useUploadAttachments(problemId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: FormData) => api.form<Attachment>(`/problems/${problemId}/attachments`, 'POST', body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useDeleteAttachment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete<void>(`/attachments/${id}`),
    onSuccess: () => invalidateAll(qc),
  })
}

// ---------- meetings ----------

export function useMeetings(includeArchived: boolean) {
  return useQuery({
    queryKey: keys.meetings(includeArchived),
    queryFn: () => api.get<Meeting[]>(`/meetings${includeArchived ? '?include_archived=true' : ''}`),
  })
}

export function useMeeting(slug: string) {
  return useQuery({
    queryKey: keys.meeting(slug),
    queryFn: () => api.get<MeetingDetail>(`/meetings/${slug}`),
    enabled: Boolean(slug),
  })
}

export function useCreateMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { meeting_date: string; note?: string }) =>
      api.post<MeetingDetail>('/meetings', body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useUpdateMeeting(slug: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { meeting_date?: string; note?: string }) =>
      api.patch<MeetingDetail>(`/meetings/${slug}`, body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useDeleteMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (slug: string) => api.delete<void>(`/meetings/${slug}`),
    onSuccess: () => invalidateAll(qc),
  })
}

// ---------- agenda ----------

export function useAddItems(slug: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (problemIds: number[]) =>
      api.post<MeetingItem[]>(`/meetings/${slug}/items`, { problem_ids: problemIds }),
    onSuccess: () => invalidateAll(qc),
  })
}

/**
 * Persists a drag-and-drop reorder.
 *
 * The list must name every item of the meeting exactly once; the server
 * rejects anything else, which is why the caller sends the whole order rather
 * than the pair that moved.
 */
export function useReorderItems(slug: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (itemIds: number[]) =>
      api.put<MeetingItem[]>(`/meetings/${slug}/items/order`, { item_ids: itemIds }),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useUpdateItem(slug: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ itemId, ...body }: { itemId: number; prep_note?: string; action_note?: string }) =>
      api.patch<MeetingItem>(`/meetings/${slug}/items/${itemId}`, body),
    onSuccess: () => invalidateAll(qc),
  })
}

export function useRemoveItem(slug: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (itemId: number) => api.delete<void>(`/meetings/${slug}/items/${itemId}`),
    onSuccess: () => invalidateAll(qc),
  })
}
