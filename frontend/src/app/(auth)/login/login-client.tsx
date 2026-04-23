'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { authApi, AuthError, clearApiKey, setApiKey, storedApiKey } from '@/lib/api'

type OnMountState =
  | { kind: 'checking' }
  | { kind: 'needs-key' }
  | { kind: 'auth-disabled' } // backend returned null identity (AUTH_ENABLED=false)

export default function LoginClient() {
  const router = useRouter()
  const [key, setKey] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [mountState, setMountState] = useState<OnMountState>({ kind: 'checking' })

  // On-mount: if we already have a key that works, skip the form.
  // Also detect auth-disabled backend (whoami resolves without 401 + no identity).
  useEffect(() => {
    let cancelled = false
    const check = async () => {
      const existing = storedApiKey()
      const envKey = process.env.NEXT_PUBLIC_API_KEY
      // No key at all → might still be auth-disabled. Try whoami; if it 401s,
      // we need a key; if it errors otherwise or returns, treat as disabled.
      try {
        const identity = await authApi.whoami()
        if (cancelled) return
        if (identity) {
          router.replace('/dashboard')
          return
        }
        // whoami resolved with null → auth disabled on backend.
        setMountState({ kind: 'auth-disabled' })
      } catch (e) {
        if (cancelled) return
        if (e instanceof AuthError) {
          // Invalid/missing key → show form; clear stale key if present.
          if (existing && !envKey) clearApiKey()
          setMountState({ kind: 'needs-key' })
          return
        }
        // Network / other error → show form so user can try.
        setMountState({ kind: 'needs-key' })
      }
    }
    check()
    return () => { cancelled = true }
  }, [router])

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!key.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      try {
        setApiKey(key.trim())
      } catch {
        setError('เบราว์เซอร์ไม่อนุญาตให้บันทึก — ลองปิดโหมดไม่ระบุตัวตน')
        setSubmitting(false)
        return
      }
      await authApi.whoami()
      router.replace('/dashboard')
    } catch (err) {
      clearApiKey()
      if (err instanceof AuthError) {
        setError('คีย์ไม่ถูกต้อง หรือหมดอายุ')
      } else {
        setError('เข้าสู่ระบบไม่สำเร็จ — ตรวจสอบ network และลองใหม่')
      }
      setSubmitting(false)
    }
  }

  if (mountState.kind === 'checking') {
    return (
      <main className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-sm text-gray-400">กำลังตรวจสอบสถานะ...</div>
      </main>
    )
  }

  return (
    <main className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="bg-white rounded-xl border border-gray-200 p-8 w-full max-w-sm shadow-sm">
        <div className="flex items-center gap-3 mb-8">
          <div className="w-12 h-12 rounded-xl bg-[#185FA5] flex items-center justify-center flex-shrink-0">
            <svg width="26" height="26" viewBox="0 0 34 34" fill="none">
              <rect x="4" y="10" width="18" height="14" rx="3" fill="#378ADD" opacity=".9" />
              <rect x="12" y="6" width="18" height="14" rx="3" fill="#185FA5" opacity=".7" />
              <path
                d="M8 17L13 22L22 13"
                stroke="white"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <div>
            <div className="font-semibold text-gray-900">NexClaim</div>
            <div className="text-xs text-gray-500">Every claim, every fund — connected.</div>
          </div>
        </div>

        {mountState.kind === 'auth-disabled' ? (
          <div className="flex flex-col gap-4">
            <div className="rounded-lg bg-blue-50 border border-blue-100 p-3 text-xs text-blue-800">
              ระบบยังไม่ได้เปิด auth — เข้าใช้งานได้เลย
            </div>
            <button
              type="button"
              onClick={() => router.replace('/dashboard')}
              className="w-full bg-primary-600 text-white rounded-lg py-2 text-sm font-medium hover:bg-primary-800 transition-colors"
            >
              ไปหน้า Dashboard
            </button>
          </div>
        ) : (
          <form className="flex flex-col gap-4" onSubmit={onSubmit}>
            <div>
              <label htmlFor="api-key" className="block text-sm text-gray-600 mb-1">
                API Key
              </label>
              <input
                id="api-key"
                type="password"
                autoComplete="off"
                autoFocus
                value={key}
                onChange={(e) => setKey(e.target.value)}
                className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-400 font-mono"
                placeholder="nck_..."
              />
              {error && (
                <div className="mt-2 rounded-lg bg-red-50 border border-red-100 px-3 py-2 text-xs text-red-700">
                  {error}
                </div>
              )}
            </div>
            <button
              type="submit"
              disabled={submitting || !key.trim()}
              className="w-full bg-primary-600 text-white rounded-lg py-2 text-sm font-medium hover:bg-primary-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {submitting ? 'กำลังตรวจสอบ...' : 'เข้าสู่ระบบ'}
            </button>
            <p className="text-xs text-gray-400 leading-relaxed">
              รับ API key ได้จากคำสั่ง{' '}
              <code className="px-1 py-0.5 rounded bg-gray-100 text-gray-600 font-mono text-[11px]">
                nexclaim auth create-admin
              </code>{' '}
              หรือจาก admin ของโรงพยาบาล
            </p>
          </form>
        )}
      </div>
    </main>
  )
}
