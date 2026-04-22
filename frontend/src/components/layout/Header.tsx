'use client'
import { usePathname } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { health } from '@/lib/api'

const titles: Record<string, string> = {
  '/dashboard':       'ภาพรวม',
  '/opd-batches':     'OPD Batches (HIS 2-way)',
  '/ipd-imports':     'IPD Imports (Share folder)',
  '/claims':          'ส่ง Claim',
  '/history':         'Ingest History (OPD batches ที่รับจาก HIS)',
  '/submissions':     'Submissions (claim_batch ที่ส่งไป FDH/CHI)',
  '/c-codes':         'C-code (ข้อผิดพลาดจาก REP)',
  '/admin/hospitals':   'โรงพยาบาล',
  '/admin/doctors':     'แพทย์',
  '/admin/inscl-maps':  'INSCL Mapping (HIS PTTYPE → NHSO INSCL)',
  '/admin/drug-maps':   'Drug Mapping (HIS Drug → TMT24)',
  '/admin/doctor-maps': 'Doctor Mapping (HIS Doctor → DRDX)',
  '/admin/icd-maps':    'ICD Mapping (HIS ICD → ICD-10/9CM)',
  '/admin/field-maps':  'Field Mapping (HIS column → target spec)',
}

function titleFor(pathname: string): string {
  if (pathname.startsWith('/opd-batches/')) return 'OPD Batch detail'
  return titles[pathname] ?? 'NexClaim'
}

export default function Header() {
  const pathname = usePathname()
  const title = titleFor(pathname)

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
        <div className="w-7 h-7 rounded-full bg-primary-50 flex items-center justify-center text-xs font-medium text-primary-600">
          JT
        </div>
      </div>
    </header>
  )
}
