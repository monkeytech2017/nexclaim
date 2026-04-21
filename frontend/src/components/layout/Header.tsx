'use client'
import { usePathname } from 'next/navigation'

const titles: Record<string, string> = {
  '/dashboard':      'ภาพรวม',
  '/claims':         'ส่ง Claim',
  '/ccode':          'C-code',
  '/reports':        'รายงาน',
  '/admin/master':   'Master Data',
  '/admin/mapping':  'HIS Mapping',
  '/admin/hospital': 'โรงพยาบาล',
  '/admin/logs':     'System Logs',
}

export default function Header() {
  const pathname = usePathname()
  const title = titles[pathname] ?? 'NexClaim'

  return (
    <header className="h-14 bg-white border-b border-gray-100 px-6 flex items-center justify-between flex-shrink-0">
      <h1 className="text-sm font-medium text-gray-900">{title}</h1>
      <div className="flex items-center gap-3">
        <span className="text-xs text-gray-400">โรงพยาบาล XXXXX</span>
        <div className="w-7 h-7 rounded-full bg-primary-50 flex items-center justify-center text-xs font-medium text-primary-600">
          JT
        </div>
      </div>
    </header>
  )
}
