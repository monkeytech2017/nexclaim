'use client'

import { createContext, useCallback, useContext, useMemo } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { authApi, AuthError, clearApiKey, type Identity } from '@/lib/api'

// AuthContext shape:
//   identity   — Identity | null. null = auth disabled / no key / unauthed.
//   authError  — true only when whoami returned 401 (AuthError). Used by
//                AuthGuard to redirect to /login. Distinct from "no identity"
//                which also covers AUTH_ENABLED=false on backend.
//   logout()   — clear stored key, invalidate whoami, push /login.
interface AuthContextValue {
  identity: Identity | null
  authError: boolean
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  identity: null,
  authError: false,
  logout: () => {},
})

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const queryClient = useQueryClient()

  const { data, error } = useQuery<Identity | null, Error>({
    queryKey: ['whoami'],
    queryFn: async () => {
      try {
        return await authApi.whoami()
      } catch (e) {
        // 401 → bubble up as AuthError so we can flag authError=true.
        if (e instanceof AuthError) throw e
        // Any other error (incl. 404 when AUTH_ENABLED=false): treat as no identity.
        return null
      }
    },
    retry: false,
    staleTime: 5 * 60_000,
  })

  const authError = error instanceof AuthError

  const logout = useCallback(() => {
    // Clear storage BEFORE invalidating — otherwise the next whoami could
    // succeed on the stale key and bounce us back out of /login.
    clearApiKey()
    queryClient.removeQueries({ queryKey: ['whoami'] })
    router.push('/login')
  }, [queryClient, router])

  const value = useMemo<AuthContextValue>(
    () => ({ identity: data ?? null, authError, logout }),
    [data, authError, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useIdentity(): Identity | null {
  return useContext(AuthContext).identity
}

export function useIsAdmin(): boolean {
  return useContext(AuthContext).identity?.role === 'admin'
}

export function useAuthError(): boolean {
  return useContext(AuthContext).authError
}

export function useLogout(): () => void {
  return useContext(AuthContext).logout
}
