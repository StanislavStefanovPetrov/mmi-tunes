import { useStore } from '../store/jobs'
import { UrlRow } from './UrlRow'
import { jobInTab, type TabKey } from './Tabs'

// What to say when a tab has nothing in it. Distinguishing these matters:
// "nothing failed" is good news, while "nothing here yet" on a fresh list is
// an instruction.
const EMPTY: Record<TabKey, { icon: string; text: string }> = {
  all: { icon: '🎵', text: 'Paste a YouTube URL above to get started.' },
  queue: { icon: '📋', text: 'Nothing waiting. Add a URL above to queue one up.' },
  active: { icon: '💤', text: 'Nothing downloading right now.' },
  done: { icon: '📁', text: 'No finished downloads yet.' },
  failed: { icon: '✅', text: 'Nothing failed.' },
}

export function UrlList({ tab }: { tab: TabKey }) {
  const jobOrder = useStore((s) => s.jobOrder)
  const jobs = useStore((s) => s.jobs)

  const visible = jobOrder.filter((id) => {
    const j = jobs[id]
    return j && jobInTab(j, tab)
  })

  if (visible.length === 0) {
    const { icon, text } = EMPTY[tab]
    return (
      <div className="flex flex-1 flex-col items-center justify-center text-center text-sm text-neutral-500">
        <div className="text-2xl">{icon}</div>
        <div className="mt-2">{text}</div>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="space-y-2 pr-1">
        {visible.map((id) => (
          <UrlRow key={id} job={jobs[id]} />
        ))}
      </div>
    </div>
  )
}
