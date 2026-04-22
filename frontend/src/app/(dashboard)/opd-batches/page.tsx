'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { opdApi, type ProcessBatchResponse, type Batch } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BatchStateBadge, InsclBadge } from '@/components/ui/badges'
import { OutcomePanel } from '@/components/SubmissionOutcome'
import { Play, RefreshCw, ChevronDown, ChevronRight } from 'lucide-react'

export default function OPDBatchesPage() {
  const qc = useQueryClient()
  const batches = useQuery({
    queryKey: ['opd-batches'],
    queryFn: opdApi.listBatches,
    refetchInterval: 10_000,
  })

  const [expanded, setExpanded] = useState<string | null>(null)
  const [outcomes, setOutcomes] = useState<Record<string, ProcessBatchResponse | { error: string }>>({})

  const process = useMutation({
    mutationFn: ({ batchId, dryRun }: { batchId: string; dryRun: boolean }) =>
      opdApi.processBatch(batchId, dryRun),
    onSuccess: (data, vars) => {
      setOutcomes(prev => ({ ...prev, [vars.batchId]: data }))
      qc.invalidateQueries({ queryKey: ['opd-batches'] })
    },
    onError: (err, vars) => {
      const msg = err instanceof Error ? err.message : String(err)
      setOutcomes(prev => ({ ...prev, [vars.batchId]: { error: msg } }))
    },
  })

  if (batches.isLoading) return <LoadingBlock />
  if (batches.isError) return <ErrorBlock error={batches.error} onRetry={() => batches.refetch()} />

  const list = batches.data?.batches ?? []
  if (list.length === 0) {
    return (
      <div className="space-y-3">
        <Toolbar onRefresh={() => batches.refetch()} isFetching={batches.isFetching} />
        <EmptyBlock label="ยังไม่มี batch — HIS จะ push ผ่าน POST /api/v1/his/opd/visits" />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <Toolbar onRefresh={() => batches.refetch()} isFetching={batches.isFetching} />
      <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 text-xs text-gray-500">
              <th className="w-6"></th>
              <th className="px-4 py-2.5 text-left font-medium">Batch ID</th>
              <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
              <th className="px-4 py-2.5 text-left font-medium">Period</th>
              <th className="px-4 py-2.5 text-left font-medium">Visits · INSCL</th>
              <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
              <th className="px-4 py-2.5 text-left font-medium">สร้าง</th>
              <th className="px-4 py-2.5 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {list.map(b => {
              const isOpen = expanded === b.batch_id
              const busy = process.isPending && process.variables?.batchId === b.batch_id
              const outcome = outcomes[b.batch_id]
              return (
                <BatchRow
                  key={b.batch_id}
                  batch={b}
                  isOpen={isOpen}
                  onToggle={() => setExpanded(isOpen ? null : b.batch_id)}
                  onProcess={dryRun => process.mutate({ batchId: b.batch_id, dryRun })}
                  busy={busy}
                  outcome={outcome}
                />
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Toolbar({ onRefresh, isFetching }: { onRefresh: () => void; isFetching: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <p className="text-xs text-gray-500">
        OPD Batches รับผ่าน <code className="bg-gray-100 px-1.5 py-0.5 rounded">POST /api/v1/his/opd/visits</code>
        · auto-refresh ทุก 10 วินาที
      </p>
      <button
        onClick={onRefresh}
        className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600"
        disabled={isFetching}
      >
        <RefreshCw className={`w-3.5 h-3.5 ${isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
      </button>
    </div>
  )
}

function BatchRow({
  batch, isOpen, onToggle, onProcess, busy, outcome,
}: {
  batch: Batch
  isOpen: boolean
  onToggle: () => void
  onProcess: (dryRun: boolean) => void
  busy: boolean
  outcome?: ProcessBatchResponse | { error: string }
}) {
  const inscls = Array.from(new Set(batch.visits?.map(v => v.inscl) ?? []))
  return (
    <>
      <tr className="hover:bg-gray-50 cursor-pointer" onClick={onToggle}>
        <td className="pl-3">
          {isOpen ? <ChevronDown className="w-4 h-4 text-gray-400" /> : <ChevronRight className="w-4 h-4 text-gray-400" />}
        </td>
        <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{batch.batch_id}</td>
        <td className="px-4 py-2.5 text-gray-600 font-mono">{batch.hospital_code}</td>
        <td className="px-4 py-2.5 text-gray-600">{batch.period}</td>
        <td className="px-4 py-2.5 flex items-center gap-2 flex-wrap">
          <span className="text-gray-700">{batch.visits?.length ?? 0}</span>
          {inscls.map(i => <InsclBadge key={i} inscl={i} />)}
        </td>
        <td className="px-4 py-2.5"><BatchStateBadge state={batch.state} /></td>
        <td className="px-4 py-2.5 text-xs text-gray-400">{formatDT(batch.created_at)}</td>
        <td className="px-4 py-2.5 text-right space-x-1.5" onClick={e => e.stopPropagation()}>
          <button
            disabled={busy}
            onClick={() => onProcess(true)}
            className="text-xs px-2.5 py-1 rounded border border-gray-200 hover:bg-gray-50 text-gray-700 disabled:opacity-50"
          >
            Dry-run
          </button>
          <button
            disabled={busy}
            onClick={() => onProcess(false)}
            className="text-xs px-2.5 py-1 rounded bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1"
          >
            <Play className="w-3 h-3" /> Submit
          </button>
        </td>
      </tr>
      {isOpen && (
        <tr>
          <td colSpan={8} className="bg-gray-50/60 px-8 py-4">
            {batch.last_error && (
              <p className="text-xs text-red-600 mb-2">last_error: {batch.last_error}</p>
            )}

            {outcome && 'error' in outcome && (
              <div className="bg-red-50 border border-red-100 rounded p-2 text-xs text-red-800 mb-2">
                {outcome.error}
              </div>
            )}
            {outcome && 'runs' in outcome && (
              <div className="space-y-2 mb-3">
                <p className="text-xs text-gray-600">
                  ผลการ process ({outcome.dryRun ? 'dry-run' : 'live'}) — {outcome.runs.length} bucket(s)
                </p>
                {outcome.runs.map((r, i) => (
                  <div key={i}>
                    {r.outcome ? (
                      <OutcomePanel outcome={r.outcome} label={`INSCL ${r.inscl} · ${r.vn_count} VN`} />
                    ) : (
                      <div className="border border-red-100 bg-red-50 rounded p-2 text-xs text-red-800">
                        {r.inscl}: {r.error}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            <details className="text-xs">
              <summary className="cursor-pointer text-gray-500 hover:text-gray-700">Visit summary JSON</summary>
              <pre className="mt-2 p-2 bg-white border border-gray-100 rounded overflow-auto max-h-64 text-[11px]">
                {JSON.stringify(batch.visits, null, 2)}
              </pre>
            </details>
          </td>
        </tr>
      )}
    </>
  )
}

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', { hour12: false, dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}
