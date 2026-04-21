'use client'
import Link from 'next/link'
import { usePathname } from 'next/navigation'

const financeNav = [
  { href: '/dashboard',  label: 'ภาพรวม' },
  { href: '/claims',     label: 'ส่ง Claim' },
  { href: '/ccode',      label: 'C-code' },
  { href: '/reports',    label: 'รายงาน' },
]

const adminNav = [
  { href: '/dashboard',       label: 'ภาพรวม' },
  { href: '/admin/master',    label: 'Master Data' },
  { href: '/admin/mapping',   label: 'HIS Mapping' },
  { href: '/admin/hospital',  label: 'โรงพยาบาล' },
  { href: '/admin/logs',      label: 'System Logs' },
]

export default function Sidebar({ role = 'finance' }: { role?: 'finance' | 'admin' }) {
  const pathname = usePathname()
  const nav = role === 'admin' ? adminNav : financeNav

  return (
    <aside className="w-56 bg-white border-r border-gray-100 flex flex-col">
      {/* Logo */}
      <div className="px-4 py-5 flex items-center gap-3 border-b border-gray-100">
        <div className="w-8 h-8 rounded-lg bg-[#0F172A] flex items-center justify-center flex-shrink-0">
          <svg width="18" height="18" viewBox="0 0 34 34" fill="none">
            <rect x="4" y="10" width="18" height="14" rx="3" fill="#378ADD" opacity=".9"/>
            <rect x="12" y="6" width="18" height="14" rx="3" fill="#185FA5" opacity=".7"/>
            <path d="M8 17L13 22L22 13" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </div>
        <span className="font-semibold text-sm text-gray-900">NexClaim</span>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-2 py-3 flex flex-col gap-0.5">
        {nav.map(item => {
          const active = pathname === item.href || pathname.startsWith(item.href + '/')
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center px-3 py-2 rounded-lg text-sm transition-colors ${
                active
                  ? 'bg-primary-50 text-primary-600 font-medium'
                  : 'text-gray-600 hover:bg-gray-50'
              }`}
            >
              {item.label}
            </Link>
          )
        })}
      </nav>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-gray-100 text-xs text-gray-400">
        v0.1.0
      </div>
    </aside>
  )
}
