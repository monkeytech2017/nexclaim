import { Loader2, AlertCircle } from 'lucide-react'
import { AuthError } from '@/lib/api'

export function LoadingBlock({ label = 'กำลังโหลด...' }: { label?: string }) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 p-10 flex items-center justify-center gap-3 text-gray-500 text-sm">
      <Loader2 className="w-4 h-4 animate-spin" />
      {label}
    </div>
  )
}

export function ErrorBlock({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const msg = error instanceof AuthError
    ? 'ต้องตั้งค่า NEXT_PUBLIC_API_KEY ก่อน'
    : error instanceof Error ? error.message : String(error)
  return (
    <div className="bg-red-50 border border-red-100 rounded-xl p-4 flex items-start gap-3">
      <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-red-900">เกิดข้อผิดพลาด</div>
        <pre className="text-xs text-red-700 mt-1 whitespace-pre-wrap break-all">{msg}</pre>
        {onRetry && (
          <button
            onClick={onRetry}
            className="mt-2 text-xs font-medium text-red-700 hover:text-red-900"
          >
            ลองอีกครั้ง →
          </button>
        )}
      </div>
    </div>
  )
}

export function EmptyBlock({ label }: { label: string }) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 p-10 text-center text-gray-400 text-sm">
      {label}
    </div>
  )
}
