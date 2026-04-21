export default function DashboardPage() {
  return (
    <div className="flex flex-col gap-6">
      {/* Stats */}
      <div className="grid grid-cols-4 gap-4">
        {[
          { label: 'รอบเดือนนี้',    value: '1,284', sub: 'รายการทั้งหมด',    color: 'text-gray-900' },
          { label: 'ส่งสำเร็จ',      value: '1,201', sub: '93.5% สำเร็จ',    color: 'text-green-600' },
          { label: 'รอตรวจสอบ',     value: '72',    sub: 'มี C-code',        color: 'text-amber-500' },
          { label: 'ยังไม่ส่ง',       value: '11',    sub: 'เกิน deadline',    color: 'text-red-500'   },
        ].map(s => (
          <div key={s.label} className="bg-white rounded-xl border border-gray-100 p-4">
            <div className="text-xs text-gray-500 mb-1">{s.label}</div>
            <div className={`text-2xl font-semibold ${s.color}`}>{s.value}</div>
            <div className="text-xs text-gray-400 mt-1">{s.sub}</div>
          </div>
        ))}
      </div>

      {/* Batch status */}
      <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
          <span className="text-sm font-medium text-gray-900">รอบการส่งล่าสุด</span>
          <span className="text-xs text-gray-400">เม.ย. 2568</span>
        </div>
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 text-xs text-gray-500">
              <th className="px-5 py-3 text-left font-medium">สิทธิ</th>
              <th className="px-5 py-3 text-left font-medium">Format</th>
              <th className="px-5 py-3 text-right font-medium">รายการ</th>
              <th className="px-5 py-3 text-left font-medium">สถานะ</th>
              <th className="px-5 py-3 text-left font-medium">ส่งเมื่อ</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {[
              { inscl: 'UCS',  fmt: '16 แฟ้ม', count: '843',   status: 'sent',      sent: '21 เม.ย. 09:32' },
              { inscl: '011',  fmt: 'CIPN/CSOP',count: '201',  status: 'sent',      sent: '21 เม.ย. 09:35' },
              { inscl: 'SSS',  fmt: 'AIPN/SSOP',count: '156', status: 'c_code',    sent: '21 เม.ย. 09:40' },
              { inscl: 'LGO',  fmt: 'CIPN/CSOP',count: '84',  status: 'pending',   sent: '—' },
            ].map(row => (
              <tr key={row.inscl} className="hover:bg-gray-50 transition-colors">
                <td className="px-5 py-3">
                  <code className="text-xs bg-gray-100 text-gray-700 px-2 py-0.5 rounded">{row.inscl}</code>
                </td>
                <td className="px-5 py-3 text-gray-600">{row.fmt}</td>
                <td className="px-5 py-3 text-right text-gray-700 font-medium">{row.count}</td>
                <td className="px-5 py-3">
                  <StatusBadge status={row.status} />
                </td>
                <td className="px-5 py-3 text-gray-400 text-xs">{row.sent}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; cls: string }> = {
    sent:    { label: 'ส่งแล้ว',         cls: 'bg-green-50 text-green-700' },
    c_code:  { label: 'มี C-code',       cls: 'bg-amber-50 text-amber-700' },
    pending: { label: 'รอส่ง',           cls: 'bg-gray-100 text-gray-600'  },
    error:   { label: 'ผิดพลาด',         cls: 'bg-red-50 text-red-700'     },
  }
  const s = map[status] ?? { label: status, cls: 'bg-gray-100 text-gray-600' }
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${s.cls}`}>
      {s.label}
    </span>
  )
}
