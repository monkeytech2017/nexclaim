'use client'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  LayoutDashboard,
  Inbox,
  FolderDown,
  History,
  Send,
  Building2,
  Stethoscope,
  ArrowRightLeft,
  Pill,
  UserCog,
  FileCode2,
  AlertCircle,
  Columns3,
  FileText,
  Activity,
} from 'lucide-react'

const sections = [
  {
    title: 'การส่งเบิก',
    items: [
      { href: '/dashboard',   label: 'ภาพรวม',       icon: LayoutDashboard },
      { href: '/opd-batches', label: 'OPD Batches',  icon: Inbox, hint: 'HIS 2-way' },
      { href: '/ipd-imports', label: 'IPD Imports',  icon: FolderDown, hint: 'Share folder' },
      { href: '/claims',      label: 'ส่ง Claim',    icon: Send, hint: 'Dev/admin' },
      { href: '/history',     label: 'Ingest History', icon: History, hint: 'OPD batches' },
      { href: '/submissions', label: 'Submissions',    icon: FileText, hint: 'FDH/CHI' },
      { href: '/send-logs',   label: 'Send Logs',      icon: Activity, hint: 'Audit trail' },
      { href: '/c-codes',     label: 'C-code',         icon: AlertCircle, hint: 'REP feedback' },
    ],
  },
  {
    title: 'Master Data',
    items: [
      { href: '/admin/hospitals',   label: 'โรงพยาบาล',        icon: Building2 },
      { href: '/admin/doctors',     label: 'แพทย์',             icon: Stethoscope },
      { href: '/admin/inscl-maps',  label: 'INSCL Mapping',     icon: ArrowRightLeft, hint: 'HIS→NHSO' },
      { href: '/admin/drug-maps',   label: 'Drug Mapping',      icon: Pill,           hint: 'HIS→TMT' },
      { href: '/admin/doctor-maps', label: 'Doctor Mapping',    icon: UserCog,        hint: 'HIS→DRDX' },
      { href: '/admin/icd-maps',    label: 'ICD Mapping',       icon: FileCode2,      hint: 'HIS→WHO' },
      { href: '/admin/field-maps',  label: 'Field Mapping',     icon: Columns3,       hint: 'HIS col→spec' },
    ],
  },
] as const

export default function Sidebar() {
  const pathname = usePathname()

  return (
    <aside className="w-60 bg-white border-r border-gray-100 flex flex-col flex-shrink-0">
      <div className="px-4 py-5 flex items-center gap-3 border-b border-gray-100">
        <div className="w-8 h-8 rounded-lg bg-[#185FA5] flex items-center justify-center flex-shrink-0">
          <svg width="18" height="18" viewBox="0 0 34 34" fill="none">
            <rect x="4" y="10" width="18" height="14" rx="3" fill="#378ADD" opacity=".9" />
            <rect x="12" y="6" width="18" height="14" rx="3" fill="#185FA5" opacity=".7" />
            <path d="M8 17L13 22L22 13" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
        <div className="flex flex-col min-w-0">
          <span className="font-semibold text-sm text-gray-900 leading-tight">NexClaim</span>
          <span className="text-[10px] text-gray-400 leading-tight">Every claim, every fund — connected.</span>
        </div>
      </div>

      <nav className="flex-1 px-2 py-3 flex flex-col gap-3 overflow-auto">
        {sections.map(section => (
          <div key={section.title}>
            <div className="px-3 pt-1 pb-1.5 text-[10px] font-semibold text-gray-400 uppercase tracking-wider">
              {section.title}
            </div>
            <div className="flex flex-col gap-0.5">
              {section.items.map(item => {
                const active = pathname === item.href || pathname.startsWith(item.href + '/')
                const Icon = item.icon
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                      active
                        ? 'bg-primary-50 text-primary-600 font-medium'
                        : 'text-gray-600 hover:bg-gray-50'
                    }`}
                  >
                    <Icon className="w-4 h-4 flex-shrink-0" />
                    <span className="flex-1">{item.label}</span>
                    {'hint' in item && item.hint && !active && (
                      <span className="text-[10px] text-gray-400 font-normal">{item.hint}</span>
                    )}
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="px-4 py-3 border-t border-gray-100 text-xs text-gray-400">v0.1.0</div>
    </aside>
  )
}
