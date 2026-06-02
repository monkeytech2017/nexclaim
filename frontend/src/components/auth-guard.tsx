'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { storedApiKey } from '@/lib/api'
import { useAuthError } from '@/lib/auth-context'

// AuthGuard — renders children immediately, redirects to /login asynchronously
// if no API key is present in localStorage (and no NEXT_PUBLIC_API_KEY fallback),
// or if whoami returned 401 (authError flag).
export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const authError = useAuthError()

  useEffect(() => {
    const hasStored = storedApiKey() !== null
    const hasEnv = !!process.env.NEXT_PUBLIC_API_KEY
    if (!hasStored && !hasEnv) {
      router.push('/login')
      return
    }
    if (authError) {
      router.push('/login')
    }
  }, [authError, router])

  return <>{children}</>
}
