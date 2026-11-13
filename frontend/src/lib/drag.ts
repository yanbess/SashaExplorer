import { StartDrag } from '../../wailsjs/go/main/App'
import { isInside, parentPath, samePath, THIS_PC } from './util'

export type TransferMode = 'copy' | 'move'
export type DropHandler = (paths: string[], dest: string, mode: TransferMode) => void

/** The page cannot read drag data before the drop, so the items wait here. */
let dragged: string[] = []
let native = false

/**
 * Starts a Windows drag of items. The page sees it as files dragged in from
 * outside, so the drop targets below still work while it is over the window,
 * and other programs can take the items too.
 */
export async function startNativeDrag(paths: string[]) {
  if (native || paths.length === 0) return
  dragged = paths
  native = true
  try {
    await StartDrag(paths)
  } catch {
    // A drag that could not start just does nothing.
  } finally {
    // The page may get the drop a moment after Windows ends the drag.
    await new Promise((resolve) => setTimeout(resolve, 300))
    native = false
    endDrag()
  }
}

/** Whether a drag of this window's items is going on. */
export function isNativeDrag(): boolean {
  return native
}

export function endDrag() {
  dragged = []
  for (const element of document.querySelectorAll('.drop-target')) element.classList.remove('drop-target')
}

export function draggedPaths(): string[] {
  return dragged
}

function volume(path: string): string {
  return (path.match(/^([a-z]:|\\\\[^\\]+\\[^\\]+)/i)?.[1] ?? '').toLowerCase()
}

/** Ctrl copies and Shift moves. Without them, Windows moves within one drive. */
export function transferMode(modifiers: { ctrlKey: boolean; shiftKey: boolean }, paths: string[], dest: string): TransferMode {
  if (modifiers.ctrlKey) return 'copy'
  if (modifiers.shiftKey) return 'move'
  return paths.length > 0 && volume(paths[0]) === volume(dest) ? 'move' : 'copy'
}

export function canDrop(paths: string[], dest: string, mode: TransferMode): boolean {
  if (paths.length === 0 || dest === THIS_PC) return false
  for (const path of paths) {
    if (samePath(path, dest) || isInside(dest, path)) return false
  }
  if (mode === 'move' && paths.every((path) => samePath(parentPath(path), dest))) return false
  return true
}

interface DropParams {
  path: string
  ondrop: DropHandler
  /** Lets an element keep part of itself for another kind of drop. */
  accept?: (event: DragEvent) => boolean
}

/** Svelte action: items from the list can be dropped on this element. */
export function dropTarget(node: HTMLElement, params: DropParams) {
  let current = params

  function over(event: DragEvent) {
    if (!current.path) return
    // Files from another program: show where they would go. The drop itself
    // goes on to the window, which receives their paths.
    if (dragged.length === 0) {
      if (!event.dataTransfer?.types.includes('Files')) return
      if (current.accept && !current.accept(event)) return
      event.preventDefault()
      event.stopPropagation()
      node.classList.add('drop-target')
      return
    }
    if (current.accept && !current.accept(event)) {
      node.classList.remove('drop-target')
      return
    }
    const mode = transferMode(event, dragged, current.path)
    if (!canDrop(dragged, current.path, mode)) return
    event.preventDefault()
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = mode
    node.classList.add('drop-target')
  }

  function leave(event: DragEvent) {
    if (!node.contains(event.relatedTarget as Node | null)) node.classList.remove('drop-target')
  }

  function drop(event: DragEvent) {
    node.classList.remove('drop-target')
    if (dragged.length === 0 || !current.path) return
    if (current.accept && !current.accept(event)) return
    const mode = transferMode(event, dragged, current.path)
    if (!canDrop(dragged, current.path, mode)) return
    event.preventDefault()
    event.stopPropagation()
    const paths = dragged
    endDrag()
    current.ondrop(paths, current.path, mode)
  }

  node.addEventListener('dragover', over)
  node.addEventListener('dragleave', leave)
  node.addEventListener('drop', drop)
  node.dataset.dropPath = current.path
  return {
    update(next: DropParams) {
      current = next
      node.dataset.dropPath = next.path
    },
    destroy() {
      node.removeEventListener('dragover', over)
      node.removeEventListener('dragleave', leave)
      node.removeEventListener('drop', drop)
    },
  }
}
