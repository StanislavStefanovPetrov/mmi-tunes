import { useEffect, useState } from 'react'
import { useStore, type Settings } from '../store/jobs'
import { GetSettings, PickFolder } from '../../wailsjs/go/main/App'

interface Props {
  open: boolean
  onClose: () => void
}

export function SettingsDrawer({ open, onClose }: Props) {
  const settings = useStore((s) => s.settings)
  const save = useStore((s) => s.saveSettings)
  const [draft, setDraft] = useState<Settings | null>(null)

  useEffect(() => {
    if (settings) setDraft({ ...settings })
  }, [settings])

  if (!open || !draft) return null

  const update = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    setDraft({ ...draft, [key]: value })
  }

  // After every save, re-read from disk so the UI shows the clamped values
  // (e.g. user typed 1000 kbps → backend clamps to 320, drawer should reflect that).
  const persist = async (next: Settings) => {
    setDraft(next)
    await save(next)
    const reloaded = await GetSettings()
    setDraft(reloaded)
  }

  const applyMMIPreset = () => {
    persist({ ...draft, bitrate: 320, sample_rate: 48000, channels: 2, thumbnail_max_px: 800 })
  }

  const pickFolder = async () => {
    const folder = await PickFolder()
    if (folder) persist({ ...draft, download_folder: folder })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-end bg-black/50" onClick={onClose}>
      <div
        className="h-full w-[420px] overflow-y-auto bg-neutral-900 p-5 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-white">Settings</h2>
          <button onClick={onClose} className="rounded p-1 text-neutral-400 hover:bg-neutral-800 hover:text-white">✕</button>
        </div>

        <Section title="Download folder">
          <div className="flex gap-2">
            <input
              readOnly
              value={draft.download_folder}
              className="flex-1 truncate rounded border border-neutral-700 bg-neutral-800 px-2 py-1 text-xs text-neutral-300"
              title={draft.download_folder}
            />
            <button
              onClick={pickFolder}
              className="rounded bg-neutral-700 px-2 py-1 text-xs text-white hover:bg-neutral-600"
            >
              Choose…
            </button>
          </div>
        </Section>

        <Section title="Audio quality">
          <button
            onClick={applyMMIPreset}
            className="mb-2 w-full rounded bg-blue-600 px-2 py-1 text-xs font-medium text-white hover:bg-blue-500"
            title="Set bitrate=320 kbps, sample rate=48 kHz, channels=stereo"
          >
            Apply Audi MMI Preset (320 / 48k / Stereo)
          </button>
          <NumberRow label="Bitrate (kbps)" value={draft.bitrate} min={64} max={320} step={8} onChange={(v) => persist({ ...draft, bitrate: v })} />
          <NumberRow label="Sample rate (Hz)" value={draft.sample_rate} min={22050} max={48000} step={50} onChange={(v) => persist({ ...draft, sample_rate: v })} />
          <SelectRow
            label="Channels"
            value={String(draft.channels)}
            options={[{ v: '1', l: 'Mono' }, { v: '2', l: 'Stereo' }]}
            onChange={(v) => persist({ ...draft, channels: parseInt(v, 10) })}
          />
        </Section>

        <Section title="Cover art">
          <NumberRow label="Max dimension (px)" value={draft.thumbnail_max_px} min={100} max={1000} step={50} onChange={(v) => persist({ ...draft, thumbnail_max_px: v })} />
          <BoolRow label="Embed thumbnail" checked={draft.embed_thumbnail} onChange={(v) => persist({ ...draft, embed_thumbnail: v })} />
          <BoolRow label="Embed metadata" checked={draft.embed_metadata} onChange={(v) => persist({ ...draft, embed_metadata: v })} />
        </Section>

        <Section title="Behaviour">
          <NumberRow label="Concurrent downloads" value={draft.concurrency} min={1} max={5} step={1} onChange={(v) => persist({ ...draft, concurrency: v })} />
          <BoolRow label="Cyrillic → Latin transliteration" checked={draft.transliterate} onChange={(v) => persist({ ...draft, transliterate: v })} />
          <BoolRow label="Skip already-downloaded videos" checked={draft.dedup_history} onChange={(v) => persist({ ...draft, dedup_history: v })} />
          <BoolRow label="Auto-detect YouTube URLs in clipboard" checked={draft.auto_detect_clipboard} onChange={(v) => persist({ ...draft, auto_detect_clipboard: v })} />
          <BoolRow label="Generate M3U playlist" checked={draft.generate_m3u} onChange={(v) => persist({ ...draft, generate_m3u: v })} />
        </Section>

        <p className="mt-4 text-[11px] text-neutral-500">
          Concurrency takes effect on next app launch. All other changes apply to new downloads immediately.
        </p>
      </div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-5 rounded-md border border-neutral-800 bg-neutral-900/50 p-3">
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-neutral-400">{title}</h3>
      <div className="space-y-2">{children}</div>
    </section>
  )
}

function NumberRow({ label, value, min, max, step, onChange }: { label: string; value: number; min: number; max: number; step: number; onChange: (v: number) => void }) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm text-neutral-200">
      <span>{label}</span>
      <input
        type="number"
        value={value}
        min={min}
        max={max}
        step={step}
        onChange={(e) => onChange(parseInt(e.target.value || '0', 10))}
        className="w-24 rounded border border-neutral-700 bg-neutral-800 px-2 py-1 text-right text-xs text-white outline-none focus:border-blue-500"
      />
    </label>
  )
}

function BoolRow({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm text-neutral-200">
      <span>{label}</span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 cursor-pointer accent-blue-600"
      />
    </label>
  )
}

function SelectRow({ label, value, options, onChange }: { label: string; value: string; options: { v: string; l: string }[]; onChange: (v: string) => void }) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm text-neutral-200">
      <span>{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-neutral-700 bg-neutral-800 px-2 py-1 text-xs text-white outline-none focus:border-blue-500"
      >
        {options.map((o) => (
          <option key={o.v} value={o.v}>{o.l}</option>
        ))}
      </select>
    </label>
  )
}
