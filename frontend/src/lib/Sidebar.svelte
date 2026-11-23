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
    /** Heights of the open sections whose bottom edge was dragged, by ID. A
     *  section without one is as tall as its content. */
    sectionHeights: Record<string, number>
    /** Sets the height of a section, or null for its first height. */
    onsectionheight: (id: string, height: number | null) => void
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
    /** The tab groups with their numbers of tabs. The first is General. */
    groups: { id: string; name: string; tabs: number }[]
    activeGroup: string
    groupsOpen: boolean
    ontogglegroups: () => void
    onshowgroup: (id: string) => void
    onnewgroup: () => void
    onrenamegroup: (id: string) => void
    ondeletegroup: (id: string) => void
    /** A tab is being dragged, which a group takes. */
    tabDragging: boolean
    ondroptab: (groupId: string) => void
    /** Moves a group before the one at an index, or to the end. */
    onmovegroup: (id: string, before: number) => void
    placesOpen: boolean
    ontoggleplaces: () => void
    /** The sections in their order, by ID. */
    order: string[]
    onmovesection: (id: string, before: number) => void
  }

  let {
    places,
    recent,
    current,
    terminalName,
    recentOpen,
    ontogglerecent,
    sectionHeights,
    onsectionheight,
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
    groups,
    activeGroup,
    groupsOpen,
    ontogglegroups,
    onshowgroup,
    onnewgroup,
    onrenamegroup,
    ondeletegroup,
    tabDragging,
    ondroptab,
    onmovegroup,
    placesOpen,
    ontoggleplaces,
    order,
    onmovesection,
  }: Props = $props()

  const GENERAL = 'general'
  let shownGroup = $derived(groups.find((group) => group.id === activeGroup))
  /** The group a dragged tab is over. */
  let dropGroup = $state('')

  // A group takes a tab dragged from the tab bar, and groups dragged among
  // themselves change their order.
  const GROUP_TYPE = 'application/x-sasha-group'
  let draggedGroup = $state('')
  /** Where a dragged group would go: an index into groups, or -1. */
  let groupInsert = $state(-1)

  function groupDragStart(event: DragEvent, id: string) {
    draggedGroup = id
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData(GROUP_TYPE, id)
    }
  }

  function groupDragEnd() {
    draggedGroup = ''
    groupInsert = -1
  }

  function groupDragOver(event: DragEvent, id: string, index: number) {
    if (draggedGroup) {
      event.preventDefault()
      event.stopPropagation()
      const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
      groupInsert = event.clientY < box.top + box.height / 2 ? index : index + 1
      return
    }
    if (!tabDragging || id === activeGroup) return
    event.preventDefault()
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
    dropGroup = id
  }

  function groupDrop(event: DragEvent, id: string) {
    if (draggedGroup) {
      event.preventDefault()
      event.stopPropagation()
      if (groupInsert >= 0) onmovegroup(draggedGroup, groupInsert)
      groupDragEnd()
      return
    }
    if (!tabDragging || id === activeGroup) return
    event.preventDefault()
    event.stopPropagation()
    dropGroup = ''
    ondroptab(id)
  }

  /** Below the last group, a dragged group goes to the end. */
  function groupListDragOver(event: DragEvent) {
    if (!draggedGroup) return
    event.preventDefault()
    event.stopPropagation()
    groupInsert = groups.length
  }

  function groupListDrop(event: DragEvent) {
    if (!draggedGroup) return
    event.preventDefault()
    event.stopPropagation()
    onmovegroup(draggedGroup, groups.length)
    groupDragEnd()
  }

  function groupDragLeave(event: DragEvent) {
    const element = event.currentTarget as HTMLElement
    if (!element.contains(event.relatedTarget as Node | null)) dropGroup = ''
  }

  // ----- Sections -----

  /** The sections of the sidebar. A new one gets an entry here, a body in the
   *  markup, and its ID in the list the program saves. */
  const SECTIONS: Record<string, { title: string }> = {
    groups: { title: 'Groups' },
    places: { title: 'Places' },
    recent: { title: 'Recent' },
  }
  const SECTION_TYPE = 'application/x-sasha-section'
  let draggedSection = $state('')
  /** Where a dragged section would go: an index into order, or -1. */
  let sectionInsert = $state(-1)

  function isOpen(id: string): boolean {
    return id === 'groups' ? groupsOpen : id === 'places' ? placesOpen : id === 'recent' ? recentOpen : true
  }

  function toggle(id: string) {
    if (id === 'groups') ontogglegroups()
    else if (id === 'places') ontoggleplaces()
    else if (id === 'recent') ontogglerecent()
  }

  /** What a folded section says beside its title. */
  function foldedNote(id: string): string {
    if (id === 'groups') return shownGroup?.name ?? ''
    if (id === 'places') return visible.length > 0 ? String(visible.length) : ''
    if (id === 'recent') return recent.length > 0 ? String(recent.length) : ''
    return ''
  }

  function sectionDragStart(event: DragEvent, id: string) {
    // A place, a group, or a recent folder in the section drags itself.
    if (event.target !== event.currentTarget && !(event.target as HTMLElement).closest('.fold')) return
    draggedSection = id
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData(SECTION_TYPE, id)
    }
  }

  function sectionDragEnd() {
    draggedSection = ''
    sectionInsert = -1
  }

  function sectionDragOver(event: DragEvent, index: number) {
    if (!draggedSection) return
    event.preventDefault()
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    sectionInsert = event.clientY < box.top + Math.min(box.height / 2, 40) ? index : index + 1
  }

  function sectionDrop(event: DragEvent) {
    if (!draggedSection) return
    event.preventDefault()
    event.stopPropagation()
    if (sectionInsert >= 0) onmovesection(draggedSection, sectionInsert)
    sectionDragEnd()
  }

  function sectionDragLeave(event: DragEvent) {
    const element = event.currentTarget as HTMLElement
    if (!element.contains(event.relatedTarget as Node | null)) sectionInsert = -1
  }

  function groupMenu(event: MouseEvent, group: { id: string; name: string }) {
    event.preventDefault()
    menu = {
      x: event.clientX,
      y: event.clientY,
      items: [
        { label: 'Show', disabled: group.id === activeGroup, action: () => onshowgroup(group.id) },
        { label: 'Rename…', action: () => onrenamegroup(group.id) },
        { label: 'Delete…', danger: true, disabled: group.id === GENERAL, action: () => ondeletegroup(group.id) },
        'separator',
        { label: 'New group…', action: onnewgroup },
      ],
    }
  }

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

  let aside = $state<HTMLElement>()
  /** The section whose edge is being dragged. */
  let resizing = $state('')

  /** Dragging the bottom edge of an open section changes its height, as the
   *  borders of the views in VS Code do. */
  function startSectionResize(event: PointerEvent, id: string) {
    event.preventDefault()
    event.stopPropagation()
    const handle = event.currentTarget as HTMLElement
    const section = handle.parentElement
    if (!section) return
    handle.setPointerCapture(event.pointerId)
    const startY = event.clientY
    const start = section.getBoundingClientRect().height
    // The other sections keep room for their titles at least.
    const titles = (order.length - 1) * 36
    const most = Math.max(80, (aside?.clientHeight ?? 600) - 200 - titles)
    resizing = id
    const move = (moveEvent: PointerEvent) => {
      onsectionheight(id, Math.round(Math.max(MIN_SECTION, Math.min(most, start + moveEvent.clientY - startY))))
    }
    const up = () => {
      resizing = ''
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
  }

  /** The lowest a section goes: its title and a row. */
  const MIN_SECTION = 64

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

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="sections" class:insert-end={sectionInsert === order.length} ondragleave={sectionDragLeave}>
    {#each order as id, index (id)}
      {@const section = SECTIONS[id]}
      {#if section}
        {@const open = isOpen(id)}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <section
          class="section section-{id}"
          class:open
          class:sized={open && !!sectionHeights[id]}
          class:line-before={sectionInsert === index}
          class:dragging={draggedSection === id}
          style:height={open && sectionHeights[id] ? `${sectionHeights[id]}px` : null}
          ondragover={(event) => sectionDragOver(event, index)}
          ondrop={sectionDrop}
        >
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="section-title"
            draggable="true"
            title="Drag to move this section"
            ondragstart={(event) => sectionDragStart(event, id)}
            ondragend={sectionDragEnd}
          >
            <!-- A tab held over the folded groups unfolds them, to drop it on one. -->
            <button
              type="button"
              class="fold"
              aria-expanded={open}
              title={open ? `Fold ${section.title}` : `Show ${section.title}`}
              onclick={() => toggle(id)}
              ondragenter={() => id === 'groups' && tabDragging && !open && toggle(id)}
            >
              <Icon name={open ? 'chevronDown' : 'chevron'} size={12} />
              <span>{section.title}</span>
              {#if !open}
                {@const folded = foldedNote(id)}
                {#if folded}<span class="count" class:group-name={id === 'groups'}>{folded}</span>{/if}
              {/if}
            </button>
            {#if id === 'groups'}
              <button type="button" class="icon small" aria-label="New group" title="New group" onclick={onnewgroup}>
                <Icon name="plus" size={14} />
              </button>
            {:else if id === 'places'}
              <button type="button" class="icon small" aria-label="Add a folder" title="Add a folder" onclick={onpickplace}>
                <Icon name="plus" size={14} />
              </button>
            {:else if id === 'recent' && open && recent.length > 0}
              <button type="button" class="link" onclick={onclearrecent}>Clear</button>
            {/if}
          </div>
          {#if open}
            {#if id === 'groups'}
              {@render groupList()}
            {:else if id === 'places'}
              {@render placeList()}
            {:else if id === 'recent'}
              {@render recentList()}
            {/if}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
              class="sash"
              class:active={resizing === id}
              title="Drag to resize, double-click for the first size"
              onpointerdown={(event) => startSectionResize(event, id)}
              ondblclick={() => onsectionheight(id, null)}
            ></div>
          {/if}
        </section>
      {/if}
    {/each}
  </div>

  <div class="spacer"></div>

  <footer>
    <button type="button" class="row settings" onclick={onsettings}>
      <Icon name="settings" /> Settings
    </button>
  </footer>
</aside>

{#snippet groupList()}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <ul class="groups" class:insert-end={groupInsert === groups.length} ondragover={groupListDragOver} ondrop={groupListDrop}>
    {#each groups as group, index (group.id)}
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <li
        class:drop-on={dropGroup === group.id}
        class:general={group.id === GENERAL}
        class:line-before={groupInsert === index}
        class:dragging={draggedGroup === group.id}
        draggable="true"
        ondragstart={(event) => groupDragStart(event, group.id)}
        ondragend={groupDragEnd}
        ondragover={(event) => groupDragOver(event, group.id, index)}
        ondragleave={groupDragLeave}
        ondrop={(event) => groupDrop(event, group.id)}
      >
        <button
          type="button"
          class="row"
          class:current={group.id === activeGroup}
          title={group.id === activeGroup
            ? `${group.name}: shown. Drag a tab onto another group to move it there.`
            : `Show ${group.name}. Double-click to rename.`}
          onclick={() => onshowgroup(group.id)}
          ondblclick={() => onrenamegroup(group.id)}
          oncontextmenu={(event) => groupMenu(event, group)}
        >
          <Icon name="tabs" />
          <span class="name">{group.name}</span>
          <span class="detail" title={`${group.tabs} ${group.tabs === 1 ? 'tab' : 'tabs'}`}>{group.tabs}</span>
        </button>
        {#if group.id !== GENERAL}
          <button
            type="button"
            class="icon small remove"
            aria-label={`Delete the group ${group.name}`}
            title="Delete the group"
            onclick={() => ondeletegroup(group.id)}
          >
            <Icon name="close" size={12} />
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{/snippet}

{#snippet placeList()}
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
{/snippet}

{#snippet recentList()}
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
{/snippet}

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

  /* The sections share the room under This PC; a long list scrolls inside its
     section, and the empty room stays at the bottom. */
  .sections {
    display: flex;
    flex: 0 1 auto;
    flex-direction: column;
    min-height: 0;
  }

  .sections.insert-end {
    box-shadow: 0 2px 0 var(--accent);
  }

  .section {
    position: relative;
    display: flex;
    flex: 0 1 auto;
    flex-direction: column;
    min-height: 32px;
    border-top: 1px solid var(--border);
  }

  .section.dragging {
    opacity: 0.5;
  }

  .section.line-before::before {
    position: absolute;
    z-index: 3;
    top: -2px;
    right: 0;
    left: 0;
    height: 2px;
    background: var(--accent);
    content: '';
  }

  .section-title {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: space-between;
    min-height: 28px;
    margin-top: 4px;
    padding: 0 6px 0 0;
    color: var(--text-muted);
    font-size: 12px;
    cursor: grab;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .places,
  .groups,
  .recent {
    flex: 0 1 auto;
    min-height: 0;
    padding-bottom: 6px;
    overflow-y: auto;
  }

  .places.insert-end,
  .groups.insert-end {
    box-shadow: inset 0 -2px 0 var(--accent);
  }

  .group-name {
    overflow: hidden;
    max-width: 120px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* General has no delete button, so its count stays on hover. */
  li.general:hover .detail {
    visibility: visible;
  }

  li.drop-on .row {
    background: var(--accent-soft);
    box-shadow: inset 0 0 0 1px var(--accent);
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

  .spacer {
    flex: 1 1 0;
    min-height: 0;
  }

  /* The bottom edge of the history changes its height. */
  .sash {
    position: absolute;
    z-index: 2;
    right: -8px;
    bottom: -3px;
    left: -8px;
    height: 5px;
    cursor: row-resize;
  }

  .sash:hover,
  .sash.active {
    background: var(--accent-soft);
  }

  /* A section given a height by its edge fills it with its list. */
  .section.sized > ul {
    flex: 1 1 0;
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
