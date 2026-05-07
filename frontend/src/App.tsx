import { useEffect, useState } from 'react'
import { CheckTools, GetSettings } from '../wailsjs/go/main/App'
import type { tools, settings } from '../wailsjs/go/models'

function App() {
  const [toolStatus, setToolStatus] = useState<tools.AllStatus | null>(null)
  const [config, setConfig] = useState<settings.Settings | null>(null)

  useEffect(() => {
    CheckTools().then(setToolStatus)
    GetSettings().then(setConfig)
  }, [])

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 bg-neutral-900 p-8">
      <header className="text-center">
        <h1 className="text-3xl font-semibold tracking-tight text-white">MMI Tunes</h1>
        <p className="mt-1 text-sm text-neutral-400">
          Phase 3 sanity check — backend API + persistence
        </p>
      </header>

      <section className="w-full max-w-md rounded-md border border-neutral-700 bg-neutral-800 p-4 text-sm text-neutral-200">
        <h2 className="mb-2 font-semibold text-white">Tool diagnostics</h2>
        {toolStatus ? (
          <ul className="space-y-1">
            <li>
              yt-dlp: {toolStatus.ytdlp.found
                ? <span className="text-green-400">{toolStatus.ytdlp.version}</span>
                : <span className="text-red-400">not found — {toolStatus.ytdlp.error}</span>}
            </li>
            <li>
              ffmpeg: {toolStatus.ffmpeg.found
                ? <span className="text-green-400">{toolStatus.ffmpeg.version}</span>
                : <span className="text-red-400">not found — {toolStatus.ffmpeg.error}</span>}
            </li>
          </ul>
        ) : (
          <div className="text-neutral-500">Probing…</div>
        )}
      </section>

      <section className="w-full max-w-md rounded-md border border-neutral-700 bg-neutral-800 p-4 text-sm text-neutral-200">
        <h2 className="mb-2 font-semibold text-white">Settings (loaded from disk)</h2>
        {config ? (
          <pre className="overflow-auto text-xs text-neutral-300">{JSON.stringify(config, null, 2)}</pre>
        ) : (
          <div className="text-neutral-500">Loading…</div>
        )}
      </section>
    </div>
  )
}

export default App
