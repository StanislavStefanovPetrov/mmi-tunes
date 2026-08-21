import { useState } from 'react'
import { useStore } from '../store/jobs'
import { CheckTools, UpdateYtDlp } from '../../wailsjs/go/main/App'

export function DiagnosticsBanner() {
  const tools = useStore((s) => s.tools)
  const [updating, setUpdating] = useState(false)
  const [updateMessage, setUpdateMessage] = useState<string | null>(null)

  if (!tools) return null

  const ytOk = tools.ytdlp.found
  const ffOk = tools.ffmpeg.found
  const jsOk = tools.jsruntime.found
  if (ytOk && ffOk && jsOk && !updateMessage) return null

  const refresh = async () => {
    const next = await CheckTools()
    useStore.setState({ tools: next })
  }

  const update = async () => {
    setUpdating(true)
    try {
      const out = await UpdateYtDlp()
      setUpdateMessage(out.split('\n').slice(-1)[0] || 'Updated.')
      await refresh()
    } catch (e: any) {
      setUpdateMessage(`Update failed: ${e?.message ?? e}`)
    } finally {
      setUpdating(false)
    }
  }

  return (
    <div className="rounded-md border border-yellow-700 bg-yellow-900/30 p-2 text-xs text-yellow-200">
      <div className="flex items-center justify-between gap-2">
        <div>
          {!ytOk && (
            <div>
              <strong>yt-dlp is missing.</strong> Install with <code className="rounded bg-neutral-800 px-1">brew install yt-dlp</code>.
            </div>
          )}
          {!ffOk && (
            <div>
              <strong>ffmpeg is missing.</strong> Install with <code className="rounded bg-neutral-800 px-1">brew install ffmpeg</code>.
            </div>
          )}
          {/* Unlike yt-dlp and ffmpeg, qjs is not a Homebrew package — it
              ships inside the app bundle, so a missing one means a broken
              install rather than something the user can go fetch. Without
              it YouTube returns no audio formats and every download fails. */}
          {!jsOk && (
            <div>
              <strong>JavaScript runtime is missing.</strong> Downloads will fail — YouTube
              needs it to decode audio formats. Reinstall MMI Tunes to restore it.
            </div>
          )}
          {updateMessage && <div className="mt-1">{updateMessage}</div>}
        </div>
        <div className="flex gap-2">
          {ytOk && (
            <button
              onClick={update}
              disabled={updating}
              className="rounded bg-yellow-700 px-2 py-1 text-[11px] text-white hover:bg-yellow-600 disabled:opacity-50"
            >
              {updating ? 'Updating…' : 'Update yt-dlp'}
            </button>
          )}
          <button
            onClick={refresh}
            className="rounded bg-neutral-700 px-2 py-1 text-[11px] text-white hover:bg-neutral-600"
          >
            Re-check
          </button>
        </div>
      </div>
    </div>
  )
}
