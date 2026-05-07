import { useState, useEffect, KeyboardEvent } from 'react'
import { useStore } from '../store/jobs'
import {
  AddURLForce,
  GetClipboardURL,
  LookupHistory,
  RevealHistoryItem,
} from '../../wailsjs/go/main/App'
import type { history } from '../../wailsjs/go/models'

interface DedupPrompt {
  url: string
  record: history.Record
}

export function AddUrlBar() {
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [clipboardSuggestion, setClipboardSuggestion] = useState<string | null>(null)
  const [dedup, setDedup] = useState<DedupPrompt | null>(null)
  const addUrl = useStore((s) => s.addUrl)
  const settings = useStore((s) => s.settings)

  useEffect(() => {
    if (!settings?.auto_detect_clipboard) return
    let cancelled = false
    const tick = async () => {
      try {
        const url = await GetClipboardURL()
        if (cancelled) return
        if (url && url !== value) {
          setClipboardSuggestion(url)
        } else if (!url) {
          setClipboardSuggestion(null)
        }
      } catch {
        // best-effort
      }
    }
    tick()
    const id = setInterval(tick, 1500)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [settings?.auto_detect_clipboard, value])

  const submit = async (url: string) => {
    setError(null)
    setDedup(null)

    // Check history first so we can show a rich dedup prompt with the
    // existing file's path + a Reveal-in-Finder action.
    if (settings?.dedup_history) {
      try {
        const existing = await LookupHistory(url)
        if (existing && existing.video_id) {
          setDedup({ url, record: existing })
          return
        }
      } catch {
        // Fall through — invalid URL will be caught by AddURL below.
      }
    }

    try {
      await addUrl(url)
      setValue('')
      setClipboardSuggestion(null)
    } catch (e: any) {
      setError(String(e?.message ?? e))
    }
  }

  const downloadAnyway = async () => {
    if (!dedup) return
    try {
      await AddURLForce(dedup.url)
      setDedup(null)
      setValue('')
      setClipboardSuggestion(null)
    } catch (e: any) {
      setError(String(e?.message ?? e))
    }
  }

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && value.trim()) {
      submit(value.trim())
    }
  }

  const filename = (path: string) => path.split('/').pop() || path

  return (
    <div className="flex flex-col gap-1">
      <div className="flex gap-2">
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKey}
          placeholder="Paste a YouTube URL and press Enter"
          className="flex-1 rounded-md border border-neutral-700 bg-neutral-800 px-3 py-2 text-sm text-white outline-none placeholder:text-neutral-500 focus:border-blue-500"
        />
        <button
          onClick={() => value.trim() && submit(value.trim())}
          disabled={!value.trim()}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 active:bg-blue-700 disabled:cursor-not-allowed disabled:bg-neutral-700"
        >
          Add
        </button>
      </div>

      {error && <div className="text-xs text-red-400">{error}</div>}

      {clipboardSuggestion && clipboardSuggestion !== value && !dedup && (
        <button
          onClick={() => submit(clipboardSuggestion)}
          className="text-left text-xs text-blue-400 hover:underline"
          title={clipboardSuggestion}
        >
          📋 Add from clipboard: {clipboardSuggestion}
        </button>
      )}

      {dedup && (
        <div className="mt-1 rounded-md border border-yellow-700 bg-yellow-900/30 p-3 text-xs text-yellow-100">
          <div className="font-semibold text-yellow-200">
            Already downloaded
          </div>
          <div className="mt-1 truncate" title={dedup.record.title}>
            {dedup.record.title || filename(dedup.record.output_path)}
          </div>
          <div
            className="mt-0.5 truncate text-[11px] text-yellow-300/80"
            title={dedup.record.output_path}
          >
            {dedup.record.output_path}
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            <button
              onClick={() => RevealHistoryItem(dedup.record.video_id)}
              className="rounded bg-yellow-700 px-2 py-1 text-[11px] font-medium text-white hover:bg-yellow-600"
            >
              📁 Reveal in Finder
            </button>
            <button
              onClick={downloadAnyway}
              className="rounded bg-neutral-700 px-2 py-1 text-[11px] text-white hover:bg-neutral-600"
            >
              Download anyway
            </button>
            <button
              onClick={() => {
                setDedup(null)
                setValue('')
                setClipboardSuggestion(null)
              }}
              className="rounded bg-neutral-800 px-2 py-1 text-[11px] text-neutral-300 hover:bg-neutral-700"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
