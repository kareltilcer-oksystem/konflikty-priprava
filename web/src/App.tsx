import { useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AppHeader } from './components/AppHeader'
import { LoginDialog } from './features/auth/LoginDialog'
import { BucketPage } from './features/problems/BucketPage'
import { ProblemDetailPage } from './features/problems/ProblemDetailPage'
import { MeetingsPage } from './features/meetings/MeetingsPage'
import { MeetingPage } from './features/meetings/MeetingPage'
import { NotFoundPage } from './components/NotFoundPage'
import { useLogout, useMe } from './api/hooks'

export function App() {
  const { data: user } = useMe()
  const logout = useLogout()
  const [loginOpen, setLoginOpen] = useState(false)

  return (
    <div className="flex min-h-screen flex-col">
      <AppHeader
        user={user}
        onSignIn={() => setLoginOpen(true)}
        onSignOut={() => logout.mutate()}
      />

      <main className="min-h-0 flex-1">
        <Routes>
          <Route path="/" element={<BucketPage user={user ?? null} />} />
          <Route path="/problem/:id" element={<ProblemDetailPage user={user ?? null} />} />
          <Route path="/porady" element={<MeetingsPage user={user ?? null} />} />
          <Route path="/porada/:slug" element={<MeetingPage user={user ?? null} />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </main>

      <LoginDialog open={loginOpen} onClose={() => setLoginOpen(false)} />
    </div>
  )
}
