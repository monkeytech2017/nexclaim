import LoginClient from './login-client'
import Providers from '@/lib/providers'

export default function LoginPage() {
  // Login lives outside the (dashboard) segment → no sidebar/header.
  // We still need Providers for React Query (whoami on-mount check).
  return (
    <Providers>
      <LoginClient />
    </Providers>
  )
}
