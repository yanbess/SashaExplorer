<script lang="ts" module>
  export interface TabInfo {
    id: string
    title: string
    path: string
    pinned: boolean
  }

  export type TabAction =
    | 'duplicate'
    | 'pin'
    | 'close'
    | 'closeOthers'
    | 'closeRight'
    | 'closeAll'
    | 'terminal'
    | 'split'
</script>

<script lang="ts">
  import Icon from './Icon.svelte'
  import Menu, { type MenuConfig } from './Menu.svelte'
  import { draggedPaths, dropTarget, type DropHandler } from './drag'
  import { RECYCLE_BIN } from './util'

  interface Props {
    tabs: TabInfo[]
    activeId: string
    /** The terminal that opens a folder, or empty when there is none. */
    terminalName: string
    onselect: (id: string) => void
    onclose: (id: string) => void
    onnew: () => void
    onmove: (id: string, targetId: string, after: boolean) => void
    onaction: (action: TabAction, id: string) => void
    ondropfiles: DropHandler
    /** The window is split, so a tab can move to the other side. */
    split: boolean
    /** A tab of the other pane is being dragged. */
    foreign: boolean
    ondragtab: (id: string) => void
    ondragtabend: () => void
    /** A tab of the other pane was dropped here, before or after targetId. */
    onreceive: (targetId: string | null, after: boolean) => void
    focused: boolean
  }

  let {
    tabs,
    activeId,
    terminalName,
    onselect,
    onclose,
    onnew,
    onmove,
    onaction,
    ondropfiles,
    split,
    foreign,
    ondragtab,
    ondragtabend,
    onreceive,
    focused,
  }: Props = $props()

  /** Holding files over a tab for a moment opens it, to drop into its folder. */
  const HOVER_OPEN_MS = 600
  let hoverTimer: ReturnType<typeof setTimeout> | undefined

  function hoverFiles(id: string) {
    if (draggedPaths().length === 0 || id === activeId) return
    clearTimeout(hoverTimer)
    hoverTimer = setTimeout(() => onselect(id), HOVER_OPEN_MS)
  }

  const TAB_TYPE = 'application/x-sasha-tab'
  let draggedId = $state('')
  let dropId = $state('')
  let dropAfter = $state(false)
  let menu = $state<MenuConfig | null>(null)

  /** Pinned tabs stay in front, so a tab can only move among its own kind. */
  function canDropOn(target: TabInfo) {
    const dragged = tabs.find((tab) => tab.id === draggedId)
    return !!dragged && dragged.id !== target.id && dragged.pinned === target.pinned
  }

  function dragStart(event: DragEvent, id: string) {
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData(TAB_TYPE, id)
    }
    draggedId = id
    ondragtab(id)
  }

  function dragOver(event: DragEvent, target: TabInfo) {
    if (foreign) {
      event.preventDefault()
      event.stopPropagation()
      const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
      dropId = target.id
      dropAfter = event.clientX > box.left + box.width / 2
      return
    }
    if (!draggedId || !canDropOn(target)) return
    event.preventDefault()
    event.stopPropagation()
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    dropId = target.id
    dropAfter = event.clientX > box.left + box.width / 2
  }

  function dragLeave(event: DragEvent) {
    const element = event.currentTarget as HTMLElement
    if (!element.contains(event.relatedTarget as Node | null)) dropId = ''
  }

  function drop(event: DragEvent, target: TabInfo) {
    if (foreign) {
      event.preventDefault()
      event.stopPropagation()
      onreceive(target.id, dropAfter)
      dropId = ''
      return
    }
    if (!draggedId) return
    event.preventDefault()
    event.stopPropagation()
    if (canDropOn(target)) onmove(draggedId, target.id, dropAfter)
    dragEnd()
  }

  function dragEnd() {
    if (draggedId) ondragtabend()
    draggedId = ''
    dropId = ''
  }

  /** The empty end of the bar takes a tab from the other pane too. */
  function barDragOver(event: DragEvent) {
    if (!foreign) return
    event.preventDefault()
    dropId = ''
  }

  function barDrop(event: DragEvent) {
    if (!foreign) return
    event.preventDefault()
    onreceive(null, true)
  }

  function openMenu(event: MouseEvent, tab: TabInfo) {
    event.preventDefault()
    const index = tabs.findIndex((other) => other.id === tab.id)
    const hasOthers = tabs.some((other) => other.id !== tab.id && !other.pinned)
    const hasRight = tabs.slice(index + 1).some((other) => !other.pinned)
    const hasUnpinned = tabs.some((other) => !other.pinned)
    const run = (action: TabAction) => () => onaction(action, tab.id)
    menu = {
      x: event.clientX,
      y: event.clientY,
      items: [
        { label: 'Duplicate tab', action: run('duplicate') },
        { label: tab.pinned ? 'Unpin tab' : 'Pin tab', action: run('pin') },
        ...(terminalName && tab.path && tab.path !== RECYCLE_BIN ? [{ label: `Open in ${terminalName}`, action: run('terminal') }] : []),
        { label: split ? 'Move to other side' : 'Open to the side', hint: split ? '' : 'Ctrl+\\', action: run('split') },
        'separator',
        { label: 'Close', hint: 'Ctrl+W', action: run('close') },
        { label: 'Close others', disabled: !hasOthers, action: run('closeOthers') },
        { label: 'Close to the right', disabled: !hasRight, action: run('closeRight') },
        { label: 'Close all', disabled: !hasUnpinned, action: run('closeAll') },
      ],
    }
  }

  function auxClick(event: MouseEvent, tab: TabInfo) {
    if (event.button === 1 && !tab.pinned) {
      event.preventDefault()
      onclose(tab.id)
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions, a11y_interactive_supports_focus -->
<div class="tabs" class:unfocused={!focused} role="tablist" ondragover={barDragOver} ondrop={barDrop}>
  {#each tabs as tab (tab.id)}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="tab"
      class:active={tab.id === activeId}
      class:pinned={tab.pinned}
      class:dragging={tab.id === draggedId}
      class:drop-before={tab.id === dropId && !dropAfter}
      class:drop-after={tab.id === dropId && dropAfter}
      draggable="true"
      ondragstart={(event) => dragStart(event, tab.id)}
      ondragend={dragEnd}
      ondragover={(event) => dragOver(event, tab)}
      ondragleave={(event) => {
        dragLeave(event)
        if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) clearTimeout(hoverTimer)
      }}
      ondrop={(event) => drop(event, tab)}
      oncontextmenu={(event) => openMenu(event, tab)}
      onauxclick={(event) => auxClick(event, tab)}
      onmousedown={(event) => event.button === 1 && event.preventDefault()}
      ondragenter={() => hoverFiles(tab.id)}
      use:dropTarget={{ path: tab.path, ondrop: ondropfiles }}
    >
      {#if tab.pinned}
        <span class="pin" aria-hidden="true"><Icon name="pin" size={12} /></span>
      {/if}
      <button
        type="button"
        class="tab-open"
        role="tab"
        aria-selected={tab.id === activeId}
        title={tab.path === RECYCLE_BIN ? 'Recycle Bin' : tab.path || 'This PC'}
        onclick={() => onselect(tab.id)}
      >
        {tab.title}
      </button>
      {#if !tab.pinned}
        <button
          type="button"
          class="tab-close"
          aria-label={`Close ${tab.title}`}
          title="Close (Ctrl+W)"
          onclick={() => onclose(tab.id)}
        >
          <Icon name="close" size={12} />
        </button>
      {/if}
    </div>
  {/each}
  <button type="button" class="new-tab" aria-label="New tab" title="New tab (Ctrl+T)" onclick={onnew}>
    <Icon name="plus" size={14} />
  </button>
</div>

{#if menu}
  <Menu config={menu} onclose={() => (menu = null)} />
{/if}

<style>
  .tabs {
    display: flex;
    flex: none;
    align-items: flex-end;
    gap: 2px;
    padding: 6px 6px 0;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    scrollbar-width: none;
  }

  .tab {
    position: relative;
    display: flex;
    flex: 0 1 200px;
    align-items: center;
    gap: 4px;
    min-width: 100px;
    height: 34px;
    padding: 0 4px 0 12px;
    border-radius: 6px 6px 0 0;
    color: var(--text-muted);
  }

  .tab:hover {
    background: var(--surface-raised);
  }

  .tab.active {
    background: var(--bg);
    box-shadow: inset 0 2px 0 var(--accent);
    color: var(--text);
  }

  .unfocused .tab.active {
    box-shadow: inset 0 2px 0 var(--border);
  }

  .tab.pinned {
    flex: 0 0 auto;
    min-width: 0;
    max-width: 160px;
    padding: 0 12px 0 10px;
  }

  .tab.dragging {
    opacity: 0.5;
  }

  .tab.drop-before::before,
  .tab.drop-after::after {
    position: absolute;
    top: 4px;
    bottom: 4px;
    width: 2px;
    background: var(--accent);
    content: '';
  }

  .tab.drop-before::before {
    left: -2px;
  }

  .tab.drop-after::after {
    right: -2px;
  }

  .pin {
    display: inline-flex;
    color: var(--accent);
  }

  .tab-open {
    flex: 1;
    min-width: 0;
    height: 30px;
    padding: 0;
    overflow: hidden;
    border: none;
    background: transparent;
    color: inherit;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tab-close,
  .new-tab {
    display: inline-flex;
    flex: none;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-muted);
  }

  .new-tab {
    width: 30px;
    height: 30px;
    margin: 0 0 2px 2px;
  }

  .tab-close:hover,
  .new-tab:hover {
    background: var(--surface-raised);
    color: var(--text);
  }
</style>
