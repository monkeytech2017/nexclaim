'use client'
import { usePathname } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { LogOut } from 'lucide-react'
import { health } from '@/lib/api'
import { useIdentity, useLogout } from '@/lib/auth-context'

const titles: Record<string, string> = {
  '/dashboard':       'ภาพรวม',
  '/opd-batches':     'OPD Batches (HIS 2-way)',
  '/ipd-imports':     'IPD Imports (Share folder)',
  '/claims':          'ส่ง Claim',
  '/history':         'Ingest History (OPD batches ที่รับจาก HIS)',
  '/submissions':     'Submissions (claim_batch ที่ส่งไป FDH/CHI)',
  '/send-logs':       'Send Logs (audit trail ทุกครั้งที่ส่งไป FDH/CHI)',
  '/c-codes':         'C-code (ข้อผิดพลาดจาก REP)',
  '/admin/hospitals':   'โรงพยาบาล',
  '/admin/doctors':     'แพทย์',
  '/admin/inscl-maps':  'INSCL Mapping (HIS PTTYPE → NHSO INSCL)',
  '/admin/drug-maps':   'Drug Mapping (HIS Drug → TMT24)',
  '/admin/doctor-maps': 'Doctor Mapping (HIS Doctor → DRDX)',
  '/admin/icd-maps':    'ICD Mapping (HIS ICD → ICD-10/9CM)',
  '/admin/field-maps':  'Field Mapping (HIS column → target spec)',
  '/admin/api-keys':    'API Keys (สำหรับ admin)',
  '/admin/audit-log':   'Audit Log (admin)',
}

function titleFor(pathname: string): string {
  if (pathname.startsWith('/opd-batches/')) return 'OPD Batch detail'
  return titles[pathname] ?? 'NexClaim'
}

export default function Header() {
  const pathname = usePathname()
  const title = titleFor(pathname)
  const identity = useIdentity()
  const logout = useLogout()

  const { data, isError } = useQuery({
    queryKey: ['health'],
    queryFn: health,
    refetchInterval: 30_000,
  })

  const up = data?.ok === true
  const statusCls = isError
    ? 'bg-red-50 text-red-700'
    : up
      ? 'bg-green-50 text-green-700'
      : 'bg-gray-100 text-gray-500'
  const statusLabel = isError ? 'Backend offline' : up ? 'Backend online' : 'กำลังเช็ค...'

  return (
    <header className="h-14 bg-white border-b border-gray-100 px-6 flex items-center justify-between flex-shrink-0">
      <h1 className="text-sm font-medium text-gray-900">{title}</h1>
      <div className="flex items-center gap-3">
        <span className={`text-[11px] px-2 py-1 rounded-full font-medium ${statusCls}`}>
          {statusLabel}
        </span>
        {identity && (
          <>
            <span
              className={`text-xs px-2 py-1 rounded-full font-medium ${
                identity.role === 'admin'
                  ? 'bg-gray-100 text-gray-700'
                  : 'bg-blue-50 text-blue-700'
              }`}
              title={identity.id}
            >
              {identity.role === 'admin'
                ? `admin — ${identity.name}`
                : `รพ. ${identity.hcode ?? ''} — ${identity.name}`}
            </span>
            <button
              type="button"
              onClick={logout}
              title="ออกจากระบบ"
              aria-label="ออกจากระบบ"
              className="w-7 h-7 rounded-full flex items-center justify-center text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </>
        )}
        <div className="w-7 h-7 rounded-full bg-primary-50 flex items-center justify-center text-xs font-medium text-primary-600">
          JT
        </div>
      </div>
    </header>
  )
}
