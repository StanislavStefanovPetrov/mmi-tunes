import { useStore } from '../store/jobs'
import { UrlRow } from './UrlRow'

export function UrlList() {
  const jobOrder = useStore((s) => s.jobOrder)
  const jobs = useStore((s) => s.jobs)

  if (jobOrder.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center text-center text-sm text-neutral-500">
        <div className="text-2xl">🎵</div>
        <div className="mt-2">Paste a YouTube URL above to get started.</div>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="space-y-2 pr-1">
        {jobOrder.map((id) => {
          const j = jobs[id]
          if (!j) return null
          return <UrlRow key={id} job={j} />
        })}
      </div>
    </div>
  )
}
