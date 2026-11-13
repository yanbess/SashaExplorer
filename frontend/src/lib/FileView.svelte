<script lang="ts" module>
  import type { files } from '../../wailsjs/go/models'

  export type Entry = files.Entry
  export type SortKey = 'name' | 'modified' | 'created' | 'deleted' | 'type' | 'size'
  export type ViewMode = 'details' | 'icons' | 'gallery'

  export interface Selection {
    selected: string[]
    anchor: string
    focus: string
  }

  export type Command =
    | 'open'
    | 'openNewTab'
    | 'rename'
    | 'delete'
    | 'deleteForever'
    | 'copy'
    | 'cut'
    | 'paste'
    | 'copyPath'
    | 'properties'
    | 'restore'
    | 'up'

  export interface Columns {
    modified: number
    type: number
    size: number
    folder: number
  }

  export interface DriveInfo {
    kind: string
    total: number
    free: number
  }
</script>

<script lang="ts">
  import { tick, untrack } from 'svelte'
  import Icon from './Icon.svelte'
  import { dropTarget, startNativeDrag, type DropHandler } from './drag'
  import FileIcon from './FileIcon.svelte'
  import FolderThumb from './FolderThumb.svelte'
  import { formatDate, formatSize, hasOwnIcon, hasPicture, iconUrl, parentPath, thumbUrl } from './util'

  interface Props {
    path: string
    entries: Entry[]
    types: Record<string, string>
    view: ViewMode
    sortKey: SortKey
    sortDesc: boolean
    selection: Selection
    renaming: string
    cut: Set<string>
    tileSize: number
    columns: Columns
    drives?: Map<string, DriveInfo>
    /** Totals of folders, or null when folder sizes are off. */
    folderSizes: Record<string, number> | null
    /** The folder a search started in. Search results show where each item is. */
    searchRoot: string
    /** Icons drawn in the style of the app instead of the Windows ones. */
    themed: boolean
    /** The list is the Recycle Bin: items show where they were and when they
     *  were deleted, and stay inside. */
    bin: boolean
    loading: boolean
    error: string
    emptyText: string
    onselection: (selection: Selection) => void
    oncommand: (command: Command, entry?: Entry) => void
    onmenu: (event: MouseEvent, entry: Entry | null) => void
    onrename: (path: string, name: string | null) => void
    onsort: (key: SortKey) => void
    oncolumns: (columns: Columns) => void
    ontilesize: (size: number) => void
    onfilterkey: (key: string) => void
    onretry: () => void
    ondropfiles: DropHandler
  }

  let {
    path,
    entries,
    types,
    view,
    sortKey,
    sortDesc,
    selection,
    renaming,
    cut,
    tileSize,
    columns,
    drives,
    folderSizes,
    searchRoot,
    themed,
    bin,
    loading,
    error,
    emptyText,
    onselection,
    oncommand,
    onmenu,
    onrename,
    onsort,
    oncolumns,
    ontilesize,
    onfilterkey,
    onretry,
    ondropfiles,
  }: Props = $props()

  const ROW = 28
  const HEADER = 30
  const GAP = 8
  const PAD = 12
  /** The two lines of a name and the padding of a tile, without the spacing. */
  const CAPTION = 38
  const OVERSCAN = 6

  let scroller = $state<HTMLElement>()
  let scrollTop = $state(0)
  let height = $state(0)
  let width = $state(0)

  let selected = $derived(new Set(selection.selected))
  let indexOf = $derived(new Map(entries.map((entry, index) => [entry.path, index])))

  /** Icons and gallery are grids of tiles; icons have a fixed medium size. */
  let grid = $derived(view !== 'details')
  let tileWidth = $derived(view === 'icons' ? 108 : tileSize)
  /** Gallery pictures are 4:3. Most photos and screenshots are wider than
   *  tall, so a square left empty room above them. */
  let thumbHeight = $derived(view === 'icons' ? 64 : Math.round(tileSize * 0.75))
  /** Room between an icon and its name: sideways in a row, under a tile. */
  let spacing = $derived(view === 'details' ? 14 : 12)
  let cols = $derived(grid ? Math.max(1, Math.floor((width - PAD * 2 + GAP) / (tileWidth + GAP))) : 1)
  let caption = $derived(CAPTION + spacing)
  let rowHeight = $derived(grid ? thumbHeight + caption + GAP : ROW)
  let rowCount = $derived(Math.ceil(entries.length / cols))
  let top = $derived(grid ? PAD : HEADER)
  let firstRow = $derived(Math.max(0, Math.floor((scrollTop - top) / rowHeight) - OVERSCAN))
  let lastRow = $derived(Math.min(rowCount, Math.ceil((scrollTop - top + height) / rowHeight) + OVERSCAN))
  let shown = $derived(entries.slice(firstRow * cols, lastRow * cols).map((entry, i) => ({ entry, index: firstRow * cols + i })))
  /** Thumbnails come in a few sizes, so the window can reuse them. */
  let thumbSize = $derived(view === 'icons' ? 128 : tileSize * devicePixelRatio <= 256 ? 256 : 512)
  let gridColumns = $derived(
    `minmax(120px, 1fr) ${searchRoot || bin ? `${track(columns.folder)} ` : ''}${track(columns.modified)} ${track(columns.type)} ${track(columns.size)}`,
  )

  /** A column keeps its width while there is room and gives way in a narrow
   *  pane, so the name stays readable. */
  function track(width: number): string {
    return `minmax(${Math.min(width, 70)}px, ${width}px)`
  }

  /** A new folder starts at the top. */
  $effect(() => {
    void path
    untrack(() => {
      if (scroller) scroller.scrollTop = 0
      scrollTop = 0
    })
  })

  export function focusList() {
    scroller?.focus({ preventScroll: true })
  }

  export async function reveal(target: string) {
    await tick()
    const index = indexOf.get(target)
    if (index === undefined || !scroller) return
    const row = Math.floor(index / cols)
    const rowTop = top + row * rowHeight
    const viewTop = scroller.scrollTop + (view === 'details' ? HEADER : 0)
    const viewBottom = scroller.scrollTop + scroller.clientHeight
    if (rowTop < viewTop) scroller.scrollTop = rowTop - (view === 'details' ? HEADER : PAD)
    else if (rowTop + rowHeight > viewBottom) scroller.scrollTop = rowTop + rowHeight - scroller.clientHeight + (grid ? PAD : 0)
  }

  function typeName(entry: Entry): string {
    if (drives?.has(entry.path)) return driveKind(drives.get(entry.path)!.kind)
    if (entry.dir) return 'File folder'
    return types[entry.ext] || (entry.ext ? `${entry.ext.slice(1).toUpperCase()} file` : 'File')
  }

  function driveKind(kind: string): string {
    return (
      { removable: 'Removable disk', network: 'Network drive', cd: 'CD drive', ram: 'RAM disk' }[kind] ?? 'Local disk'
    )
  }

  function sizeText(entry: Entry): string {
    const drive = drives?.get(entry.path)
    if (drive) return drive.total ? `${formatSize(drive.free)} free` : ''
    // The Recycle Bin knows the size of a deleted folder.
    if (!entry.dir || bin) return formatSize(entry.size)
    if (!folderSizes) return ''
    const bytes = folderSizes[entry.path]
    return bytes === undefined ? '…' : formatSize(bytes)
  }

  /** Where a search result is, from the folder the search started in. */
  function folderText(entry: Entry): string {
    const folder = parentPath(entry.path)
    const root = searchRoot.replace(/\\$/, '')
    if (folder.toLowerCase() === root.toLowerCase()) return '.'
    if (folder.toLowerCase().startsWith(root.toLowerCase() + '\\')) return folder.slice(root.length + 1)
    return folder
  }

  function select(next: string[], anchor: string, focus: string) {
    onselection({ selected: next, anchor, focus })
  }

  function range(from: string, to: string): string[] {
    const a = indexOf.get(from) ?? 0
    const b = indexOf.get(to) ?? 0
    return entries.slice(Math.min(a, b), Math.max(a, b) + 1).map((entry) => entry.path)
  }

  function itemMouseDown(event: MouseEvent, entry: Entry) {
    if (event.button === 1) {
      event.preventDefault()
      return
    }
    if (event.button !== 0 && event.button !== 2) return
    scroller?.focus({ preventScroll: true })
    if (event.button === 0 && renaming !== entry.path) press = { x: event.clientX, y: event.clientY, entry }
    if (event.button === 2) {
      if (!selected.has(entry.path)) select([entry.path], entry.path, entry.path)
      return
    }
    if (event.shiftKey) {
      const span = range(selection.anchor || entry.path, entry.path)
      const next = event.ctrlKey ? [...new Set([...selection.selected, ...span])] : span
      select(next, selection.anchor || entry.path, entry.path)
    } else if (event.ctrlKey) {
      const next = selected.has(entry.path)
        ? selection.selected.filter((item) => item !== entry.path)
        : [...selection.selected, entry.path]
      select(next, entry.path, entry.path)
    } else if (!selected.has(entry.path)) {
      select([entry.path], entry.path, entry.path)
    }
  }

  /** A plain click on a selected item keeps the others until the button is up,
   *  so a drag can take them all. */
  function itemClick(event: MouseEvent, entry: Entry) {
    if (event.shiftKey || event.ctrlKey) return
    if (selection.selected.length > 1 || selection.focus !== entry.path) select([entry.path], entry.path, entry.path)
  }

  function itemAuxClick(event: MouseEvent, entry: Entry) {
    if (event.button === 1 && entry.dir) {
      event.preventDefault()
      oncommand('openNewTab', entry)
    }
  }

  function backgroundMouseDown(event: MouseEvent) {
    if (event.target !== event.currentTarget && !(event.target as HTMLElement).classList.contains('canvas')) return
    if (event.button === 0 && !event.ctrlKey && !event.shiftKey) select([], '', '')
    if (event.button === 2) select([], '', '')
  }

  function contextMenu(event: MouseEvent, entry: Entry | null) {
    event.preventDefault()
    event.stopPropagation()
    onmenu(event, entry)
  }

  /** A drag of items is a Windows drag, so they can also leave the window for
   *  Explorer or any other program. It starts once the mouse moves a little
   *  with the button down. */
  let press: { x: number; y: number; entry: Entry } | null = null
  const DRAG_DISTANCE = 5

  function pressMove(event: MouseEvent) {
    if (!press) return
    if ((event.buttons & 1) === 0) {
      press = null
      return
    }
    if (Math.hypot(event.clientX - press.x, event.clientY - press.y) < DRAG_DISTANCE || renaming) return
    const entry = press.entry
    press = null
    if (drives?.has(entry.path) || bin) return
    const paths = selected.has(entry.path) ? selection.selected : [entry.path]
    if (!selected.has(entry.path)) select([entry.path], entry.path, entry.path)
    startNativeDrag(paths)
  }

  function move(delta: number, event: KeyboardEvent) {
    if (entries.length === 0) return
    const current = indexOf.get(selection.focus)
    let index = current === undefined ? (delta > 0 ? 0 : entries.length - 1) : current + delta
    index = Math.max(0, Math.min(entries.length - 1, index))
    const target = entries[index].path
    if (event.shiftKey) {
      const anchor = selection.anchor || selection.focus || target
      select(range(anchor, target), anchor, target)
    } else if (event.ctrlKey) {
      select(selection.selected, selection.anchor, target)
    } else {
      select([target], target, target)
    }
    reveal(target)
  }

  function pageSize(): number {
    return Math.max(1, Math.floor((height - top) / rowHeight)) * cols
  }

  function keydown(event: KeyboardEvent) {
    if (renaming) return
    const ctrl = event.ctrlKey && !event.altKey
    const focusEntry = entries[indexOf.get(selection.focus) ?? -1]
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        return move(cols, event)
      case 'ArrowUp':
        event.preventDefault()
        return move(-cols, event)
      case 'ArrowRight':
        if (!grid) return
        event.preventDefault()
        return move(1, event)
      case 'ArrowLeft':
        if (!grid) return
        event.preventDefault()
        return move(-1, event)
      case 'PageDown':
        event.preventDefault()
        return move(pageSize(), event)
      case 'PageUp':
        event.preventDefault()
        return move(-pageSize(), event)
      case 'Home':
        event.preventDefault()
        return move(-entries.length, event)
      case 'End':
        event.preventDefault()
        return move(entries.length, event)
      case ' ':
        if (ctrl && focusEntry) {
          event.preventDefault()
          const next = selected.has(focusEntry.path)
            ? selection.selected.filter((item) => item !== focusEntry.path)
            : [...selection.selected, focusEntry.path]
          select(next, focusEntry.path, focusEntry.path)
          return
        }
        break
      case 'Enter':
        event.preventDefault()
        return oncommand(event.ctrlKey ? 'openNewTab' : 'open')
      case 'F2':
        event.preventDefault()
        return oncommand('rename')
      case 'Delete':
        event.preventDefault()
        return oncommand(event.shiftKey ? 'deleteForever' : 'delete')
      case 'Backspace':
        event.preventDefault()
        return oncommand('up')
      case 'Escape':
        if (selection.selected.length > 0) {
          event.preventDefault()
          select([], '', selection.focus)
        }
        return
    }
    if (ctrl) {
      const key = event.code
      if (key === 'KeyA') {
        event.preventDefault()
        select(entries.map((entry) => entry.path), selection.anchor, selection.focus)
      } else if (key === 'KeyC' && event.shiftKey) {
        event.preventDefault()
        oncommand('copyPath')
      } else if (key === 'KeyC') {
        event.preventDefault()
        oncommand('copy')
      } else if (key === 'KeyX') {
        event.preventDefault()
        oncommand('cut')
      } else if (key === 'KeyV') {
        event.preventDefault()
        oncommand('paste')
      }
      return
    }
    if (event.altKey && event.key === 'Enter') {
      event.preventDefault()
      oncommand('properties')
      return
    }
    if (event.key.length === 1 && !event.altKey && !event.metaKey && event.key !== ' ') {
      event.preventDefault()
      onfilterkey(event.key)
    }
  }

  function wheel(event: WheelEvent) {
    if (!event.ctrlKey || view !== 'gallery') return
    event.preventDefault()
    ontilesize(Math.max(96, Math.min(320, tileSize + (event.deltaY < 0 ? 16 : -16))))
  }

  /** Dragging the right edge of a header changes the width of its column. */
  function resizeColumn(event: PointerEvent, key: keyof Columns) {
    event.preventDefault()
    event.stopPropagation()
    const handle = event.currentTarget as HTMLElement
    handle.setPointerCapture(event.pointerId)
    const startX = event.clientX
    const startWidth = columns[key]
    const moveHandler = (moveEvent: PointerEvent) => {
      // The handle sits on the left edge of the column it resizes.
      const next = Math.max(60, Math.min(600, startWidth - (moveEvent.clientX - startX)))
      oncolumns({ ...columns, [key]: next })
    }
    const up = () => {
      handle.removeEventListener('pointermove', moveHandler)
      handle.removeEventListener('pointerup', up)
    }
    handle.addEventListener('pointermove', moveHandler)
    handle.addEventListener('pointerup', up)
  }

  function renameInput(input: HTMLInputElement) {
    input.focus()
    const entry = entries.find((item) => item.path === renaming)
    const dot = input.value.lastIndexOf('.')
    if (entry && !entry.dir && dot > 0) input.setSelectionRange(0, dot)
    else input.select()
  }

  let renameDone = false
  $effect(() => {
    void renaming
    renameDone = false
  })

  function renameKey(event: KeyboardEvent, entry: Entry) {
    event.stopPropagation()
    if (event.key === 'Enter') {
      event.preventDefault()
      renameDone = true
      onrename(entry.path, (event.currentTarget as HTMLInputElement).value)
      focusList()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      renameDone = true
      onrename(entry.path, null)
      focusList()
    }
  }

  function renameBlur(event: FocusEvent, entry: Entry) {
    if (renameDone) return
    renameDone = true
    onrename(entry.path, (event.currentTarget as HTMLInputElement).value)
  }

  function imageFailed(event: Event) {
    const image = event.currentTarget as HTMLImageElement
    image.style.visibility = 'hidden'
    image.parentElement?.classList.add('no-image')
  }

  /** The date column shows the creation date while the list is sorted by it. */
  let byCreated = $derived(sortKey === 'created')

  /** The drawn icons leave room under their shapes, out of a 24 unit box: a
   *  file ends at 21.4, a folder at 19.7, a drive at 20. That room is taken
   *  back, so the spacing is what one sees between the icon and its name. */
  const TRIM = { file: 2.6 / 24, folder: 4.3 / 24, drive: 4 / 24 }

  let headers: { key: SortKey | 'folder'; label: string; resize?: keyof Columns }[] = $derived([
    { key: 'name', label: 'Name' },
    ...(bin
      ? [{ key: 'folder' as const, label: 'Original location', resize: 'folder' as const }]
      : searchRoot
        ? [{ key: 'folder' as const, label: 'Folder', resize: 'folder' as const }]
        : []),
    bin
      ? { key: 'deleted', label: 'Date deleted', resize: 'modified' }
      : byCreated
      ? { key: 'created', label: 'Date created', resize: 'modified' }
      : { key: 'modified', label: 'Date modified', resize: 'modified' },
    { key: 'type', label: 'Type', resize: 'type' },
    { key: 'size', label: 'Size', resize: 'size' },
  ])
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="scroller"
  class:gallery={grid}
  role="listbox"
  aria-multiselectable="true"
  tabindex="0"
  bind:this={scroller}
  bind:clientHeight={height}
  bind:clientWidth={width}
  onscroll={() => (scrollTop = scroller?.scrollTop ?? 0)}
  onkeydown={keydown}
  onmousedown={backgroundMouseDown}
  oncontextmenu={(event) => contextMenu(event, null)}
  onwheel={wheel}
  onmousemove={pressMove}
  onmouseup={() => (press = null)}
  use:dropTarget={{ path, ondrop: ondropfiles }}
>
  {#if view === 'details'}
    <div class="header" style:grid-template-columns={gridColumns}>
      {#each headers as header (header.key)}
        <button type="button" class="head" class:sorted={sortKey === header.key} onclick={() => header.key !== 'folder' && onsort(header.key)}>
          {#if header.resize}
            <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
            <span
              class="resize"
              onpointerdown={(event) => resizeColumn(event, header.resize!)}
              onclick={(event) => event.stopPropagation()}
            ></span>
          {/if}
          <span class="head-label">{header.label}</span>
          {#if sortKey === header.key}
            <Icon name={sortDesc ? 'down' : 'up'} size={12} />
          {/if}
        </button>
      {/each}
    </div>
  {/if}

  <div class="canvas" style:height={`${rowCount * rowHeight + (grid ? PAD * 2 : 0)}px`}>
    {#each shown as { entry, index } (entry.path)}
      {@const row = Math.floor(index / cols)}
      {@const col = index % cols}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        class={view === 'details' ? 'row' : 'tile'}
        class:selected={selected.has(entry.path)}
        class:focused={selection.focus === entry.path}
        class:hidden-item={entry.hidden}
        class:cut={cut.has(entry.path)}
        role="option"
        tabindex="-1"
        aria-selected={selected.has(entry.path)}
        style:top={view === 'details' ? `${row * ROW}px` : `${PAD + row * rowHeight}px`}
        style:left={grid ? `${PAD + col * (tileWidth + GAP)}px` : undefined}
        style:width={grid ? `${tileWidth}px` : undefined}
        style:height={grid ? `${thumbHeight + caption}px` : undefined}
        style:grid-template-columns={view === 'details' ? gridColumns : undefined}
        draggable="false"
        title={grid ? (searchRoot ? entry.path : entry.name) : undefined}
        onmousedown={(event) => itemMouseDown(event, entry)}
        onclick={(event) => itemClick(event, entry)}
        onauxclick={(event) => itemAuxClick(event, entry)}
        ondblclick={() => renaming !== entry.path && oncommand('open', entry)}
        oncontextmenu={(event) => contextMenu(event, entry)}
        ondragstart={(event) => event.preventDefault()}
        use:dropTarget={{ path: entry.dir && !bin ? entry.path : '', ondrop: ondropfiles }}
      >
        {#if view === 'details'}
          <span class="name-cell" style:gap={`${spacing}px`}>
            {#if themed && !hasOwnIcon(entry)}
              <FileIcon {entry} drive={drives?.has(entry.path)} size={18} />
            {:else}
              <img class="small-icon" src={iconUrl(entry, 32)} alt="" draggable="false" onerror={imageFailed} />
            {/if}
            {#if renaming === entry.path}
              <input
                class="rename"
                value={entry.name}
                spellcheck="false"
                use:renameInput
                onkeydown={(event) => renameKey(event, entry)}
                onblur={(event) => renameBlur(event, entry)}
                onmousedown={(event) => event.stopPropagation()}
                ondblclick={(event) => event.stopPropagation()}
              />
            {:else}
              <span class="name">{entry.name}</span>
            {/if}
          </span>
          {#if bin}
            <span class="cell muted" title={entry.original}>{parentPath(entry.original ?? '')}</span>
          {:else if searchRoot}
            <span class="cell muted" title={parentPath(entry.path)}>{folderText(entry)}</span>
          {/if}
          <span class="cell muted">{formatDate(bin ? (entry.deleted ?? 0) : byCreated ? entry.created : entry.modified)}</span>
          <span class="cell muted">{typeName(entry)}</span>
          <span class="cell muted size">{sizeText(entry)}</span>
        {:else}
          <span class="thumb" class:small={view === 'icons'} style:height={`${thumbHeight}px`}>
            {#if drives?.has(entry.path)}
              {@const drive = drives.get(entry.path)!}
              {#if themed}
                {@const size = Math.round(thumbHeight * 0.55)}
                <span class="glyph" style:margin-bottom={`${-size * TRIM.drive}px`}><FileIcon {entry} drive {size} /></span>
              {:else}
                <img class="drive-icon" src={iconUrl(entry, 96)} alt="" draggable="false" onerror={imageFailed} />
              {/if}
              {#if drive.total > 0}
                <span class="capacity" title={`${formatSize(drive.free)} free of ${formatSize(drive.total)}`}>
                  <span style:width={`${(1 - drive.free / drive.total) * 100}%`}></span>
                </span>
              {/if}
            {:else if themed && entry.dir}
              {@const size = Math.round(Math.min(thumbHeight * 0.8, 220))}
              <span class="glyph" style:margin-bottom={`${-size * TRIM.folder}px`}>
                {#if bin}
                  <FileIcon {entry} {size} />
                {:else}
                  <FolderThumb folder={entry} {size} />
                {/if}
              </span>
            {:else if themed && !hasPicture(entry) && !hasOwnIcon(entry)}
              {@const size = Math.round(Math.min(thumbHeight * 0.72, 140))}
              <span class="glyph" style:margin-bottom={`${-size * TRIM.file}px`}><FileIcon {entry} {size} /></span>
            {:else}
              <img
                src={thumbUrl(entry.path, thumbSize, entry.modified)}
                alt=""
                loading="lazy"
                draggable="false"
                onerror={imageFailed}
              />
            {/if}
          </span>
          {#if renaming === entry.path}
            <input
              class="rename"
              value={entry.name}
              spellcheck="false"
              use:renameInput
              onkeydown={(event) => renameKey(event, entry)}
              onblur={(event) => renameBlur(event, entry)}
              onmousedown={(event) => event.stopPropagation()}
              ondblclick={(event) => event.stopPropagation()}
            />
          {:else}
            <span class="caption" style:margin-top={`${spacing}px`}>{entry.name}</span>
          {/if}
        {/if}
      </div>
    {/each}
  </div>

  {#if error}
    <div class="message">
      <p class="error">{error}</p>
      <button type="button" onclick={onretry}>Try again</button>
    </div>
  {:else if entries.length === 0 && !loading}
    <div class="message"><p>{emptyText}</p></div>
  {/if}
</div>

<style>
  .scroller {
    position: relative;
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    background: var(--bg);
    user-select: none;
  }

  .scroller:focus {
    outline: none;
  }

  .header {
    position: sticky;
    z-index: 2;
    top: 0;
    display: grid;
    height: 30px;
    min-width: 300px;
    border-bottom: 1px solid var(--border);
    background: var(--bg);
  }

  .head {
    position: relative;
    display: flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    padding: 0 10px;
    overflow: hidden;
    border: none;
    border-radius: 0;
    background: transparent;
    color: var(--text-muted);
    font-size: 12px;
    text-align: left;
  }

  .head:hover {
    background: var(--surface);
    color: var(--text);
  }

  .head.sorted {
    color: var(--text);
  }

  .head :global(svg) {
    color: var(--accent);
  }

  .head-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .resize {
    position: absolute;
    top: 6px;
    bottom: 6px;
    left: 0;
    width: 7px;
    border-left: 1px solid var(--border);
    cursor: col-resize;
  }

  .canvas {
    position: relative;
    min-width: 300px;
  }

  .gallery .canvas,
  .gallery .header {
    min-width: 0;
  }

  .row {
    position: absolute;
    right: 0;
    left: 0;
    display: grid;
    align-items: center;
    height: 28px;
    margin: 0 4px;
    border-radius: 5px;
    cursor: default;
  }

  .row:hover,
  .tile:hover {
    background: var(--surface);
  }

  .row.selected,
  .tile.selected {
    background: var(--selected);
  }

  .row.selected:hover,
  .tile.selected:hover {
    background: var(--selected-strong);
  }

  .scroller:focus-visible .focused,
  .scroller:focus .focused.selected {
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .hidden-item,
  .cut {
    opacity: 0.55;
  }

  .name-cell {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding: 0 8px;
  }

  .small-icon {
    flex: none;
    width: 18px;
    height: 18px;
  }

  .name,
  .cell {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cell {
    padding: 0 10px;
    font-size: 13px;
  }

  .muted {
    color: var(--text-muted);
  }

  .size {
    text-align: right;
  }

  .rename {
    flex: 1;
    height: 24px;
    padding: 0 6px;
    border-color: var(--accent);
    border-radius: 4px;
    user-select: text;
  }

  .tile {
    position: absolute;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 4px;
    border-radius: 8px;
    cursor: default;
  }

  /* Pictures and icons stand on the name, so the spacing is the only room
     between them. */
  .thumb {
    position: relative;
    display: flex;
    align-items: flex-end;
    justify-content: center;
    width: 100%;
    overflow: hidden;
    border-radius: 6px;
  }

  .thumb img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border-radius: 4px;
  }

  .thumb.small img {
    max-width: 64px;
    max-height: 64px;
  }

  .thumb .drive-icon {
    width: 50%;
    height: 50%;
  }

  .capacity {
    position: absolute;
    right: 12%;
    bottom: 14%;
    left: 12%;
    height: 6px;
    overflow: hidden;
    border-radius: 3px;
    background: var(--surface-raised);
  }

  .capacity span {
    display: block;
    height: 100%;
    background: var(--accent);
  }

  .glyph {
    display: flex;
  }

  .caption {
    display: -webkit-box;
    flex: none;
    width: 100%;
    overflow: hidden;
    font-size: 12px;
    line-height: 16px;
    text-align: center;
    word-break: break-word;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
  }

  .tile .rename {
    flex: none;
    width: 100%;
    margin-top: 4px;
    font-size: 12px;
  }

  .message {
    position: absolute;
    top: 80px;
    right: 0;
    left: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 0 24px;
    color: var(--text-muted);
    text-align: center;
    pointer-events: none;
  }

  .message button {
    pointer-events: auto;
  }
</style>
