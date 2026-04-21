export default function LoginPage() {
  return (
    <main className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="bg-white rounded-xl border border-gray-200 p-8 w-full max-w-sm shadow-sm">
        <div className="flex items-center gap-3 mb-8">
          <div className="w-10 h-10 rounded-xl bg-[#0F172A] flex items-center justify-center">
            <svg width="22" height="22" viewBox="0 0 34 34" fill="none">
              <rect x="4" y="10" width="18" height="14" rx="3" fill="#378ADD" opacity=".9"/>
              <rect x="12" y="6" width="18" height="14" rx="3" fill="#185FA5" opacity=".7"/>
              <path d="M8 17L13 22L22 13" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
          <div>
            <div className="font-semibold text-gray-900">NexClaim</div>
            <div className="text-xs text-gray-500">Healthcare Claim Middleware</div>
          </div>
        </div>
        <form className="flex flex-col gap-4">
          <div>
            <label className="block text-sm text-gray-600 mb-1">ชื่อผู้ใช้</label>
            <input
              type="text"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-400"
              placeholder="username"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-600 mb-1">รหัสผ่าน</label>
            <input
              type="password"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-400"
              placeholder="••••••••"
            />
          </div>
          <button
            type="submit"
            className="w-full bg-primary-600 text-white rounded-lg py-2 text-sm font-medium hover:bg-primary-800 transition-colors"
          >
            เข้าสู่ระบบ
          </button>
        </form>
      </div>
    </main>
  )
}
