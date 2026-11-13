import { SvelteMap } from 'svelte/reactivity'
import { FolderPictures } from '../../wailsjs/go/main/App'

export interface Picture {
  path: string
  modified: number
}

/** Pictures of folders by "path|modified". The time of a folder changes when
 *  items come or go, so a changed folder is asked again. */
const known = new SvelteMap<string, Picture[]>()
const pending = new Map<string, string>()
let timer: ReturnType<typeof setTimeout> | undefined

const BATCH_DELAY_MS = 40
const LIMIT = 4000

function keyOf(folder: { path: string; modified: number }): string {
  return `${folder.path}|${folder.modified}`
}

/**
 * The pictures a folder's icon shows, or undefined until they arrive. Asking
 * schedules a request, and the icons of a screen go together in one.
 */
export function folderPictures(folder: { path: string; modified: number }): Picture[] | undefined {
  const key = keyOf(folder)
  const found = known.get(key)
  if (found) return found
  if (!pending.has(key)) {
    pending.set(key, folder.path)
    timer ??= setTimeout(flush, BATCH_DELAY_MS)
  }
  return undefined
}

async function flush() {
  timer = undefined
  const batch = [...pending]
  pending.clear()
  if (batch.length === 0) return
  try {
    const result = await FolderPictures(batch.map(([, path]) => path))
    if (known.size > LIMIT) known.clear()
    for (const [key, path] of batch) known.set(key, result[path] ?? [])
  } catch {
    for (const [key] of batch) known.set(key, [])
  }
}
