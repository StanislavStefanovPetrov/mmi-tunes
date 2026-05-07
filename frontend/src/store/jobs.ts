import { create } from 'zustand'
import {
  AddURL,
  AddURLForce,
  CancelAll,
  CancelJob,
  CheckTools,
  ClearCompleted,
  GetSettings,
  ListJobs,
  RemoveJob,
  SaveSettings,
  StartAll,
} from '../../wailsjs/go/main/App'
import type { queue, settings, tools } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'

type Job = queue.Job
type Settings = settings.Settings
type ToolStatus = tools.AllStatus

interface State {
  jobs: Record<string, Job>
  jobOrder: string[]
  settings: Settings | null
  tools: ToolStatus | null
  busyCount: number
  lastError: string | null

  init(): Promise<void>
  reload(): Promise<void>
  addUrl(rawURL: string, force?: boolean): Promise<void>
  removeJob(id: string): Promise<void>
  cancelJob(id: string): Promise<void>
  cancelAll(): Promise<void>
  startAll(): Promise<void>
  clearCompleted(): Promise<void>
  saveSettings(s: Settings): Promise<void>
  setError(msg: string | null): void
}

function indexJobs(list: Job[]): { jobs: Record<string, Job>; order: string[] } {
  const jobs: Record<string, Job> = {}
  const order: string[] = []
  for (const j of list) {
    jobs[j.id] = j
    order.push(j.id)
  }
  return { jobs, order }
}

export const useStore = create<State>((set, get) => ({
  jobs: {},
  jobOrder: [],
  settings: null,
  tools: null,
  busyCount: 0,
  lastError: null,

  async init() {
    const [jobsList, cfg, toolStatus] = await Promise.all([
      ListJobs(),
      GetSettings(),
      CheckTools(),
    ])
    const indexed = indexJobs(jobsList || [])
    set({ jobs: indexed.jobs, jobOrder: indexed.order, settings: cfg, tools: toolStatus })

    EventsOn('job:added', (j: Job) => {
      const { jobs, jobOrder } = get()
      if (!jobs[j.id]) {
        set({
          jobs: { ...jobs, [j.id]: j },
          jobOrder: [...jobOrder, j.id],
        })
      }
    })

    const updateJob = (j: Job) => {
      const { jobs } = get()
      set({ jobs: { ...jobs, [j.id]: j } })
    }

    EventsOn('job:status', updateJob)
    EventsOn('job:progress', updateJob)
    EventsOn('job:done', updateJob)
    EventsOn('job:error', updateJob)
    EventsOn('job:removed', (j: Job) => {
      const { jobs, jobOrder } = get()
      const next = { ...jobs }
      delete next[j.id]
      set({ jobs: next, jobOrder: jobOrder.filter((x) => x !== j.id) })
    })
  },

  async reload() {
    const list = await ListJobs()
    const indexed = indexJobs(list || [])
    set({ jobs: indexed.jobs, jobOrder: indexed.order })
  },

  async addUrl(rawURL, force = false) {
    set({ lastError: null })
    try {
      if (force) {
        await AddURLForce(rawURL)
      } else {
        await AddURL(rawURL)
      }
    } catch (e: any) {
      const msg = String(e?.message ?? e)
      set({ lastError: msg })
      throw e
    }
  },

  async removeJob(id) {
    await RemoveJob(id)
  },

  async cancelJob(id) {
    await CancelJob(id)
  },

  async cancelAll() {
    await CancelAll()
  },

  async startAll() {
    await StartAll()
  },

  async clearCompleted() {
    await ClearCompleted()
    await get().reload()
  },

  async saveSettings(s) {
    await SaveSettings(s)
    set({ settings: s })
  },

  setError(msg) {
    set({ lastError: msg })
  },
}))

export type { Job, Settings, ToolStatus }
