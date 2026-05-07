import { useState, useEffect, KeyboardEvent } from 'react'
import { useStore } from '../store/jobs'
import { GetClipboardURL } from '../../wailsjs/go/main/App'

export function AddUrlBar() {
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [clipboardSuggestion, setClipboardSuggestion] = useState<string | null>(null)
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
        // Ignore — clipboard polling is best-effort.
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
    try {
      await addUrl(url)
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

      {clipboardSuggestion && clipboardSuggestion !== value && (
        <button
          onClick={() => submit(clipboardSuggestion)}
          className="text-left text-xs text-blue-400 hover:underline"
          title={clipboardSuggestion}
        >
          📋 Add from clipboard: {clipboardSuggestion}
        </button>
      )}
    </div>
  )
}
