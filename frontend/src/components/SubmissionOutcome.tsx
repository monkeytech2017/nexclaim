import type { SubmitResponse, ValidationError, Submission } from '@/lib/api'
import { FormatBadge } from '@/components/ui/badges'

// Renders a pipeline.Outcome as a set of compact cards — used by both
// OPD batch-process and IPD import result panels.
export function OutcomePanel({ outcome, label }: { outcome: SubmitResponse; label?: string }) {
  return (
    <div className="border border-gray-100 rounded-lg bg-gray-50/50 p-3 space-y-2">
      <div className="flex items-center justify-between text-xs text-gray-600">
        <span>{label ?? `INSCL ${outcome.inscl}`}</span>
        <span>OPD: <b>{outcome.opdCount}</b> · IPD: <b>{outcome.ipdCount}</b></span>
      </div>

      {outcome.validationErrors && outcome.validationErrors.length > 0 && (
        <ValidationErrorList errors={outcome.validationErrors} />
      )}

      {outcome.submissions && outcome.submissions.length > 0 ? (
        <div className="space-y-1.5">
          {outcome.submissions.map((s, i) => (
            <SubmissionRow key={i} sub={s} />
          ))}
        </div>
      ) : (
        <p className="text-xs text-gray-400">ไม่มีข้อมูลพอสำหรับ submission</p>
      )}
    </div>
  )
}

function SubmissionRow({ sub }: { sub: Submission }) {
  return (
    <div className="bg-white border border-gray-100 rounded px-3 py-2 flex items-center justify-between text-xs">
      <div className="flex items-center gap-2.5">
        <FormatBadge format={sub.format} />
        <span className="font-mono text-gray-700">{sub.zipName ?? '—'}</span>
        <span className="text-gray-400">
          {sub.filesN > 0 ? `${sub.filesN} files` : null}
          {sub.xmlBytes > 0 ? `${(sub.xmlBytes / 1024).toFixed(1)}KB XML` : null}
          {sub.zipBytes > 0 ? ` · ${(sub.zipBytes / 1024).toFixed(1)}KB zip` : null}
        </span>
      </div>
      <div>
        {sub.error ? (
          <span className="text-red-600">{sub.error}</span>
        ) : sub.txnId ? (
          <span className="text-green-700">{sub.status ?? 'OK'} · {sub.txnId}</span>
        ) : (
          <span className="text-gray-400">dry-run</span>
        )}
      </div>
    </div>
  )
}

function ValidationErrorList({ errors }: { errors: ValidationError[] }) {
  const shown = errors.slice(0, 5)
  const rest = errors.length - shown.length
  return (
    <div className="bg-amber-50 border border-amber-100 rounded p-2">
      <div className="text-[11px] font-medium text-amber-900 mb-1">
        Validation {errors.length} issue(s)
      </div>
      <ul className="text-[11px] text-amber-800 space-y-0.5">
        {shown.map((e, i) => (
          <li key={i}>
            <code className="text-amber-900">{e.field}</code>
            {e.cCode && <span className="ml-1 text-amber-600">[{e.cCode}]</span>}
            {' '}— {e.reason}
          </li>
        ))}
        {rest > 0 && <li className="text-amber-600">... +{rest} more</li>}
      </ul>
    </div>
  )
}
