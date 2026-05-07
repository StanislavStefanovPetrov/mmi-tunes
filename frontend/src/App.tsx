import { useEffect, useState } from 'react'
import { useStore } from './store/jobs'
import { AddUrlBar } from './components/AddUrlBar'
import { UrlList } from './components/UrlList'
import { SettingsDrawer } from './components/SettingsDrawer'
import { DiagnosticsBanner } from './components/DiagnosticsBanner'
import { DownloadButton } from './components/DownloadButton'

function App() {
  const init = useStore((s) => s.init)
  const [settingsOpen, setSettingsOpen] = useState(false)

  useEffect(() => {
    init()
  }, [init])

  return (
    <div className="flex h-full flex-col bg-neutral-900 text-white">
      <header
        className="flex items-center justify-between border-b border-neutral-800 px-4 py-2"
        style={{ ['--wails-draggable' as any]: 'drag' }}
      >
        <div className="flex items-center gap-2 pl-16" style={{ ['--wails-draggable' as any]: 'drag' }}>
          <span className="text-base font-semibold">MMI Tunes</span>
        </div>
        <button
          onClick={() => setSettingsOpen(true)}
          className="rounded p-1.5 text-neutral-400 hover:bg-neutral-800 hover:text-white"
          title="Settings"
          style={{ ['--wails-draggable' as any]: 'no-drag' }}
        >
          ⚙
        </button>
      </header>

      <main className="flex flex-1 flex-col gap-3 overflow-hidden p-4">
        <DiagnosticsBanner />
        <AddUrlBar />
        <UrlList />
      </main>

      <DownloadButton />

      <SettingsDrawer open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
  )
}

export default App
