import { useStore, type Job } from '../store/jobs'

export type TabKey = 'all' | 'queue' | 'active' | 'done' | 'failed'

// Which job statuses each tab shows. "all" is everything, so it has no entry.
//
// Cancelled sits with error rather than with queued: both are jobs that ended
// without producing a file, and both are what the retry button acts on.
const TAB_STATUSES: Record<Exclude<TabKey, 'all'>, string[]> = {
  queue: ['queued'],
  active: ['running'],
  done: ['done'],
  failed: ['error', 'cancelled'],
}

export function jobInTab(job: Job, tab: TabKey): boolean {
  if (tab === 'all') return true
  return TAB_STATUSES[tab].includes(job.status)
}

const TABS: { key: TabKey; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'queue', label: 'Queue' },
  { key: 'active', label: 'Active' },
  { key: 'done', label: 'Done' },
  { key: 'failed', label: 'Failed' },
]

export function Tabs({ active, onChange }: { active: TabKey; onChange: (tab: TabKey) => void }) {
  const jobs = useStore((s) => s.jobs)
  const order = useStore((s) => s.jobOrder)

  const counts = {} as Record<TabKey, number>
  for (const t of TABS) counts[t.key] = 0
  for (const id of order) {
    const j = jobs[id]
    if (!j) continue
    for (const t of TABS) {
      if (jobInTab(j, t.key)) counts[t.key]++
    }
  }

  return (
    <div className="flex shrink-0 gap-1 border-b border-neutral-800">
      {TABS.map((t) => {
        const isActive = t.key === active
        const count = counts[t.key]
        return (
          <button
            key={t.key}
            onClick={() => onChange(t.key)}
            className={`-mb-px flex items-center gap-1.5 border-b-2 px-3 py-1.5 text-xs transition-colors ${
              isActive
                ? 'border-blue-500 text-white'
                : 'border-transparent text-neutral-400 hover:text-neutral-200'
            }`}
          >
            <span>{t.label}</span>
            {count > 0 && (
              <span
                className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${
                  // Failures are the one count worth pulling the eye, so it
                  // stays red even when the tab is not selected.
                  t.key === 'failed'
                    ? 'bg-red-600 text-white'
                    : isActive
                      ? 'bg-neutral-700 text-white'
                      : 'bg-neutral-800 text-neutral-400'
                }`}
              >
                {count}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
