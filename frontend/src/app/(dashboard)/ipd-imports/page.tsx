'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ipdApi, type IPDImportResult } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { OutcomePanel } from '@/components/SubmissionOutcome'
import { Play, RefreshCw, Folder, CheckCircle2, XCircle } from 'lucide-react'

export default function IPDImportsPage() {
  const qc = useQueryClient()
  const imports = useQuery({
    queryKey: ['ipd-imports'],
    queryFn: ipdApi.listImports,
    refetchInterval: 10_000,
  })

  const [results, setResults] = useState<Record<string, IPDImportResult | { error: string }>>({})

  const run = useMutation({
    mutationFn: ({ id, dryRun }: { id: string; dryRun: boolean }) =>
      ipdApi.runImport(id, dryRun),
    onSuccess: (data, vars) => {
      setResults(prev => ({ ...prev, [vars.id]: data }))
      qc.invalidateQueries({ queryKey: ['ipd-imports'] })
    },
    onError: (err, vars) => {
      const msg = err instanceof Error ? err.message : String(err)
      setResults(prev => ({ ...prev, [vars.id]: { error: msg } }))
    },
  })

  if (imports.isLoading) return <LoadingBlock />
  if (imports.isError) return <ErrorBlock error={imports.error} onRetry={() => imports.refetch()} />

  const list = imports.data?.imports ?? []

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-gray-500">
          อ่านจาก <code className="bg-gray-100 px-1.5 py-0.5 rounded">$IPD_SHARE_ROOT/incoming/</code>
          {' '}— หลัง import สำเร็จ folder จะย้ายไป <code className="bg-gray-100 px-1.5 py-0.5 rounded">processed/</code> หรือ <code className="bg-gray-100 px-1.5 py-0.5 rounded">error/</code>
        </p>
        <button
          onClick={() => imports.refetch()}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600"
          disabled={imports.isFetching}
        >
          <RefreshCw className={`w-3.5 h-3.5 ${imports.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
        </button>
      </div>

      {list.length === 0 ? (
        <EmptyBlock label="ไม่มี folder ใน incoming/ — ตั้ง IPD_SHARE_ROOT + วาง MANIFEST.json เพื่อเริ่มใช้งาน" />
      ) : (
        <div className="space-y-2">
          {list.map(entry => {
            const busy = run.isPending && run.variables?.id === entry.export_id
            const result = results[entry.export_id]
            return (
              <article key={entry.export_id} className="bg-white rounded-xl border border-gray-100 overflow-hidden">
                <div className="px-5 py-3 flex items-center gap-3 border-b border-gray-100">
                  <Folder className="w-4 h-4 text-gray-400" />
                  <div className="flex-1 min-w-0">
                    <div className="font-mono text-sm text-gray-900">{entry.export_id}</div>
                    <div className="font-mono text-[11px] text-gray-400 truncate">{entry.path}</div>
                  </div>
                  {entry.ready ? (
                    <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full bg-green-50 text-green-700">
                      <CheckCircle2 className="w-3 h-3" /> พร้อมนำเข้า
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full bg-gray-100 text-gray-500">
                      <XCircle className="w-3 h-3" /> ยังไม่มี MANIFEST
                    </span>
                  )}
                  <div className="space-x-1.5">
                    <button
                      disabled={!entry.ready || busy}
                      onClick={() => run.mutate({ id: entry.export_id, dryRun: true })}
                      className="text-xs px-2.5 py-1 rounded border border-gray-200 hover:bg-gray-50 text-gray-700 disabled:opacity-50"
                    >
                      Dry-run
                    </button>
                    <button
                      disabled={!entry.ready || busy}
                      onClick={() => run.mutate({ id: entry.export_id, dryRun: false })}
                      className="text-xs px-2.5 py-1 rounded bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1"
                    >
                      <Play className="w-3 h-3" /> Import
                    </button>
                  </div>
                </div>

                {result && (
                  <div className="px-5 py-3 space-y-2 bg-gray-50/40">
                    {'error' in result ? (
                      <div className="bg-red-50 border border-red-100 rounded p-2 text-xs text-red-800">
                        {result.error}
                      </div>
                    ) : (
                      <>
                        <p className="text-xs text-gray-600">
                          ผลการ import ({result.dryRun ? 'dry-run' : 'live'}) — {result.runs.length} bucket(s)
                        </p>
                        {result.moveError && (
                          <p className="text-xs text-amber-700">
                            moveError: {result.moveError}
                          </p>
                        )}
                        {result.runs.map((r, i) =>
                          r.outcome ? (
                            <OutcomePanel key={i} outcome={r.outcome} label={`INSCL ${r.inscl} · ${r.admit_count} admits`} />
                          ) : (
                            <div key={i} className="border border-red-100 bg-red-50 rounded p-2 text-xs text-red-800">
                              {r.inscl}: {r.error}
                            </div>
                          ),
                        )}
                      </>
                    )}
                  </div>
                )}
              </article>
            )
          })}
        </div>
      )}
    </div>
  )
}
