import Sidebar from '@/components/layout/Sidebar'
import Header from '@/components/layout/Header'
import Providers from '@/lib/providers'
import { AuthProvider } from '@/lib/auth-context'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <Providers>
      <AuthProvider>
        <div className="flex h-screen bg-gray-50">
          <Sidebar />
          <div className="flex-1 flex flex-col min-w-0">
            <Header />
            <main className="flex-1 overflow-auto p-6">{children}</main>
          </div>
        </div>
      </AuthProvider>
    </Providers>
  )
}
