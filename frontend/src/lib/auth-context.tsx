'use client'

import { createContext, useContext } from 'react'
import { useQuery } from '@tanstack/react-query'
import { authApi, AuthError, type Identity } from '@/lib/api'

// null = auth disabled on server, OR no API key configured,
// OR whoami failed (unauthed). The UI should treat all three the same:
// "no identity" — don't block rendering, don't force a login prompt.
const AuthContext = createContext<Identity | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { data } = useQuery<Identity | null>({
    queryKey: ['whoami'],
    queryFn: async () => {
      try {
        return await authApi.whoami()
      } catch (e) {
        // Auth disabled (404 on endpoint) or unauthed — render without identity.
        if (e instanceof AuthError) return null
        // Any other error (incl. 404 when AUTH_ENABLED=false): also treat as no identity.
        return null
      }
    },
    retry: false,
    staleTime: 5 * 60_000,
  })

  return <AuthContext.Provider value={data ?? null}>{children}</AuthContext.Provider>
}

export function useIdentity(): Identity | null {
  return useContext(AuthContext)
}

export function useIsAdmin(): boolean {
  return useContext(AuthContext)?.role === 'admin'
}
