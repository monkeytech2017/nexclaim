'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthError } from '@/lib/auth-context'

// AuthGuard — renders children immediately; redirects to /login only when
// whoami returned 401 (authError).
//
// whoami is the single source of truth: request() already attaches the stored
// key (or NEXT_PUBLIC_API_KEY fallback), so the backend's response disambiguates
// every case for us:
//   - auth DISABLED  → 204 → identity null, NO authError → stay (default dev mode)
//   - auth enabled, missing/expired key → 401 → authError → /login
//   - auth enabled, valid key → 200 → identity → stay
//
// A naive "redirect when localStorage has no key" check would loop forever in
// auth-disabled mode (/dashboard → /login → skip → /dashboard → …), so we lean
// entirely on authError instead.
export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const authError = useAuthError()

  useEffect(() => {
    if (authError) {
      router.push('/login')
    }
  }, [authError, router])

  return <>{children}</>
}
