import { useState } from 'react'
import { Greet } from '../wailsjs/go/main/App'

function App() {
  const [name, setName] = useState('')
  const [greeting, setGreeting] = useState('')

  const greet = async () => {
    const result = await Greet(name || 'world')
    setGreeting(result)
  }

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 bg-neutral-900 p-8">
      <header className="text-center">
        <h1 className="text-3xl font-semibold tracking-tight text-white">MMI Tunes</h1>
        <p className="mt-1 text-sm text-neutral-400">
          YouTube → Audi MMI compatible MP3 — bootstrap stub
        </p>
      </header>

      <div className="flex w-full max-w-md gap-2">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="your name"
          className="flex-1 rounded-md border border-neutral-700 bg-neutral-800 px-3 py-2 text-sm text-white outline-none focus:border-neutral-500"
        />
        <button
          onClick={greet}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 active:bg-blue-700"
        >
          Greet
        </button>
      </div>

      {greeting && (
        <div className="text-sm text-neutral-200">{greeting}</div>
      )}
    </div>
  )
}

export default App
