<script lang="ts" module>
  export interface Place {
    path: string
    label: string
    icon: import('./Icon.svelte').IconName
    /** A drive that is not connected now keeps its place but is not shown. */
    present: boolean
    detail?: string
  }
</script>

<script lang="ts">
  import iconImage from '../assets/images/icon.png'
  import Icon from './Icon.svelte'
  import Menu, { type MenuConfig } from './Menu.svelte'
  import { draggedPaths, dropTarget, endDrag, type DropHandler } from './drag'
  import { baseName, formatSize, RECYCLE_BIN, samePath, shortPath, THIS_PC } from './util'

  interface Props {
    places: Place[]
    recent: string[]
    current: string
    /** The terminal that opens a folder, or empty when there is none. */
    terminalName: string
    /** The list of recent folders is shown; folded, only its title is. */
    recentOpen: boolean
    ontogglerecent: () => void
    /** The height of the shown history, which its top edge changes. */
    recentHeight: number
    onrecentheight: (height: number) => void
    folderPaths: (paths: string[]) => string[]
    onopen: (path: string, newTab: boolean) => void
    onaddplaces: (paths: string[], before: number) => void
    onmoveplace: (path: string, before: number) => void
    onremoveplace: (path: string) => void
    onpickplace: () => void
    onremoverecent: (path: string) => void
    onclearrecent: () => void
    onterminal: (path: string) => void
    onproperties: (path: string) => void
    ondropfiles: DropHandler
    onsettings: (event: MouseEvent) => void
    /** What the Recycle Bin holds, or null before it is known. */
    recycleBin: { items: number; bytes: number } | null
    onopenbin: (newTab: boolean) => void
    onbinexplorer: () => void
    onemptybin: () => void
    /** Items dropped on the Recycle Bin go there. */
    ondropbin: DropHandler
  }

  let {
    places,
    recent,
    current,
    terminalName,
    recentOpen,
    ontogglerecent,
    recentHeight,
    onrecentheight,
    folderPaths,
    onopen,
    onaddplaces,
    onmoveplace,
    onremoveplace,
    onpickplace,
    onremoverecent,
    onclearrecent,
    onterminal,
    onproperties,
    ondropfiles,
    onsettings,
    recycleBin,
    onopenbin,
    onbinexplorer,
    onemptybin,
    ondropbin,
  }: Props = $props()

  let binFull = $derived((recycleBin?.items ?? 0) > 0)
  let binTitle = $derived(
    !recycleBin
      ? 'Recycle Bin'
      : binFull
        ? `Recycle Bin: ${recycleBin.items.toLocaleString('en-US')} ${recycleBin.items === 1 ? 'item' : 'items'}, ${formatSize(recycleBin.bytes)}`
        : 'Recycle Bin is empty',
  )

  function binMenu(event: MouseEvent) {
    event.preventDefault()
    menu = {
      x: event.clientX,
      y: event.clientY,
      items: [
        { label: 'Open', action: () => onopenbin(false) },
        { label: 'Open in new tab', action: () => onopenbin(true) },
        { label: 'Open in Explorer', action: onbinexplorer },
        'separator',
        { label: 'Empty Recycle Bin', danger: true, disabled: !binFull, action: onemptybin },
      ],
    }
  }

  const PLACE_TYPE = 'application/x-sasha-place'
  let visible = $derived(places.filter((place) => place.present))
  let draggedPlace = $state('')
  /** Where a place or a folder would go: an index into places, or -1. */
  let insertAt = $state(-1)
  let menu = $state<MenuConfig | null>(null)

  /** A dragged place moves; dragged folders from the list are added. */
  function insertionFor(event: DragEvent, index: number, edgesOnly: boolean): number {
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    const offset = event.clientY - box.top
    const edge = edgesOnly ? box.height * 0.3 : box.height / 2
    const target = places.indexOf(visible[index])
    if (offset < edge) return target
    if (offset > box.height - edge) return target + 1
    return -1
  }

  function placeDragStart(event: DragEvent, place: Place) {
    draggedPlace = place.path
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData(PLACE_TYPE, place.path)
    }
  }

  function placeDragOver(event: DragEvent, index: number) {
    if (draggedPlace) {
      event.preventDefault()
      event.stopPropagation()
      insertAt = insertionFor(event, index, false)
      return
    }
    // Folders from the list: the edges add them here, the middle drops the
    // items into the place, which the drop target handles.
    const folders = folderPaths(draggedPaths())
    if (folders.length === 0) return
    const at = insertionFor(event, index, true)
    if (at < 0) {
      insertAt = -1
      return
    }
    event.preventDefault()
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
    insertAt = at
  }

  /** The middle of a place takes items into it; the edges are for the sidebar. */
  function inMiddle(event: DragEvent): boolean {
    if (folderPaths(draggedPaths()).length === 0) return true
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    const offset = event.clientY - box.top
    return offset >= box.height * 0.3 && offset <= box.height * 0.7
  }

  function placeDrop(event: DragEvent) {
    if (insertAt < 0) return
    event.preventDefault()
    event.stopPropagation()
    if (draggedPlace) onmoveplace(draggedPlace, insertAt)
    else onaddplaces(folderPaths(draggedPaths()), insertAt)
    finish()
  }

  function listDragOver(event: DragEvent) {
    if (draggedPlace || folderPaths(draggedPaths()).length > 0) {
      event.preventDefault()
      if (event.dataTransfer) event.dataTransfer.dropEffect = draggedPlace ? 'move' : 'copy'
      insertAt = places.length
    }
  }

  function listDrop(event: DragEvent) {
    if (insertAt !== places.length) return
    event.preventDefault()
    event.stopPropagation()
    if (draggedPlace) onmoveplace(draggedPlace, insertAt)
    else onaddplaces(folderPaths(draggedPaths()), insertAt)
    finish()
  }

  function listDragLeave(event: DragEvent) {
    const element = event.currentTarget as HTMLElement
    if (!element.contains(event.relatedTarget as Node | null)) insertAt = -1
  }

  function finish() {
    draggedPlace = ''
    insertAt = -1
    endDrag()
  }

  function lineBefore(place: Place): boolean {
    return insertAt >= 0 && places[insertAt] === place
  }

  function placeMenu(event: MouseEvent, place: Place) {
    event.preventDefault()
    menu = {
      x: event.clientX,
      y: event.clientY,
      items: [
        { label: 'Open', action: () => onopen(place.path, false) },
        { label: 'Open in new tab', action: () => onopen(place.path, true) },
        ...(terminalName ? [{ label: `Open in ${terminalName}`, action: () => onterminal(place.path) }] : []),
        'separator',
        { label: 'Remove from sidebar', action: () => onremoveplace(place.path) },
        { label: 'Add a folder…', action: onpickplace },
        'separator',
        { label: 'Properties', action: () => onproperties(place.path) },
      ],
    }
  }

  function recentMenu(event: MouseEvent, path: string) {
    event.preventDefault()
    menu = {
      x: event.clientX,
      y: event.clientY,
      items: [
        { label: 'Open', action: () => onopen(path, false) },
        { label: 'Open in new tab', action: () => onopen(path, true) },
        { label: 'Add to sidebar', action: () => onaddplaces([path], places.length) },
        'separator',
        { label: 'Remove from history', action: () => onremoverecent(path) },
        { label: 'Clear history', action: onclearrecent },
      ],
    }
  }

  const DEFAULT_RECENT_HEIGHT = 220
  let aside = $state<HTMLElement>()

  /** Dragging the top edge of the history changes its height, as in VS Code. */
  function startRecentResize(event: PointerEvent) {
    event.preventDefault()
    const handle = event.currentTarget as HTMLElement
    handle.setPointerCapture(event.pointerId)
    const startY = event.clientY
    const start = recentHeight
    // The places keep some room above it.
    const most = Math.max(80, (aside?.clientHeight ?? 600) - 260)
    const move = (moveEvent: PointerEvent) => {
      onrecentheight(Math.round(Math.max(60, Math.min(most, start + startY - moveEvent.clientY))))
    }
    const up = () => {
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
  }

  function middleClick(event: MouseEvent, path: string) {
    if (event.button === 1) {
      event.preventDefault()
      onopen(path, true)
    }
  }
</script>

<aside bind:this={aside}>
  <div class="brand">
    <img src={iconImage} alt="" />
    <span>SashaExplorer</span>
  </div>

  <button
    type="button"
    class="row pc"
    class:current={current === THIS_PC}
    onclick={() => onopen(THIS_PC, false)}
    onauxclick={(event) => middleClick(event, THIS_PC)}
  >
    <Icon name="pc" /> This PC
  </button>

  <div class="section-title">
    <span>Places</span>
    <button type="button" class="icon small" aria-label="Add a folder" title="Add a folder" onclick={onpickplace}>
      <Icon name="plus" size={14} />
    </button>
  </div>
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <ul
    class="places"
    class:insert-end={insertAt === places.length}
    ondragover={listDragOver}
    ondrop={listDrop}
    ondragleave={listDragLeave}
  >
    {#each visible as place, index (place.path)}
      <li
        class:line-before={lineBefore(place)}
        class:dragging={draggedPlace === place.path}
        draggable="true"
        ondragstart={(event) => placeDragStart(event, place)}
        ondragend={finish}
        ondragover={(event) => placeDragOver(event, index)}
        ondrop={placeDrop}
        use:dropTarget={{ path: place.path, ondrop: ondropfiles, accept: inMiddle }}
      >
        <button
          type="button"
          class="row"
          class:current={samePath(current, place.path)}
          title={place.path}
          onclick={() => onopen(place.path, false)}
          onauxclick={(event) => middleClick(event, place.path)}
          oncontextmenu={(event) => placeMenu(event, place)}
        >
          <Icon name={place.icon} />
          <span class="name">{place.label}</span>
          {#if place.detail}<span class="detail">{place.detail}</span>{/if}
        </button>
        <button
          type="button"
          class="icon small remove"
          aria-label={`Remove ${place.label} from the sidebar`}
          title="Remove from the sidebar"
          onclick={() => onremoveplace(place.path)}
        >
          <Icon name="close" size={12} />
        </button>
      </li>
    {/each}
    {#if visible.length === 0}
      <li class="empty">Drag folders here to keep them at hand.</li>
    {/if}
  </ul>
  <div class="bin" use:dropTarget={{ path: RECYCLE_BIN, ondrop: ondropbin }}>
    <button
      type="button"
      class="row"
      class:full={binFull}
      class:current={current === RECYCLE_BIN}
      title={binTitle}
      onclick={() => onopenbin(false)}
      onauxclick={(event) => {
        if (event.button === 1) {
          event.preventDefault()
          onopenbin(true)
        }
      }}
      oncontextmenu={binMenu}
    >
      <Icon name={binFull ? 'binFull' : 'bin'} />
      <span class="name">Recycle Bin</span>
      {#if binFull && recycleBin}<span class="detail">{formatSize(recycleBin.bytes)}</span>{/if}
    </button>
    {#if binFull}
      <button
        type="button"
        class="icon small remove"
        aria-label="Empty Recycle Bin"
        title="Empty Recycle Bin"
        onclick={onemptybin}
      >
        <Icon name="trash" size={13} />
      </button>
    {/if}
  </div>

  <div class="spacer"></div>
  <section class="recent-section" class:open={recentOpen} style:height={recentOpen ? `${recentHeight}px` : null}>
  {#if recentOpen}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="sash"
      title="Drag to resize, double-click for the first size"
      onpointerdown={startRecentResize}
      ondblclick={() => onrecentheight(DEFAULT_RECENT_HEIGHT)}
    ></div>
  {/if}
  <div class="section-title recent-title">
    <button type="button" class="fold" aria-expanded={recentOpen} title={recentOpen ? 'Fold the history' : 'Show the history'} onclick={ontogglerecent}>
      <Icon name={recentOpen ? 'chevronDown' : 'chevron'} size={12} />
      <span>Recent</span>
      {#if !recentOpen && recent.length > 0}<span class="count">{recent.length}</span>{/if}
    </button>
    {#if recentOpen && recent.length > 0}
      <button type="button" class="link" onclick={onclearrecent}>Clear</button>
    {/if}
  </div>
  {#if recentOpen}
  <ul class="recent">
    {#each recent as path (path)}
      <li use:dropTarget={{ path, ondrop: ondropfiles }}>
        <button
          type="button"
          class="row"
          title={path}
          onclick={() => onopen(path, false)}
          onauxclick={(event) => middleClick(event, path)}
          oncontextmenu={(event) => recentMenu(event, path)}
        >
          <Icon name="clock" size={14} />
          <span class="name">{shortPath(path)}</span>
        </button>
        <button
          type="button"
          class="icon small remove"
          aria-label={`Remove ${baseName(path)} from history`}
          title="Remove from history"
          onclick={() => onremoverecent(path)}
        >
          <Icon name="close" size={12} />
        </button>
      </li>
    {/each}
    {#if recent.length === 0}
      <li class="empty">Folders you open appear here.</li>
    {/if}
  </ul>
  {/if}
  </section>

  <footer>
    <button type="button" class="row settings" onclick={onsettings}>
      <Icon name="settings" /> Settings
    </button>
  </footer>
</aside>

{#if menu}
  <Menu config={menu} onclose={() => (menu = null)} />
{/if}

<style>
  aside {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    padding: 0 8px 8px;
    background: var(--surface);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 6px 10px;
    font-weight: 600;
  }

  .brand img {
    width: 42px;
    height: 42px;
    border-radius: 6px;
    box-shadow: 0 0 12px rgb(229 140 240 / 35%);
  }

  .section-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 28px;
    margin-top: 8px;
    padding: 0 6px;
    color: var(--text-muted);
    font-size: 12px;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .places {
    flex: 0 1 auto;
    min-height: 40px;
    padding-bottom: 6px;
    overflow-y: auto;
  }

  .places.insert-end {
    box-shadow: inset 0 -2px 0 var(--accent);
  }

  .recent-title {
    padding-top: 8px;
    padding-left: 0;
    border-top: 1px solid var(--border);
  }

  .fold {
    display: inline-flex;
    flex: 1;
    align-items: center;
    gap: 4px;
    height: 24px;
    padding: 0 6px 0 2px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: 12px;
    text-align: left;
  }

  .fold:hover {
    color: var(--text);
  }

  .count {
    margin-left: 2px;
    padding: 0 6px;
    border-radius: 8px;
    background: var(--surface-raised);
    font-size: 11px;
  }

  /* The history sits at the bottom, above the settings; the room between
     stays empty, as in VS Code. */
  .spacer {
    flex: 1 1 0;
    min-height: 0;
  }

  .recent-section {
    position: relative;
    display: flex;
    flex: none;
    flex-direction: column;
    min-height: 0;
  }

  .sash {
    position: absolute;
    z-index: 2;
    top: 6px;
    right: -8px;
    left: -8px;
    height: 5px;
    cursor: row-resize;
  }

  .sash:hover {
    background: var(--accent-soft);
  }

  .recent {
    flex: 1 1 0;
    min-height: 0;
    overflow-y: auto;
  }

  li {
    position: relative;
    display: flex;
    align-items: center;
    border-radius: 6px;
  }

  li.dragging {
    opacity: 0.5;
  }

  li.line-before::before {
    position: absolute;
    top: -1px;
    right: 4px;
    left: 4px;
    height: 2px;
    background: var(--accent);
    content: '';
  }

  .row {
    display: flex;
    flex: 1;
    align-items: center;
    gap: 8px;
    min-width: 0;
    height: 30px;
    padding: 0 8px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text);
    text-align: left;
  }

  .row :global(svg) {
    flex: none;
    color: var(--text-muted);
  }

  .row:hover,
  li:hover .row {
    background: var(--surface-raised);
  }

  .row.current {
    background: var(--selected);
  }

  .row.current :global(svg) {
    color: var(--accent);
  }

  .pc {
    flex: none;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .detail {
    color: var(--text-muted);
    font-size: 11px;
  }

  .recent .row {
    height: 28px;
    font-size: 13px;
  }

  .small {
    width: 24px;
    height: 24px;
    border-color: transparent;
    background: transparent;
    color: var(--text-muted);
  }

  .small:hover {
    color: var(--text);
  }

  .remove {
    position: absolute;
    right: 3px;
    display: none;
  }

  li:hover .remove,
  li:focus-within .remove,
  .bin:hover .remove,
  .bin:focus-within .remove {
    display: inline-flex;
  }

  li:hover .detail,
  .bin:hover .detail {
    visibility: hidden;
  }

  .bin {
    position: relative;
    display: flex;
    flex: none;
    align-items: center;
    margin-bottom: 4px;
    border-radius: 6px;
  }

  .bin:hover .row {
    background: var(--surface-raised);
  }

  .bin .full :global(svg) {
    color: var(--accent);
  }

  footer {
    padding-top: 6px;
    border-top: 1px solid var(--border);
  }

  .settings {
    width: 100%;
    color: var(--text-muted);
  }

  .link {
    height: 22px;
    padding: 0 4px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: 12px;
  }

  .link:hover {
    color: var(--text);
  }

  .empty {
    padding: 2px 8px 6px;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
