// Minimal CSV parser — handles comma, pipe, tab, semicolon delimiters.
//
// Not RFC 4180 compliant (no quoted fields with embedded delimiters). If
// users need that, swap for a library like papaparse. For the admin bulk
// imports we control the input shape — columns here are codes and
// identifiers, not free text.

const DELIMS = [',', '|', '\t', ';'] as const
type Delim = typeof DELIMS[number]

/** Auto-detect by counting occurrences in the first non-empty line. */
export function detectDelimiter(text: string): Delim {
  const firstLine = text.split(/\r?\n/).find(l => l.trim().length > 0) ?? ''
  let best: Delim = ','
  let bestCount = -1
  for (const d of DELIMS) {
    const c = firstLine.split(d).length - 1
    if (c > bestCount) { best = d; bestCount = c }
  }
  return best
}

export interface ParsedCSV {
  headers: string[]
  rows: Record<string, string>[]
  delimiter: Delim
}

/** Parse CSV-like text into header[] + row objects keyed by header name. */
export function parseCSV(text: string, delimiter?: Delim): ParsedCSV {
  const d = delimiter ?? detectDelimiter(text)
  const lines = text.split(/\r?\n/).filter(l => l.trim().length > 0)
  if (lines.length === 0) return { headers: [], rows: [], delimiter: d }

  const headers = lines[0].split(d).map(h => h.trim())
  const rows: Record<string, string>[] = []
  for (let i = 1; i < lines.length; i++) {
    const cols = lines[i].split(d)
    const row: Record<string, string> = {}
    headers.forEach((h, j) => {
      row[h] = (cols[j] ?? '').trim()
    })
    rows.push(row)
  }
  return { headers, rows, delimiter: d }
}
