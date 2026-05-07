import { useStore } from '../store/jobs'

export function DownloadButton() {
  const jobs = useStore((s) => s.jobs)
  const order = useStore((s) => s.jobOrder)
  const startAll = useStore((s) => s.startAll)
  const cancelAll = useStore((s) => s.cancelAll)
  const clearCompleted = useStore((s) => s.clearCompleted)
  const clearAll = useStore((s) => s.clearAll)

  let queued = 0, running = 0, done = 0, error = 0
  for (const id of order) {
    const j = jobs[id]
    if (!j) continue
    if (j.status === 'queued') queued++
    else if (j.status === 'running') running++
    else if (j.status === 'done') done++
    else if (j.status === 'error') error++
  }

  const total = order.length
  const anyRunning = running > 0

  return (
    <div className="flex items-center justify-between gap-2 border-t border-neutral-800 bg-neutral-900 px-3 py-2">
      <div className="text-xs text-neutral-400">
        {total === 0 ? '—' : (
          <>
            <span>{done}/{total} done</span>
            {error > 0 && <span className="ml-2 text-red-400">{error} error</span>}
            {running > 0 && <span className="ml-2 text-blue-400">{running} running</span>}
            {queued > 0 && <span className="ml-2">{queued} queued</span>}
          </>
        )}
      </div>
      <div className="flex gap-2">
        {total > 0 && (
          <button
            onClick={() => {
              if (confirm(`Remove all ${total} jobs from the list? Files already downloaded stay on disk.`)) {
                clearAll()
              }
            }}
            className="rounded bg-neutral-700 px-3 py-1.5 text-xs text-neutral-300 hover:bg-red-700 hover:text-white"
            title="Remove every job from the list (files on disk stay)"
          >
            Clear all
          </button>
        )}
        {done > 0 && !anyRunning && (
          <button
            onClick={() => clearCompleted()}
            className="rounded bg-neutral-700 px-3 py-1.5 text-xs text-white hover:bg-neutral-600"
          >
            Clear completed
          </button>
        )}
        {anyRunning ? (
          <button
            onClick={() => cancelAll()}
            className="rounded bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-500"
          >
            Cancel all
          </button>
        ) : (
          <button
            onClick={() => startAll()}
            disabled={queued + error === 0}
            className="rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:bg-neutral-700"
          >
            Download all
          </button>
        )}
      </div>
    </div>
  )
}
