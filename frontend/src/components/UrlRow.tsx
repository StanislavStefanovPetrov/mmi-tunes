import { useStore, type Job } from '../store/jobs'
import { RevealInFinder } from '../../wailsjs/go/main/App'

const STATUS_LABEL: Record<string, string> = {
  queued: 'Queued',
  running: 'Downloading',
  done: 'Done',
  error: 'Error',
  cancelled: 'Cancelled',
}

const STATUS_COLOR: Record<string, string> = {
  queued: 'bg-neutral-600 text-neutral-100',
  running: 'bg-blue-600 text-white',
  done: 'bg-green-600 text-white',
  error: 'bg-red-600 text-white',
  cancelled: 'bg-neutral-500 text-neutral-200',
}

const STAGE_LABEL: Record<string, string> = {
  metadata: 'Fetching info',
  download: 'Downloading',
  convert: 'Converting',
  embed_metadata: 'Tagging',
  embed_thumbnail: 'Cover art',
  post_process: 'Resizing cover',
  done: 'Done',
}

function formatStage(j: Job): string {
  const stage = j.progress?.stage
  if (!stage) return STATUS_LABEL[j.status] ?? j.status
  return STAGE_LABEL[stage] ?? stage
}

export function UrlRow({ job }: { job: Job }) {
  const removeJob = useStore((s) => s.removeJob)
  const cancelJob = useStore((s) => s.cancelJob)
  const startJob = useStore((s) => s.startJob)

  const showProgress = job.status === 'running'
  const pct = Math.max(0, Math.min(100, job.progress?.percent ?? 0))

  return (
    <div className="rounded-md border border-neutral-700 bg-neutral-800/60 px-3 py-2">
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm text-white">
            {job.title || job.url}
          </div>
          <div className="flex items-center gap-2 text-xs text-neutral-400">
            <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide ${STATUS_COLOR[job.status] ?? ''}`}>
              {formatStage(job)}
            </span>
            <span className="truncate" title={job.url}>{job.url}</span>
          </div>
        </div>

        <div className="flex shrink-0 gap-1">
          {(job.status === 'queued' || job.status === 'error' || job.status === 'cancelled') && (
            <button
              onClick={() => startJob(job.id)}
              className="rounded bg-blue-600 px-2 py-1 text-xs text-white hover:bg-blue-500"
              title={job.status === 'error' || job.status === 'cancelled' ? 'Retry download' : 'Download this clip'}
            >
              {job.status === 'queued' ? '⬇' : '↻'}
            </button>
          )}
          {job.status === 'done' && job.output_path && (
            <button
              onClick={() => RevealInFinder(job.output_path!)}
              className="rounded bg-neutral-700 px-2 py-1 text-xs text-neutral-100 hover:bg-neutral-600"
              title="Reveal in Finder"
            >
              📁
            </button>
          )}
          {job.status === 'running' && (
            <button
              onClick={() => cancelJob(job.id)}
              className="rounded bg-neutral-700 px-2 py-1 text-xs text-neutral-100 hover:bg-red-700"
              title="Cancel"
            >
              ✕
            </button>
          )}
          <button
            onClick={() => removeJob(job.id)}
            className="rounded bg-neutral-700 px-2 py-1 text-xs text-neutral-300 hover:bg-red-700 hover:text-white"
            title="Remove"
          >
            🗑
          </button>
        </div>
      </div>

      {showProgress && (
        <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-neutral-700">
          <div
            className="h-full bg-blue-500 transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
      )}

      {job.status === 'error' && job.error && (
        <div className="mt-1 text-xs text-red-400">{job.error}</div>
      )}
    </div>
  )
}
