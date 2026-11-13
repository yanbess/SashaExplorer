<script lang="ts">
  import { onMount, tick } from 'svelte'
  import * as api from '../wailsjs/go/main/App'
  import { files, state as stateModels, terminals, undo, winshell } from '../wailsjs/go/models'
  import { ClipboardSetText, EventsOn, OnFileDrop, OnFileDropOff, WindowSetTitle } from '../wailsjs/runtime/runtime'
  import AddressBar from './lib/AddressBar.svelte'
  import ConfirmDialog, { type ConfirmConfig } from './lib/ConfirmDialog.svelte'
  import FileView, {
    type Columns,
    type Command,
    type DriveInfo,
    type Entry,
    type Selection,
    type SortKey,
    type ViewMode,
  } from './lib/FileView.svelte'
  import Icon, { type IconName } from './lib/Icon.svelte'
  import Menu, { type MenuConfig, type MenuEntry } from './lib/Menu.svelte'
  import PreviewPane from './lib/PreviewPane.svelte'
  import SettingsDialog, { type SettingsSection, type TerminalInfo, type Theme } from './lib/SettingsDialog.svelte'
  import Sidebar, { type Place } from './lib/Sidebar.svelte'
  import TabBar, { type TabAction } from './lib/TabBar.svelte'
  import { canDrop, isNativeDrag, transferMode, type TransferMode } from './lib/drag'
  import {
    baseName,
    compareNames,
    errorText,
    formatSize,
    isDriveRoot,
    parentPath,
    RECYCLE_BIN,
    samePath,
    THIS_PC,
    uniqueName,
  } from './lib/util'

  const SAVE_DELAY_MS = 300
  const DRIVE_POLL_MS = 15000
  const MAX_RECENT = 30
  const TOAST_MS = 7000
  const DEFAULT_SIDEBAR_WIDTH = 230
  const DEFAULT_PREVIEW_WIDTH = 320
  const DEFAULT_SPLIT = 0.5
  const UPDATE_EVERY_MS = 24 * 60 * 60 * 1000

  interface Search {
    query: string
    root: string
    running: boolean
    results: Entry[]
    truncated: boolean
  }

  interface Tab {
    id: string
    path: string
    pinned: boolean
    back: string[]
    forward: string[]
    entries: Entry[]
    types: Record<string, string>
    /** The folder the entries belong to, or null before the first load. */
    shown: string | null
    loading: boolean
    error: string
    selection: Selection
    filter: string
    token: number
    /** Results of a search in subfolders, shown instead of the folder. */
    search: Search | null
    /** Totals of the folders in the list. */
    sizes: Record<string, number>
  }

  interface Pane {
    id: string
    tabs: Tab[]
    activeId: string
  }

  let panes = $state<Pane[]>([])
  let focusedId = $state('')
  let splitRatio = $state(0.5)
  let places = $state<string[]>([])
  let removedDrives = $state<string[]>([])
  let recent = $state<string[]>([])
  let showHidden = $state(false)
  /** Icons drawn in the style of the app; off shows the Windows ones. */
  let themedIcons = $state(true)
  let sidebarWidth = $state(230)
  let previewWidth = $state(320)
  let previewOpen = $state(true)
  let tileSize = $state(160)
  let columns = $state<Columns>({ modified: 150, type: 160, size: 100, folder: 220 })
  /** How each folder is shown, by folderKey(). Folders not in it use DEFAULT_VIEW. */
  let folderViews = $state<Record<string, FolderView>>({})
  let recycleBin = $state<winshell.RecycleBin | null>(null)
  let known = $state<Record<string, string>>({})
  let drives = $state<files.Drive[]>([])
  /** Terminals found on the computer, and those added by hand. */
  let foundTerminals = $state<TerminalInfo[]>([])
  let customTerminals = $state<TerminalInfo[]>([])
  let defaultTerminal = $state('')
  /** The default when none was chosen, which the program picks. */
  let fallbackTerminal = $state('')
  let recentOpen = $state(false)
  let recentHeight = $state(220)
  /** Found terminals the user took out of the list. */
  let hiddenTerminals = $state<string[]>([])
  let theme = $state<Theme>('system')
  let keepWindowSize = $state(true)
  let keepMaximized = $state(true)
  let checkUpdates = $state(true)
  let lastUpdateCheck = $state(0)
  let skippedVersion = $state('')
  let version = $state('dev')
  /** A newer release to tell about, or null. */
  let release = $state<{ version: string; url: string } | null>(null)
  /** The open settings and their first section, or null. */
  let settings = $state<SettingsSection | null>(null)
  let clipboardFiles = $state(false)
  let cutPaths = $state<Set<string>>(new Set())
  let undoState = $state<undo.State>(undo.State.createFrom({}))
  let menu = $state<MenuConfig | null>(null)
  let confirm = $state<ConfirmConfig | null>(null)
  let renaming = $state('')
  let toast = $state('')
  let ready = $state(false)
  /** A tab being dragged, which may go to the other pane or open a split. */
  let draggingTab = $state<{ paneId: string; tabId: string } | null>(null)
  /** Where a dragged tab would land in a pane's list area. */
  let splitZone = $state<{ paneId: string; side: 'left' | 'right' | 'all' } | null>(null)

  let fileViews = $state<Record<string, FileView>>({})
  let addressBars = $state<Record<string, AddressBar>>({})
  let filterInputs = $state<Record<string, HTMLInputElement>>({})
  let panesElement = $state<HTMLElement>()
  let saveTimer: ReturnType<typeof setTimeout> | undefined
  let toastTimer: ReturnType<typeof setTimeout> | undefined

  let focused = $derived(panes.find((pane) => pane.id === focusedId) ?? panes[0])
  let active = $derived(focused ? activeOf(focused) : undefined)
  let driveMap = $derived(
    new Map<string, DriveInfo>(drives.map((drive) => [drive.path, { kind: drive.kind, total: drive.total, free: drive.free }])),
  )
  let driveEntries = $derived(
    drives.map((drive) =>
      files.Entry.createFrom({
        name: driveLabel(drive.path),
        path: drive.path,
        dir: true,
        size: drive.total,
        modified: 0,
        created: 0,
        ext: '',
      }),
    ),
  )
  let visibleByPane = $derived(
    new Map(
      panes.map((pane) => {
        const tab = activeOf(pane)
        return [pane.id, tab ? visibleEntries(tab) : []]
      }),
    ),
  )
  let visible = $derived(focused ? (visibleByPane.get(focused.id) ?? []) : [])
  let selectedEntries = $derived.by(() => {
    if (!active) return []
    const chosen = new Set(active.selection.selected)
    return visible.filter((entry) => chosen.has(entry.path))
  })
  let placeList = $derived(places.map(placeOf))
  let terminalList = $derived([...foundTerminals.filter((item) => !hiddenTerminals.includes(item.id)), ...customTerminals])
  let terminal = $derived(
    terminalList.find((item) => item.id === defaultTerminal) ??
      terminalList.find((item) => item.id === fallbackTerminal) ??
      terminalList[0],
  )
  let terminalName = $derived(terminal?.name ?? '')

  function activeOf(pane: Pane): Tab | undefined {
    return pane.tabs.find((tab) => tab.id === pane.activeId)
  }

  function paneOf(tab: Tab): Pane | undefined {
    return panes.find((pane) => pane.tabs.includes(tab))
  }

  function viewOf(tab: Tab): FileView | undefined {
    const pane = paneOf(tab)
    return pane ? fileViews[pane.id] : undefined
  }

  // ----- Names and places -----

  function driveLabel(path: string): string {
    const drive = drives.find((item) => samePath(item.path, path))
    const letter = path.slice(0, 2).toUpperCase()
    const kinds: Record<string, string> = { removable: 'USB Drive', network: 'Network Drive', cd: 'CD Drive' }
    const label = drive?.label || kinds[drive?.kind ?? ''] || 'Local Disk'
    return `${label} (${letter})`
  }

  const KNOWN_PLACES: Record<string, { label: string; icon: IconName }> = {
    desktop: { label: 'Desktop', icon: 'desktop' },
    downloads: { label: 'Downloads', icon: 'download' },
    documents: { label: 'Documents', icon: 'file' },
    pictures: { label: 'Pictures', icon: 'image' },
    music: { label: 'Music', icon: 'folder' },
    videos: { label: 'Videos', icon: 'folder' },
    home: { label: '', icon: 'home' },
  }

  function placeOf(path: string): Place {
    if (path === RECYCLE_BIN) return { path, label: 'Recycle Bin', icon: 'bin', present: true }
    if (isDriveRoot(path)) {
      const present = drives.some((drive) => samePath(drive.path, path))
      return { path, label: driveLabel(path), icon: 'drive', present }
    }
    for (const [name, info] of Object.entries(KNOWN_PLACES)) {
      if (known[name] && samePath(known[name], path)) {
        return { path, label: info.label || baseName(path), icon: info.icon, present: true }
      }
    }
    return { path, label: baseName(path), icon: 'folder', present: true }
  }

  function titleOf(path: string): string {
    if (path === THIS_PC) return 'This PC'
    if (isDriveRoot(path)) return driveLabel(path)
    return placeOf(path).label
  }

  function tabTitle(tab: Tab): string {
    return tab.search ? `Search: ${tab.search.query}` : titleOf(tab.path)
  }

  function labelFor(path: string, fallback: string): string {
    return isDriveRoot(path) ? driveLabel(path) : fallback
  }

  // ----- Listing -----

  function typeName(tab: Tab, entry: Entry): string {
    if (entry.dir) return ''
    return tab.types[entry.ext] || entry.ext
  }

  function sizeOf(tab: Tab, entry: Entry): number {
    return entry.dir && !isBin(tab) ? (tab.sizes[entry.path] ?? -1) : entry.size
  }

  /** The tab shows the Recycle Bin, whose items are inside it, not in folders. */
  function isBin(tab: Tab): boolean {
    return tab.path === RECYCLE_BIN && !tab.search
  }

  function visibleEntries(tab: Tab): Entry[] {
    let list = tab.search ? tab.search.results : tab.path === THIS_PC ? driveEntries : tab.entries
    if (!showHidden && !tab.search) list = list.filter((entry) => !entry.hidden)
    const filter = tab.search ? '' : tab.filter.trim().toLowerCase()
    if (filter) list = list.filter((entry) => entry.name.toLowerCase().includes(filter))
    const { sortKey, sortDesc } = viewOfTab(tab)
    const direction = sortDesc ? -1 : 1
    const compare = (a: Entry, b: Entry): number => {
      if (a.dir !== b.dir) return a.dir ? -1 : 1
      let result = 0
      if (sortKey === 'modified') result = a.modified - b.modified
      else if (sortKey === 'created') result = a.created - b.created
      else if (sortKey === 'deleted') result = (a.deleted ?? 0) - (b.deleted ?? 0)
      else if (sortKey === 'size') result = sizeOf(tab, a) - sizeOf(tab, b)
      else if (sortKey === 'type') result = compareNames(typeName(tab, a), typeName(tab, b))
      // Drives go by letter, not by label.
      const name = tab.path === THIS_PC ? compareNames(a.path, b.path) : compareNames(a.name, b.name)
      return (result || name) * direction
    }
    return [...list].sort(compare)
  }

  function emptySelection(): Selection {
    return { selected: [], anchor: '', focus: '' }
  }

  function newTab(path: string, saved?: { pinned?: boolean }): Tab {
    return {
      id: crypto.randomUUID(),
      path,
      pinned: saved?.pinned ?? false,
      back: [],
      forward: [],
      entries: [],
      types: {},
      shown: null,
      loading: false,
      error: '',
      selection: emptySelection(),
      filter: '',
      token: 0,
      search: null,
      sizes: {},
    }
  }

  function newPane(tabs: Tab[], activeId = tabs[0]?.id ?? ''): Pane {
    return { id: crypto.randomUUID(), tabs, activeId }
  }

  // ----- Views of folders -----

  interface FolderView {
    view: ViewMode
    sortKey: SortKey
    sortDesc: boolean
    used: number
  }

  const DEFAULT_VIEW: FolderView = { view: 'details', sortKey: 'name', sortDesc: false, used: 0 }

  /** Each folder keeps its view and sort order. Search results share one, and
   *  so does This PC. */
  function folderKey(path: string, search = false): string {
    if (search) return '<search>'
    if (path === THIS_PC) return '<this pc>'
    if (path === RECYCLE_BIN) return '<recycle bin>'
    return (isDriveRoot(path) ? path.slice(0, 2) : path.replace(/[\\/]+$/, '')).toLowerCase()
  }

  function tabKey(tab: Tab): string {
    return folderKey(tab.path, !!tab.search)
  }

  /** The Recycle Bin starts with what was deleted last. */
  const BIN_VIEW: FolderView = { view: 'details', sortKey: 'deleted', sortDesc: true, used: 0 }

  function defaultView(tab: Tab): FolderView {
    return isBin(tab) ? BIN_VIEW : DEFAULT_VIEW
  }

  function viewOfTab(tab: Tab): FolderView {
    return folderViews[tabKey(tab)] ?? defaultView(tab)
  }

  function setFolderView(tab: Tab, change: Partial<FolderView>) {
    const key = tabKey(tab)
    const next = { ...viewOfTab(tab), ...change, used: Date.now() }
    const base = defaultView(tab)
    // The default needs no entry, which keeps the list short.
    if (next.view === base.view && next.sortKey === base.sortKey && next.sortDesc === base.sortDesc) {
      delete folderViews[key]
    } else {
      folderViews[key] = next
    }
  }

  const DEFAULT_TILE_SIZE = 160

  /** Choosing the gallery also undoes a size set with Ctrl+wheel. */
  function setView(tab: Tab, view: ViewMode) {
    if (view === 'gallery') tileSize = DEFAULT_TILE_SIZE
    setFolderView(tab, { view })
  }

  /** Dates and sizes start with the newest and largest, the rest from A. */
  function sortBy(tab: Tab, key: SortKey) {
    const current = viewOfTab(tab)
    if (current.sortKey === key) setFolderView(tab, { sortDesc: !current.sortDesc })
    else setFolderView(tab, { sortKey: key, sortDesc: ['modified', 'created', 'deleted', 'size'].includes(key) })
  }

  type HistoryMove = 'push' | 'back' | 'forward' | 'none'

  /** Shows a folder in a tab. A folder that cannot be read leaves the tab where
   *  it was, unless the tab has nothing to show yet. */
  async function navigate(tab: Tab, path: string, move: HistoryMove = 'push', select = ''): Promise<boolean> {
    stopSearch(tab)
    const token = ++tab.token
    const previous = tab.path
    tab.loading = true
    try {
      let folderPath = THIS_PC
      let entries: Entry[] = []
      let types: Record<string, string> = {}
      if (path === THIS_PC) {
        drives = await api.Drives()
      } else if (path === RECYCLE_BIN) {
        const folder = await api.ListRecycleBin()
        folderPath = RECYCLE_BIN
        entries = folder.entries ?? []
        types = folder.types ?? {}
      } else {
        const folder = await api.ListDir(path)
        folderPath = folder.path
        entries = folder.entries ?? []
        types = folder.types ?? {}
      }
      if (token !== tab.token) return false
      const changed = tab.shown === null || !samePath(previous, folderPath)
      if (changed && tab.shown !== null) {
        if (move === 'push') {
          tab.back.push(previous)
          tab.forward = []
        } else if (move === 'back') {
          tab.forward.push(previous)
          tab.back.pop()
        } else if (move === 'forward') {
          tab.back.push(previous)
          tab.forward.pop()
        }
      }
      tab.path = folderPath
      tab.shown = folderPath
      tab.entries = entries
      tab.types = types
      tab.error = ''
      if (changed) {
        tab.filter = ''
        tab.sizes = {}
      }
      tab.selection = select ? { selected: [select], anchor: select, focus: select } : emptySelection()
      if (folderPath !== THIS_PC && folderPath !== RECYCLE_BIN) addRecent(folderPath)
      requestSizes(tab, false)
      if (select) viewOf(tab)?.reveal(select)
      return true
    } catch (e) {
      if (token !== tab.token) return false
      if (tab.shown === null) {
        tab.path = path
        tab.shown = path
        tab.error = errorText(e)
      } else {
        showToast(errorText(e))
      }
      return false
    } finally {
      if (token === tab.token) tab.loading = false
    }
  }

  /** Reads the folder again and keeps the selection of items still there. */
  async function refresh(tab: Tab, fresh = false) {
    if (tab.search) {
      if (fresh) startSearch(tab, tab.search.query, tab.search.root)
      return
    }
    if (tab.path === THIS_PC) {
      drives = await api.Drives()
      return
    }
    const token = ++tab.token
    try {
      const folder = isBin(tab) ? await api.ListRecycleBin() : await api.ListDir(tab.path)
      if (token !== tab.token) return
      tab.entries = folder.entries ?? []
      tab.types = folder.types ?? {}
      tab.shown = isBin(tab) ? RECYCLE_BIN : folder.path
      tab.error = ''
      const existing = new Set(tab.entries.map((entry) => entry.path))
      const kept = tab.selection.selected.filter((path) => existing.has(path))
      if (kept.length !== tab.selection.selected.length) {
        tab.selection = { ...tab.selection, selected: kept }
      }
      requestSizes(tab, fresh)
    } catch (e) {
      if (token !== tab.token) return
      tab.entries = []
      tab.error = errorText(e)
    }
  }

  function refreshVisible() {
    for (const pane of panes) {
      const tab = activeOf(pane)
      if (tab) refresh(tab)
    }
    refreshRecycleBin()
  }

  // ----- Folder sizes -----

  function requestSizes(tab: Tab, fresh: boolean) {
    if (tab.search || tab.path === THIS_PC || isBin(tab)) return
    const folders = tab.entries.filter((entry) => entry.dir && (showHidden || !entry.hidden)).map((entry) => entry.path)
    if (folders.length === 0) return
    api.FolderSizes(tab.id, folders, fresh)
  }

  function findTab(id: string): Tab | undefined {
    for (const pane of panes) {
      const tab = pane.tabs.find((item) => item.id === id)
      if (tab) return tab
    }
  }

  // ----- Search -----

  function startSearch(tab: Tab, query: string, root = tab.path) {
    query = query.trim()
    if (!query || root === THIS_PC || root === RECYCLE_BIN) return
    tab.search = { query, root, running: true, results: [], truncated: false }
    tab.selection = emptySelection()
    api
      .Search(tab.id, root, query, showHidden)
      .then((done) => {
        if (tab.search?.query === query && tab.search.root === root && !done.cancelled) {
          tab.search.running = false
          tab.search.truncated = done.truncated
        }
      })
      .catch((e) => {
        if (tab.search) tab.search.running = false
        showToast(errorText(e))
      })
  }

  function stopSearch(tab: Tab) {
    if (!tab.search) return
    if (tab.search.running) api.CancelSearch(tab.id)
    tab.search = null
    tab.filter = ''
    tab.selection = emptySelection()
  }

  function addRecent(path: string) {
    recent = [path, ...recent.filter((item) => !samePath(item, path))].slice(0, MAX_RECENT)
  }

  function goBack(tab: Tab) {
    if (tab.search) return stopSearch(tab)
    const target = tab.back.at(-1)
    if (target !== undefined) navigate(tab, target, 'back')
  }

  function goForward(tab: Tab) {
    const target = tab.forward.at(-1)
    if (target !== undefined) navigate(tab, target, 'forward')
  }

  function goUp(tab: Tab) {
    if (tab.search) return stopSearch(tab)
    if (tab.path === THIS_PC) return
    navigate(tab, parentPath(tab.path), 'push', tab.path)
  }

  async function submitAddress(tab: Tab, text: string) {
    if (text.toLowerCase() === 'this pc') {
      navigate(tab, THIS_PC)
      return
    }
    if (['recycle bin', 'shell:recyclebinfolder'].includes(text.toLowerCase())) {
      navigate(tab, RECYCLE_BIN)
      return
    }
    try {
      const location = await api.Resolve(text)
      await navigate(tab, location.folder, 'push', location.select)
    } catch (e) {
      showToast(errorText(e))
    }
  }

  // ----- Tabs and panes -----

  function insertIndex(pane: Pane): number {
    const index = pane.tabs.findIndex((tab) => tab.id === pane.activeId)
    const pinned = pane.tabs.filter((tab) => tab.pinned).length
    return Math.max(index + 1, pinned)
  }

  /** The last tab that shows a folder, in either pane. */
  function findOpenTab(path: string): { pane: Pane; tab: Tab } | undefined {
    let found: { pane: Pane; tab: Tab } | undefined
    for (const pane of panes) {
      for (const tab of pane.tabs) {
        if (!tab.search && samePath(tab.path, path)) found = { pane, tab }
      }
    }
    return found
  }

  /** Shows a folder in a tab: the one already open on it, or a new one. */
  function openTab(path: string, activate = true, select = '', pane = focused, at?: number): Tab | undefined {
    if (!pane) return
    // A new tab of This PC is what Ctrl+T and the plus button ask for.
    const open = path === THIS_PC ? undefined : findOpenTab(path)
    if (open) {
      if (select) {
        open.tab.selection = { selected: [select], anchor: select, focus: select }
        // A file made after the folder was read needs it read again.
        if (!open.tab.entries.some((entry) => samePath(entry.path, select))) navigate(open.tab, open.tab.path, 'none', select)
      }
      if (activate) activateTab(open.pane, open.tab.id)
      if (select) tick().then(() => fileViews[open.pane.id]?.reveal(select))
      return open.tab
    }
    const index = at ?? insertIndex(pane)
    pane.tabs.splice(index, 0, newTab(path))
    const tab = pane.tabs[index]
    navigate(tab, path, 'none', select)
    if (activate) activateTab(pane, tab.id)
    return tab
  }

  function activateTab(pane: Pane, id: string) {
    pane.activeId = id
    focusedId = pane.id
    renaming = ''
    const tab = pane.tabs.find((item) => item.id === id)
    if (!tab) return
    if (tab.shown === null) navigate(tab, tab.path, 'none')
    else refresh(tab)
    focusList(pane)
  }

  function closeTabs(pane: Pane, ids: string[]) {
    for (const id of ids) {
      const index = pane.tabs.findIndex((tab) => tab.id === id)
      if (index < 0) continue
      const [tab] = pane.tabs.splice(index, 1)
      api.CancelFolderSizes(tab.id)
      if (tab.search?.running) api.CancelSearch(tab.id)
      if (pane.activeId === id) pane.activeId = pane.tabs[Math.min(index, pane.tabs.length - 1)]?.id ?? ''
    }
    if (pane.tabs.length === 0) {
      if (panes.length > 1) {
        removePane(pane)
        return
      }
      pane.tabs.push(newTab(THIS_PC))
      pane.activeId = pane.tabs[0].id
    }
    activateTab(pane, pane.activeId)
  }

  function removePane(pane: Pane) {
    panes = panes.filter((other) => other.id !== pane.id)
    api.Watch(1, '')
    const rest = panes[0]
    if (rest) activateTab(rest, rest.activeId)
  }

  function pinnedCount(pane: Pane) {
    return pane.tabs.filter((tab) => tab.pinned).length
  }

  function duplicate(tab: Tab): Tab {
    const copy = newTab(tab.path, tab)
    navigate(copy, tab.path, 'none')
    return copy
  }

  function handleTabAction(pane: Pane, action: TabAction, id: string) {
    const index = pane.tabs.findIndex((tab) => tab.id === id)
    const tab = pane.tabs[index]
    if (!tab) return
    const unpinned = (list: Tab[]) => list.filter((item) => !item.pinned).map((item) => item.id)
    switch (action) {
      case 'duplicate': {
        const copy = duplicate(tab)
        pane.tabs.splice(tab.pinned ? pinnedCount(pane) : index + 1, 0, copy)
        activateTab(pane, copy.id)
        return
      }
      case 'pin': {
        const [moved] = pane.tabs.splice(index, 1)
        moved.pinned = !moved.pinned
        pane.tabs.splice(pinnedCount(pane), 0, moved)
        return
      }
      case 'split':
        return panes.length > 1 ? moveTabToPane(pane, id, otherPane(pane)!, null, true) : splitWith(pane, id, 'right')
      case 'terminal':
        return openTerminal(tab.path)
      case 'close':
        return closeTabs(pane, [id])
      case 'closeOthers':
        return closeTabs(pane, unpinned(pane.tabs).filter((other) => other !== id))
      case 'closeRight':
        return closeTabs(pane, unpinned(pane.tabs.slice(index + 1)))
      case 'closeAll':
        return closeTabs(pane, unpinned(pane.tabs))
    }
  }

  function moveTab(pane: Pane, id: string, targetId: string, after: boolean) {
    const from = pane.tabs.findIndex((tab) => tab.id === id)
    if (from < 0) return
    const [tab] = pane.tabs.splice(from, 1)
    const target = pane.tabs.findIndex((candidate) => candidate.id === targetId)
    pane.tabs.splice(target + (after ? 1 : 0), 0, tab)
  }

  function otherPane(pane: Pane): Pane | undefined {
    return panes.find((other) => other.id !== pane.id)
  }

  /** Moves a tab to the other pane. The last tab of a pane closes the split. */
  function moveTabToPane(from: Pane, tabId: string, to: Pane, targetId: string | null, after: boolean) {
    const index = from.tabs.findIndex((tab) => tab.id === tabId)
    if (index < 0 || from === to) return
    const [tab] = from.tabs.splice(index, 1)
    tab.pinned = false
    let at = to.tabs.length
    if (targetId) {
      const target = to.tabs.findIndex((other) => other.id === targetId)
      at = Math.max(pinnedCount(to), target + (after ? 1 : 0))
    }
    to.tabs.splice(at, 0, tab)
    if (from.activeId === tabId) from.activeId = from.tabs[Math.min(index, from.tabs.length - 1)]?.id ?? ''
    if (from.tabs.length === 0) panes = panes.filter((pane) => pane.id !== from.id)
    activateTab(to, tab.id)
  }

  /** Opens a tab in a new pane beside the others. A pane's only tab is copied,
   *  so that side keeps something to show. */
  function splitWith(from: Pane, tabId: string, side: 'left' | 'right') {
    if (panes.length > 1) return
    const index = from.tabs.findIndex((tab) => tab.id === tabId)
    if (index < 0) return
    let tab: Tab
    if (from.tabs.length > 1) {
      ;[tab] = from.tabs.splice(index, 1)
      tab.pinned = false
      if (from.activeId === tabId) from.activeId = from.tabs[Math.min(index, from.tabs.length - 1)].id
    } else {
      tab = duplicate(from.tabs[index])
    }
    const pane = newPane([tab])
    panes = side === 'right' ? [...panes, pane] : [pane, ...panes]
    activateTab(pane, tab.id)
  }

  /** The split button opens the active tab beside, or puts the panes back
   *  together. */
  function toggleSplit() {
    if (panes.length > 1) {
      const [first, second] = panes
      first.tabs.push(...second.tabs.map((tab) => ({ ...tab, pinned: false })))
      panes = [first]
      api.Watch(1, '')
      activateTab(first, first.activeId)
      return
    }
    if (focused && active) splitWith(focused, active.id, 'right')
  }

  function stepTab(direction: number) {
    const pane = focused
    if (!pane || pane.tabs.length < 2) return
    const index = pane.tabs.findIndex((tab) => tab.id === pane.activeId)
    activateTab(pane, pane.tabs[(index + direction + pane.tabs.length) % pane.tabs.length].id)
  }

  /** A tab dragged over a list area shows where it would open. */
  function tabZoneOver(event: DragEvent, pane: Pane) {
    if (!draggingTab) return
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    let side: 'left' | 'right' | 'all' | null = null
    if (draggingTab.paneId !== pane.id) side = 'all'
    else if (panes.length === 1) {
      const x = (event.clientX - box.left) / box.width
      side = x > 0.5 ? 'right' : x < 0.2 ? 'left' : null
    }
    if (!side) {
      splitZone = null
      return
    }
    event.preventDefault()
    event.stopPropagation()
    splitZone = { paneId: pane.id, side }
  }

  function tabZoneDrop(event: DragEvent, pane: Pane) {
    const dragged = draggingTab
    const zone = splitZone
    splitZone = null
    if (!dragged || !zone || zone.paneId !== pane.id) return
    event.preventDefault()
    event.stopPropagation()
    const from = panes.find((item) => item.id === dragged.paneId)
    if (!from) return
    if (zone.side === 'all') moveTabToPane(from, dragged.tabId, pane, null, true)
    else splitWith(from, dragged.tabId, zone.side)
    draggingTab = null
  }

  /** The preview panel sits in the last pane, under its path bar. Its width is
   *  set aside first, so the split ratio is about the lists alone. */
  function paneFlex(index: number): string {
    const last = index === panes.length - 1
    const basis = last && previewOpen ? previewWidth + 5 : 0
    return `${index === 0 ? splitRatio : 1 - splitRatio} 1 ${basis}px`
  }

  /** The preview belongs to the focused pane, whichever pane holds it. */
  function inPreview(event: Event): boolean {
    return !!(event.target as Element | null)?.closest?.('.preview')
  }

  function startSplitResize(event: PointerEvent) {
    event.preventDefault()
    const handle = event.currentTarget as HTMLElement
    handle.setPointerCapture(event.pointerId)
    const box = panesElement!.getBoundingClientRect()
    const lists = box.width - (previewOpen ? previewWidth + 5 : 0)
    const move = (moveEvent: PointerEvent) => {
      splitRatio = Math.max(0.15, Math.min(0.85, (moveEvent.clientX - box.left) / lists))
    }
    const up = () => {
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
  }

  // ----- Sidebar -----

  function addPlaces(paths: string[], before: number) {
    const list = [...places]
    let at = Math.max(0, Math.min(before, list.length))
    for (const path of paths) {
      const existing = list.findIndex((item) => samePath(item, path))
      if (existing >= 0) {
        list.splice(existing, 1)
        if (existing < at) at--
      }
      list.splice(at++, 0, path)
      removedDrives = removedDrives.filter((drive) => !samePath(drive, path))
    }
    places = list
  }

  function movePlace(path: string, before: number) {
    const from = places.findIndex((item) => samePath(item, path))
    if (from < 0) return
    const list = [...places]
    const [moved] = list.splice(from, 1)
    list.splice(from < before ? before - 1 : before, 0, moved)
    places = list
  }

  async function confirmRemovePlace(path: string) {
    const label = placeOf(path).label
    const what = isDriveRoot(path) ? 'The drive' : 'The folder'
    if (await ask(`Remove "${label}" from Places?`, `${what} itself stays where it is. You can add it again later.`, 'Remove')) {
      removePlace(path)
    }
  }

  async function confirmClearRecent() {
    if (await ask('Clear the history?', 'The list of recent folders will be emptied.', 'Clear')) recent = []
  }

  function removePlace(path: string) {
    places = places.filter((item) => !samePath(item, path))
    if (isDriveRoot(path) && !removedDrives.some((drive) => samePath(drive, path))) {
      removedDrives = [...removedDrives, path]
    }
  }

  async function pickPlace() {
    try {
      const folder = await api.PickFolder()
      if (folder) addPlaces([folder], places.length)
    } catch (e) {
      showToast(errorText(e))
    }
  }

  /** New drives join the sidebar on their own, unless they were removed from it. */
  function mergeDrives() {
    const missing = drives
      .map((drive) => drive.path)
      .filter(
        (path) =>
          !places.some((place) => samePath(place, path)) && !removedDrives.some((drive) => samePath(drive, path)),
      )
    if (missing.length > 0) places = [...places, ...missing]
  }

  async function reloadDrives() {
    try {
      drives = await api.Drives()
      mergeDrives()
    } catch {
      // Drives come back on the next poll.
    }
  }

  // ----- Recycle Bin -----

  async function refreshRecycleBin() {
    try {
      const before = recycleBin
      recycleBin = await api.RecycleBin()
      // Something was deleted or restored elsewhere: the open Recycle Bin follows.
      if (before && (before.items !== recycleBin.items || before.bytes !== recycleBin.bytes)) {
        for (const pane of panes) {
          const tab = activeOf(pane)
          if (tab && isBin(tab)) refresh(tab)
        }
      }
    } catch {
      // Shown as unknown until the next check.
    }
  }

  async function emptyRecycleBin() {
    try {
      await api.EmptyRecycleBin()
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  function openBinInExplorer() {
    api.OpenRecycleBin().catch((e) => showToast(errorText(e)))
  }

  async function restoreItems(list: Entry[]) {
    if (list.length === 0) return
    try {
      await api.RestoreFromBin(list.map((entry) => entry.path))
      if (list.length <= 3) showToast(`Restored to ${list.map((entry) => entry.original).join(', ')}`, false)
      else showToast(`${list.length} items were restored where they were.`, false)
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  async function purgeItems(list: Entry[]) {
    if (list.length === 0) return
    const what = list.length === 1 ? `"${list[0].name}"` : `${list.length} items`
    if (!(await ask('Delete permanently?', `${what} will be removed from the Recycle Bin. This cannot be undone.`, 'Delete', true))) return
    try {
      await api.PurgeFromBin(list.map((entry) => entry.path))
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  /** Items dropped on the Recycle Bin are deleted there, which undo brings back. */
  async function dropOnBin(paths: string[]) {
    try {
      await api.Delete(paths, false)
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  /** The folders among dragged items, from any pane. */
  function folderPaths(paths: string[]): string[] {
    if (paths.length === 0) return []
    const dirs = new Set<string>()
    for (const list of visibleByPane.values()) for (const entry of list) if (entry.dir) dirs.add(entry.path)
    return paths.filter((path) => dirs.has(path))
  }

  function openPlace(path: string, inNewTab: boolean) {
    if (inNewTab || !active) openTab(path, false)
    else {
      navigate(active, path)
      if (focused) focusList(focused)
    }
  }

  // ----- Commands -----

  function targets(entry?: Entry): Entry[] {
    if (!active) return []
    if (entry && !active.selection.selected.includes(entry.path)) return [entry]
    if (selectedEntries.length > 0) return selectedEntries
    const focus = visible.find((item) => item.path === active!.selection.focus)
    return focus ? [focus] : []
  }

  async function openEntries(list: Entry[]) {
    if (!active || list.length === 0) return
    const tab = active
    const folders = list.filter((entry) => entry.dir)
    const others = list.filter((entry) => !entry.dir)
    if (folders.length === 1 && others.length === 0) {
      await navigate(tab, folders[0].path)
      return
    }
    for (const folder of folders) openTab(folder.path, false)
    if (others.length > 10 && !(await ask(`Open ${others.length} files?`, 'Each one opens in its program.', 'Open'))) return
    for (const file of others) {
      try {
        const folder = await api.OpenItem(file.path)
        if (folder) await navigate(tab, folder)
      } catch (e) {
        showToast(errorText(e))
      }
    }
  }

  /** Resolves a question asked with ask() when its dialog closes. A promise
   *  ignores the second answer, so closing after "yes" keeps the yes. */
  let answer: ((value: boolean) => void) | null = null

  function ask(title: string, message: string, confirmLabel: string, danger = false): Promise<boolean> {
    return new Promise((resolve) => {
      answer = resolve
      confirm = { title, message, confirmLabel, danger, onconfirm: () => resolve(true) }
    })
  }

  async function runCommand(command: Command, entry?: Entry) {
    const tab = active
    if (!tab) return
    const list = targets(entry)
    const paths = list.map((item) => item.path)
    const inFolder = tab.path !== THIS_PC && !isBin(tab)
    if (isBin(tab)) {
      // The Recycle Bin restores and deletes for good; the rest does not apply.
      if (command === 'restore') return restoreItems(list)
      if (command === 'delete' || command === 'deleteForever') return purgeItems(list)
      if (command !== 'open' && command !== 'openNewTab' && command !== 'up') return
    }
    try {
      switch (command) {
        case 'open':
          return await openEntries(list)
        case 'openNewTab':
          for (const item of list.filter((candidate) => candidate.dir)) openTab(item.path, false)
          return
        case 'up':
          return goUp(tab)
        case 'rename':
          if (list.length === 1 && inFolder) renaming = list[0].path
          return
        case 'delete':
          if (paths.length && inFolder) await api.Delete(paths, false)
          return
        case 'deleteForever': {
          if (!paths.length || !inFolder) return
          const what = paths.length === 1 ? `"${list[0].name}"` : `${paths.length} items`
          if (await ask('Delete permanently?', `${what} will be deleted without the Recycle Bin. This cannot be undone.`, 'Delete', true)) {
            await api.Delete(paths, true)
          }
          return
        }
        case 'copy':
        case 'cut':
          if (!paths.length || !inFolder) return
          await api.SetClipboard(paths, command === 'cut')
          await refreshClipboard()
          return
        case 'paste': {
          const dest = entry?.dir && list.length === 1 ? entry.path : tab.search ? '' : tab.path
          if (!dest || dest === THIS_PC) return
          await api.Paste(dest)
          await refreshClipboard()
          return
        }
        case 'copyPath':
          await ClipboardSetText((paths.length ? paths : [tab.path]).join('\r\n'))
          return
        case 'properties':
          await api.Properties(paths.length ? paths : [tab.path])
          return
      }
    } catch (e) {
      showToast(errorText(e))
    } finally {
      if (['delete', 'deleteForever', 'paste'].includes(command)) refreshVisible()
    }
  }

  async function transfer(paths: string[], dest: string, mode: TransferMode) {
    if (dest === RECYCLE_BIN) return dropOnBin(paths)
    try {
      await api.Transfer(paths, dest, mode)
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  async function undoRedo(redo: boolean) {
    try {
      const label = redo ? await api.Redo() : await api.Undo()
      void label
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  async function createItem(kind: 'folder' | 'text') {
    const tab = active
    if (!tab || tab.path === THIS_PC || tab.search) return
    const names = tab.entries.map((entry) => entry.name)
    try {
      const path =
        kind === 'folder'
          ? await api.CreateFolder(tab.path, uniqueName('New folder', '', names))
          : await api.CreateFile(tab.path, uniqueName('New Text Document', '.txt', names))
      await refresh(tab)
      tab.filter = ''
      tab.selection = { selected: [path], anchor: path, focus: path }
      renaming = path
      viewOf(tab)?.reveal(path)
    } catch (e) {
      showToast(errorText(e))
    }
  }

  function createNamedFile() {
    const tab = active
    if (!tab || tab.path === THIS_PC || tab.search) return
    confirm = {
      title: 'New file',
      inputLabel: 'Name with extension',
      inputValue: uniqueName('New file', '.txt', tab.entries.map((entry) => entry.name)),
      selectBase: true,
      confirmLabel: 'Create',
      onconfirm: async (name) => {
        const path = await api.CreateFile(tab.path, name)
        await refresh(tab)
        tab.selection = { selected: [path], anchor: path, focus: path }
        viewOf(tab)?.reveal(path)
      },
    }
  }

  async function finishRename(path: string, name: string | null) {
    renaming = ''
    const tab = active
    if (!tab || name === null) return
    const trimmed = name.trim()
    if (!trimmed || trimmed === baseName(path)) return
    try {
      const renamed = await api.Rename(path, trimmed)
      if (tab.search) {
        const index = tab.search.results.findIndex((item) => item.path === path)
        if (index >= 0) tab.search.results[index] = await api.Stat(renamed)
      } else {
        await refresh(tab)
      }
      tab.selection = { selected: [renamed], anchor: renamed, focus: renamed }
      viewOf(tab)?.reveal(renamed)
    } catch (e) {
      showToast(errorText(e))
    }
  }

  async function openTerminal(path: string, chosen = terminal) {
    if (!path || !chosen) return
    try {
      await api.OpenTerminal(terminals.Terminal.createFrom(chosen), path)
    } catch (e) {
      showToast(errorText(e))
    }
  }

  /** The arrow of the terminal button lists every terminal, as in VS Code. */
  function terminalMenu(event: MouseEvent, path: string) {
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    const items: MenuEntry[] = terminalList.map((item) => ({
      label: item.name,
      checked: item.id === terminal?.id,
      action: () => openTerminal(path, item),
    }))
    items.push(
      'separator',
      ...(terminalList.length > 1 ? [{ label: 'Choose the default…', action: () => (settings = 'terminal') }] : []),
      { label: 'Add a terminal…', action: () => (settings = 'terminal') },
    )
    menu = { x: Math.max(8, box.right - 220), y: box.bottom + 4, items }
  }

  async function nativeMenu(paths: string[], extended: boolean) {
    try {
      await api.NativeMenu(paths, extended)
    } catch (e) {
      showToast(errorText(e))
    }
    refreshVisible()
  }

  async function refreshClipboard() {
    try {
      const value = await api.GetClipboard()
      clipboardFiles = value.hasFiles
      cutPaths = new Set(value.cut ?? [])
    } catch {
      clipboardFiles = false
    }
  }

  // ----- Menus -----

  function viewItems(tab: Tab): MenuEntry[] {
    return [
      { label: 'Details', checked: viewOfTab(tab).view === 'details', action: () => setView(tab, 'details') },
      { label: 'Medium icons', checked: viewOfTab(tab).view === 'icons', action: () => setView(tab, 'icons') },
      { label: 'Gallery', checked: viewOfTab(tab).view === 'gallery', action: () => setView(tab, 'gallery') },
    ]
  }

  function sortItems(tab: Tab): MenuEntry[] {
    const keys: [SortKey, string][] = [
      ['name', 'Name'],
      ['modified', 'Date modified'],
      ...(isBin(tab) ? [['deleted', 'Date deleted'] as [SortKey, string]] : [['created', 'Date created'] as [SortKey, string]]),
      ['type', 'Type'],
      ['size', 'Size'],
    ]
    const current = viewOfTab(tab)
    return [
      ...keys.map(([key, label]) => ({
        label: `Sort by ${label.toLowerCase()}`,
        checked: current.sortKey === key,
        action: () => current.sortKey !== key && sortBy(tab, key),
      })),
      'separator',
      { label: 'Ascending', checked: !current.sortDesc, action: () => setFolderView(tab, { sortDesc: false }) },
      { label: 'Descending', checked: current.sortDesc, action: () => setFolderView(tab, { sortDesc: true }) },
    ]
  }

  function newItems(): MenuEntry[] {
    return [
      { label: 'Folder', hint: 'Ctrl+Shift+N', action: () => createItem('folder') },
      { label: 'Text document', action: () => createItem('text') },
      { label: 'File…', action: createNamedFile },
    ]
  }

  async function showListMenu(event: MouseEvent, entry: Entry | null) {
    const tab = active
    if (!tab) return
    const x = event.clientX
    const y = event.clientY
    const extended = event.shiftKey
    await refreshClipboard()
    const inFolder = tab.path !== THIS_PC
    if (isBin(tab)) {
      menu = { x, y, items: binMenuItems(tab, entry) }
      return
    }
    if (!entry) {
      const items: MenuEntry[] = [...viewItems(tab), 'separator', ...sortItems(tab)]
      if (inFolder && !tab.search) {
        items.push(
          'separator',
          ...newItems().map((item) => (item === 'separator' ? item : { ...item, label: `New ${item.label.toLowerCase()}` })),
          { label: 'Paste', hint: 'Ctrl+V', disabled: !clipboardFiles, action: () => runCommand('paste') },
          { label: undoState.canUndo ? `Undo ${undoState.undoLabel}` : 'Undo', hint: 'Ctrl+Z', disabled: !undoState.canUndo, action: () => undoRedo(false) },
          'separator',
          { label: 'Refresh', hint: 'F5', action: () => refresh(tab, true) },
          { label: 'Show hidden files', checked: showHidden, action: () => (showHidden = !showHidden) },
          'separator',
          { label: 'Copy path', action: () => runCommand('copyPath') },
          { label: 'Add to sidebar', action: () => addPlaces([tab.path], places.length) },
          ...(terminal ? [{ label: `Open in ${terminalName}`, action: () => openTerminal(tab.path) }] : []),
          'separator',
          { label: 'Properties', action: () => runCommand('properties') },
          { label: 'More options…', action: () => nativeMenu([tab.path], extended) },
        )
      } else {
        items.push('separator', { label: 'Refresh', hint: 'F5', action: () => refresh(tab, true) })
      }
      menu = { x, y, items }
      return
    }

    const list = targets(entry)
    const paths = list.map((item) => item.path)
    const single = list.length === 1 ? list[0] : null
    const folders = list.filter((item) => item.dir)
    const sameFolder = paths.every((path) => samePath(parentPath(path), parentPath(paths[0])))
    const items: MenuEntry[] = [{ label: 'Open', hint: 'Enter', action: () => runCommand('open', entry) }]
    if (folders.length > 0) {
      items.push({ label: 'Open in new tab', hint: 'Ctrl+Enter', action: () => runCommand('openNewTab', entry) })
    }
    if (single && tab.search) {
      items.push({ label: 'Open file location', action: () => navigate(tab, parentPath(single.path), 'push', single.path) })
    }
    if (single && !single.dir) {
      items.push({ label: 'Open with…', action: () => api.OpenWith(single.path).catch((e) => showToast(errorText(e))) })
    }
    if (inFolder) {
      items.push(
        'separator',
        { label: 'Cut', hint: 'Ctrl+X', action: () => runCommand('cut', entry) },
        { label: 'Copy', hint: 'Ctrl+C', action: () => runCommand('copy', entry) },
      )
      if (single?.dir) {
        items.push({ label: 'Paste into folder', disabled: !clipboardFiles, action: () => runCommand('paste', entry) })
      }
    }
    items.push({ label: 'Copy path', hint: 'Ctrl+Shift+C', action: () => runCommand('copyPath', entry) })
    if (inFolder) {
      items.push(
        'separator',
        { label: 'Rename', hint: 'F2', disabled: !single, action: () => runCommand('rename', entry) },
        { label: 'Delete', hint: 'Del', danger: true, action: () => runCommand('delete', entry) },
        { label: 'Delete permanently', hint: 'Shift+Del', danger: true, action: () => runCommand('deleteForever', entry) },
      )
    }
    if (folders.length > 0) {
      items.push('separator', {
        label: folders.length > 1 ? 'Add folders to sidebar' : 'Add to sidebar',
        action: () => addPlaces(folders.map((item) => item.path), places.length),
      })
      if (single?.dir && terminal) {
        items.push({ label: `Open in ${terminalName}`, action: () => openTerminal(single.path) })
      }
    }
    items.push(
      'separator',
      { label: 'Properties', hint: 'Alt+Enter', disabled: !sameFolder, action: () => runCommand('properties', entry) },
      { label: 'More options…', disabled: !sameFolder, action: () => nativeMenu(paths, extended) },
    )
    menu = { x, y, items }
  }

  function binMenuItems(tab: Tab, entry: Entry | null): MenuEntry[] {
    const all = visibleEntries(tab)
    if (!entry) {
      return [
        ...viewItems(tab),
        'separator',
        ...sortItems(tab),
        'separator',
        { label: 'Restore all items', disabled: all.length === 0, action: () => restoreItems(all) },
        { label: 'Empty Recycle Bin', danger: true, disabled: all.length === 0, action: emptyRecycleBin },
        'separator',
        { label: 'Refresh', hint: 'F5', action: () => refresh(tab, true) },
        { label: 'Open in Explorer', action: openBinInExplorer },
      ]
    }
    const list = targets(entry)
    const single = list.length === 1 ? list[0] : null
    return [
      { label: 'Restore', action: () => restoreItems(list) },
      ...(single ? [{ label: 'Open', hint: 'Enter', action: () => runCommand('open', entry) }] : []),
      'separator',
      { label: 'Delete permanently', hint: 'Del', danger: true, action: () => purgeItems(list) },
      'separator',
      {
        label: 'Copy original path',
        action: () => ClipboardSetText(list.map((item) => item.original ?? '').join('\r\n')).catch(() => {}),
      },
    ]
  }

  function showButtonMenu(event: MouseEvent, items: MenuEntry[]) {
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
    menu = { x: box.left, y: box.bottom + 4, items }
  }

  // ----- Updates -----

  /** Asks GitHub for a newer release. Quiet, it shows nothing for a skipped one. */
  async function checkForUpdate(quiet: boolean): Promise<string> {
    lastUpdateCheck = Date.now()
    const latest = await api.CheckUpdate()
    if (!latest.newer) {
      return version === 'dev'
        ? `The latest release is ${latest.version}. This development build is not compared with it.`
        : `SashaExplorer ${version} is the latest version.`
    }
    if (!quiet || latest.version !== skippedVersion) release = { version: latest.version, url: latest.url }
    return `Version ${latest.version} is available.`
  }

  function skipRelease() {
    if (release) skippedVersion = release.version
    release = null
  }

  // ----- Drops from other programs -----

  function dropPathAt(x: number, y: number): { dest: string; sidebar: boolean } {
    let element = document.elementFromPoint(x, y) as HTMLElement | null
    const sidebar = !!element?.closest('.places')
    while (element) {
      const zone = element.closest<HTMLElement>('[data-drop-path]')
      if (!zone) break
      if (zone.dataset.dropPath) return { dest: zone.dataset.dropPath, sidebar }
      element = zone.parentElement
    }
    return { dest: '', sidebar }
  }

  async function externalDrop(x: number, y: number, paths: string[]) {
    // Items dragged from this window were handled by the page already.
    if (!paths?.length || isNativeDrag()) return
    const { dest, sidebar } = dropPathAt(x, y)
    if (sidebar && !dest) {
      const folders: string[] = []
      for (const path of paths) {
        try {
          const location = await api.Resolve(path)
          if (!location.select) folders.push(location.folder)
        } catch {
          // Not a folder that can be added.
        }
      }
      if (folders.length) addPlaces(folders, places.length)
      return
    }
    if (!dest) return
    if (dest === RECYCLE_BIN) {
      dropOnBin(paths)
      return
    }
    const keys = await api.GetModifiers()
    const mode = transferMode({ ctrlKey: keys.ctrl, shiftKey: keys.shift }, paths, dest)
    if (canDrop(paths, dest, mode)) transfer(paths, dest, mode)
  }

  // ----- Keyboard, mouse, layout -----

  function focusList(pane: Pane) {
    tick().then(() => fileViews[pane.id]?.focusList())
  }

  function focusFilter(pane: Pane, text = '') {
    const tab = activeOf(pane)
    if (!tab) return
    if (text && !tab.search) tab.filter += text
    filterInputs[pane.id]?.focus()
  }

  function filterKey(event: KeyboardEvent, pane: Pane, tab: Tab) {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      if (tab.search) stopSearch(tab)
      tab.filter = ''
      focusList(pane)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      if (tab.filter.trim()) startSearch(tab, tab.filter, tab.search?.root ?? tab.path)
      focusList(pane)
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      const first = visibleByPane.get(pane.id)?.[0]
      if (first) tab.selection = { selected: [first.path], anchor: first.path, focus: first.path }
      focusList(pane)
    }
  }

  function inTextField(): boolean {
    const element = document.activeElement
    return element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement
  }

  function handleKeydown(event: KeyboardEvent) {
    const tab = active
    const pane = focused
    if (!tab || !pane || confirm) return
    const ctrl = event.ctrlKey && !event.altKey
    const code = event.code
    let handled = true
    if (ctrl && !event.shiftKey && code === 'KeyT') openTab(THIS_PC)
    else if (ctrl && !event.shiftKey && code === 'KeyW') {
      if (!tab.pinned) closeTabs(pane, [tab.id])
    } else if (ctrl && code === 'Tab') stepTab(event.shiftKey ? -1 : 1)
    else if (ctrl && /^Digit[1-9]$/.test(code)) {
      const n = Number(code.slice(5))
      const target = n === 9 ? pane.tabs.at(-1) : pane.tabs[n - 1]
      if (target) activateTab(pane, target.id)
    } else if ((ctrl && code === 'KeyL') || (event.altKey && code === 'KeyD')) addressBars[pane.id]?.edit()
    else if (ctrl && code === 'KeyF') focusFilter(pane)
    else if (ctrl && code === 'Backslash') toggleSplit()
    else if (ctrl && !inTextField() && ((code === 'KeyZ' && !event.shiftKey) || code === 'KeyY' || (code === 'KeyZ' && event.shiftKey))) {
      undoRedo(code !== 'KeyZ' || event.shiftKey)
    } else if (event.key === 'F5' || (ctrl && code === 'KeyR')) refresh(tab, true)
    else if (event.altKey && event.key === 'ArrowLeft') goBack(tab)
    else if (event.altKey && event.key === 'ArrowRight') goForward(tab)
    else if (event.altKey && event.key === 'ArrowUp') goUp(tab)
    else if (ctrl && event.shiftKey && code === 'KeyN') createItem('folder')
    else handled = false
    if (handled) {
      event.preventDefault()
      event.stopPropagation()
    }
  }

  /** The side buttons of a mouse go back and forward. */
  function mouseUp(event: MouseEvent) {
    if (!active) return
    if (event.button === 3) {
      event.preventDefault()
      goBack(active)
    } else if (event.button === 4) {
      event.preventDefault()
      goForward(active)
    }
  }

  /** A double click on a splitter puts its side back to the first size. */
  function resetSize(side: 'sidebar' | 'preview' | 'split') {
    if (side === 'sidebar') sidebarWidth = DEFAULT_SIDEBAR_WIDTH
    else if (side === 'preview') previewWidth = DEFAULT_PREVIEW_WIDTH
    else splitRatio = DEFAULT_SPLIT
  }

  function startResize(event: PointerEvent, side: 'sidebar' | 'preview') {
    event.preventDefault()
    const handle = event.currentTarget as HTMLElement
    handle.setPointerCapture(event.pointerId)
    const startX = event.clientX
    const start = side === 'sidebar' ? sidebarWidth : previewWidth
    const move = (moveEvent: PointerEvent) => {
      const delta = moveEvent.clientX - startX
      if (side === 'sidebar') sidebarWidth = Math.round(Math.max(160, Math.min(480, start + delta)))
      else previewWidth = Math.round(Math.max(200, Math.min(900, start - delta)))
    }
    const up = () => {
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
  }

  let toastError = $state(true)

  function showToast(message: string, error = true) {
    toast = message
    toastError = error
    clearTimeout(toastTimer)
    toastTimer = setTimeout(() => (toast = ''), TOAST_MS)
  }

  // ----- State -----

  function savedTab(tab: Tab) {
    return { path: tab.path, pinned: tab.pinned }
  }

  function snapshot() {
    return {
      panes: panes.map((pane) => ({
        tabs: pane.tabs.map(savedTab),
        activeIndex: pane.tabs.findIndex((tab) => tab.id === pane.activeId),
      })),
      focusedPane: Math.max(0, panes.findIndex((pane) => pane.id === focusedId)),
      splitRatio,
      windowsIcons: !themedIcons,
      places: [...places],
      removedDrives: [...removedDrives],
      placesReady: true,
      recent: [...recent],
      showHidden,
      sidebarWidth,
      previewWidth,
      previewOpen,
      tileSize,
      columns: { ...columns },
      folderViews: { ...folderViews },
      recentOpen,
      recentHeight,
      hiddenTerminals: [...hiddenTerminals],
      theme,
      keepWindowSize,
      keepMaximized,
      terminals: customTerminals.map(({ id, name, path, args }) => ({ id, name, path, args })),
      defaultTerminal,
      checkUpdates,
      lastUpdateCheck,
      skippedVersion,
    }
  }

  $effect(() => {
    const value = snapshot()
    if (!ready) return
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
      api.SaveState(stateModels.State.createFrom(value)).catch((e) => showToast(errorText(e)))
    }, SAVE_DELAY_MS)
  })

  // The page follows the theme, and so does the title bar of the window.
  $effect(() => {
    document.documentElement.dataset.theme = theme
    if (ready) api.SetTheme(theme)
  })

  $effect(() => {
    WindowSetTitle(active ? `${tabTitle(active)} — SashaExplorer` : 'SashaExplorer')
  })

  $effect(() => {
    panes.forEach((pane, index) => {
      const tab = activeOf(pane)
      api.Watch(index, tab && !tab.search && !isBin(tab) ? tab.path : '')
    })
  })

  async function openPaths(paths: string[]) {
    for (const path of paths) {
      try {
        const location = await api.Resolve(path)
        openTab(location.folder, true, location.select, focused, focused?.tabs.length)
      } catch (e) {
        showToast(errorText(e))
      }
    }
  }

  onMount(() => {
    const stops = [
      EventsOn('dir:changed', (path: string) => {
        for (const pane of panes) {
          const tab = activeOf(pane)
          if (tab && !tab.search && samePath(tab.path, path)) refresh(tab)
        }
      }),
      EventsOn('open:paths', (paths: string[]) => openPaths(paths)),
      EventsOn('search:found', (found: { job: string; entries: Entry[] }) => {
        const tab = findTab(found.job)
        if (tab?.search?.running) tab.search.results.push(...found.entries)
      }),
      EventsOn('sizes:found', (found: { job: string; path: string; bytes: number }) => {
        const tab = findTab(found.job)
        if (tab) tab.sizes[found.path] = found.bytes
      }),
      EventsOn('undo:changed', (state: undo.State) => (undoState = state)),
    ]
    OnFileDrop((x, y, paths) => externalDrop(x, y, paths), false)
    const poll = setInterval(() => {
      reloadDrives()
      refreshRecycleBin()
    }, DRIVE_POLL_MS)
    const onFocus = () => {
      refreshClipboard()
      reloadDrives()
      refreshRecycleBin()
    }
    window.addEventListener('focus', onFocus)
    const endTabDrag = () => {
      draggingTab = null
      splitZone = null
    }
    window.addEventListener('dragend', endTabDrag)

    api
      .GetStartup()
      .then(async (startup) => {
        const saved = startup.state
        known = startup.known ?? {}
        drives = startup.drives ?? []
        version = startup.version
        foundTerminals = (startup.terminals ?? []).map((item) => ({ ...item, custom: false }))
        fallbackTerminal = startup.defaultTerminal
        customTerminals = (saved.terminals ?? []).map((item) => ({ ...item, custom: true }))
        defaultTerminal = saved.defaultTerminal ?? ''
        recentOpen = saved.recentOpen
        recentHeight = saved.recentHeight || 220
        hiddenTerminals = saved.hiddenTerminals ?? []
        theme = (saved.theme || 'system') as Theme
        keepWindowSize = saved.keepWindowSize
        keepMaximized = saved.keepMaximized
        checkUpdates = saved.checkUpdates
        lastUpdateCheck = saved.lastUpdateCheck ?? 0
        skippedVersion = saved.skippedVersion ?? ''
        places = saved.places ?? []
        removedDrives = saved.removedDrives ?? []
        recent = saved.recent ?? []
        showHidden = saved.showHidden
        themedIcons = !saved.windowsIcons
        sidebarWidth = saved.sidebarWidth
        previewWidth = saved.previewWidth
        previewOpen = saved.previewOpen
        tileSize = saved.tileSize
        splitRatio = saved.splitRatio || 0.5
        columns = { ...columns, ...(saved.columns ?? {}) }
        folderViews = (saved.folderViews ?? {}) as Record<string, FolderView>
        mergeDrives()
        // Earlier versions kept the view in each tab; it now goes to its folder.
        for (const pane of saved.panes ?? []) {
          for (const item of pane.tabs ?? []) {
            const key = folderKey(item.path)
            if (item.view && !folderViews[key] && (item.view !== 'details' || item.sortKey !== 'name' || item.sortDesc)) {
              folderViews[key] = {
                view: item.view as ViewMode,
                sortKey: (item.sortKey || 'name') as SortKey,
                sortDesc: item.sortDesc ?? false,
                used: Date.now(),
              }
            }
          }
        }

        panes = (saved.panes ?? []).map((pane) => {
          const tabs = (pane.tabs ?? []).map((item) => newTab(item.path, item))
          return newPane(tabs, tabs[pane.activeIndex]?.id ?? tabs[0]?.id)
        })
        if (panes.length === 0) panes = [newPane([newTab(THIS_PC)])]
        // Each side loads its tab; the focused one comes last, so it keeps the focus.
        const focusIndex = Math.min(saved.focusedPane, panes.length - 1)
        panes.forEach((pane, index) => index !== focusIndex && activateTab(pane, pane.activeId))
        activateTab(panes[focusIndex], panes[focusIndex].activeId)
        undoState = await api.UndoState()
        ready = true
        if (startup.paths?.length) await openPaths(startup.paths)
        refreshClipboard()
        refreshRecycleBin()
        if (checkUpdates && version !== 'dev' && Date.now() - lastUpdateCheck > UPDATE_EVERY_MS) {
          checkForUpdate(true).catch(() => {
            // No network or no answer: the next start asks again.
          })
        }
      })
      .catch((e) => showToast(errorText(e)))

    return () => {
      for (const stop of stops) stop()
      OnFileDropOff()
      clearInterval(poll)
      window.removeEventListener('focus', onFocus)
      window.removeEventListener('dragend', endTabDrag)
    }
  })
</script>

<svelte:window onkeydowncapture={handleKeydown} onmouseup={mouseUp} />

{#snippet paneView(pane: Pane, tab: Tab, last: boolean)}
  {@const list = visibleByPane.get(pane.id) ?? []}
  {@const isFocused = pane.id === focused?.id}
  <TabBar
    tabs={pane.tabs.map((item) => ({ id: item.id, title: tabTitle(item), path: item.search ? '' : item.path, pinned: item.pinned }))}
    activeId={pane.activeId}
    {terminalName}
    split={panes.length > 1}
    foreign={!!draggingTab && draggingTab.paneId !== pane.id}
    focused={isFocused && panes.length > 1}
    onselect={(id) => activateTab(pane, id)}
    onclose={(id) => closeTabs(pane, [id])}
    onnew={() => openTab(THIS_PC, true, '', pane, pane.tabs.length)}
    onmove={(id, targetId, after) => moveTab(pane, id, targetId, after)}
    onaction={(action, id) => handleTabAction(pane, action, id)}
    ondropfiles={transfer}
    ondragtab={(id) => (draggingTab = { paneId: pane.id, tabId: id })}
    ondragtabend={() => {
      draggingTab = null
      splitZone = null
    }}
    onreceive={(targetId, after) => {
      const dragged = draggingTab
      draggingTab = null
      const from = panes.find((item) => item.id === dragged?.paneId)
      if (dragged && from) moveTabToPane(from, dragged.tabId, pane, targetId, after)
    }}
  />

  <div class="toolbar">
    <div class="group">
      <button type="button" class="icon flat" title="Back (Alt+Left)" aria-label="Back" disabled={tab.back.length === 0 && !tab.search} onclick={() => goBack(tab)}>
        <Icon name="back" />
      </button>
      <button type="button" class="icon flat" title="Forward (Alt+Right)" aria-label="Forward" disabled={tab.forward.length === 0} onclick={() => goForward(tab)}>
        <Icon name="forward" />
      </button>
      <button type="button" class="icon flat" title="Up (Alt+Up)" aria-label="Up" disabled={tab.path === THIS_PC && !tab.search} onclick={() => goUp(tab)}>
        <Icon name="up" />
      </button>
      <button type="button" class="icon flat" title="Refresh (F5)" aria-label="Refresh" onclick={() => refresh(tab, true)}>
        <Icon name="reload" />
      </button>
    </div>

    <AddressBar
      bind:this={addressBars[pane.id]}
      path={tab.search ? tab.search.root : tab.path}
      {labelFor}
      onnavigate={(path) => navigate(tab, path)}
      onsubmit={(text) => submitAddress(tab, text)}
      ondropfiles={transfer}
    />

    <label class="filter" class:active-filter={tab.filter || tab.search} title="Type to filter this folder. Enter searches subfolders too.">
      <Icon name="search" size={14} />
      {#if tab.search}
        <span class="search-chip">in subfolders</span>
      {/if}
      <input
        bind:this={filterInputs[pane.id]}
        value={tab.search ? tab.search.query : tab.filter}
        oninput={(event) => {
          const value = (event.currentTarget as HTMLInputElement).value
          if (tab.search) {
            tab.search.query = value
            tab.filter = value
          } else tab.filter = value
        }}
        placeholder={tab.path === THIS_PC ? 'Filter' : 'Filter, Enter to search'}
        aria-label="Filter this folder, or press Enter to search subfolders"
        spellcheck="false"
        onkeydown={(event) => filterKey(event, pane, tab)}
      />
      {#if tab.filter || tab.search}
        <button
          type="button"
          class="clear"
          aria-label="Clear"
          onclick={() => {
            stopSearch(tab)
            tab.filter = ''
            focusList(pane)
          }}
        >
          <Icon name="close" size={12} />
        </button>
      {/if}
    </label>
  </div>

  <div class="commands">
    <div class="segmented" role="radiogroup" aria-label="View">
      <button type="button" role="radio" aria-checked={viewOfTab(tab).view === 'details'} class:on={viewOfTab(tab).view === 'details'} title="Details" aria-label="Details" onclick={() => setView(tab, 'details')}>
        <Icon name="list" />
      </button>
      <button type="button" role="radio" aria-checked={viewOfTab(tab).view === 'icons'} class:on={viewOfTab(tab).view === 'icons'} title="Medium icons" aria-label="Medium icons" onclick={() => setView(tab, 'icons')}>
        <Icon name="icons" size={14} />
      </button>
      <button type="button" role="radio" aria-checked={viewOfTab(tab).view === 'gallery'} class:on={viewOfTab(tab).view === 'gallery'} title="Gallery (Ctrl+wheel changes the size)" aria-label="Gallery" onclick={() => setView(tab, 'gallery')}>
        <Icon name="image" />
      </button>
    </div>
    <button type="button" class="flat labeled" title="Sort" onclick={(event) => showButtonMenu(event, sortItems(tab))}>
      <Icon name="sort" /><span>Sort</span>
    </button>
    {#if isBin(tab)}
      {@const chosen = isFocused ? selectedEntries : []}
      <button type="button" class="flat labeled" title="Put the selected items back where they were" disabled={chosen.length === 0} onclick={() => restoreItems(chosen)}>
        <Icon name="undo" /><span>Restore</span>
      </button>
      <button type="button" class="flat labeled" title="Restore every item" disabled={list.length === 0} onclick={() => restoreItems(list)}>
        <span>Restore all</span>
      </button>
      <button type="button" class="flat labeled danger-button" title="Delete everything in the Recycle Bin for good" disabled={list.length === 0} onclick={emptyRecycleBin}>
        <Icon name="trash" /><span>Empty</span>
      </button>
    {:else}
      <button type="button" class="flat labeled" title="Create a folder or a file" disabled={tab.path === THIS_PC || !!tab.search} onclick={(event) => showButtonMenu(event, newItems())}>
        <Icon name="plus" /><span>New</span>
      </button>
    {/if}
    {#if !tab.search && tab.path !== THIS_PC && !isBin(tab)}
      {@const inSidebar = places.some((place) => samePath(place, tab.path))}
      <button
        type="button"
        class="icon flat"
        class:on={inSidebar}
        title={inSidebar ? 'Remove this folder from the sidebar' : 'Add this folder to the sidebar'}
        aria-label={inSidebar ? 'Remove from the sidebar' : 'Add to the sidebar'}
        aria-pressed={inSidebar}
        onclick={() => (inSidebar ? confirmRemovePlace(tab.path) : addPlaces([tab.path], places.length))}
      >
        <Icon name="bookmark" />
      </button>
    {/if}
    <span class="divider"></span>
    <button type="button" class="icon flat" title={undoState.canUndo ? `Undo ${undoState.undoLabel} (Ctrl+Z)` : 'Nothing to undo'} aria-label="Undo" disabled={!undoState.canUndo} onclick={() => undoRedo(false)}>
      <Icon name="undo" />
    </button>
    <button type="button" class="icon flat" title={undoState.canRedo ? `Redo ${undoState.redoLabel} (Ctrl+Y)` : 'Nothing to redo'} aria-label="Redo" disabled={!undoState.canRedo} onclick={() => undoRedo(true)}>
      <Icon name="redo" />
    </button>
    <span class="spacer"></span>
    <button type="button" class="icon flat" class:on={panes.length > 1} title={panes.length > 1 ? 'Join the panes (Ctrl+\\)' : 'Split (Ctrl+\\). You can also drag a tab to the side.'} aria-label="Split" aria-pressed={panes.length > 1} onclick={toggleSplit}>
      <Icon name="split" />
    </button>
    {#if terminal}
      {@const folder = tab.search?.root ?? tab.path}
      {@const noFolder = tab.path === THIS_PC || isBin(tab)}
      <span class="split-button">
        <button type="button" class="icon flat lucy" title={`Open this folder in ${terminalName}`} aria-label={`Open in ${terminalName}`} disabled={noFolder} onclick={() => openTerminal(folder)}>
          <Icon name="terminal" />
        </button>
        <button type="button" class="flat more" title="Open in another terminal" aria-label="Choose a terminal" disabled={noFolder} onclick={(event) => terminalMenu(event, folder)}>
          <Icon name="chevronDown" size={12} />
        </button>
      </span>
    {/if}
    <button type="button" class="icon flat" class:on={previewOpen} title="Preview panel" aria-label="Preview panel" aria-pressed={previewOpen} onclick={() => (previewOpen = !previewOpen)}>
      <Icon name="panel" />
    </button>
  </div>

  <div class="content">
  <div class="column">
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="list"
    ondragover={(event) => tabZoneOver(event, pane)}
    ondragleave={(event) => {
      if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) splitZone = null
    }}
    ondrop={(event) => tabZoneDrop(event, pane)}
  >
    <FileView
      bind:this={fileViews[pane.id]}
      path={tab.search ? '' : tab.path}
      entries={list}
      types={tab.types}
      view={viewOfTab(tab).view}
      sortKey={viewOfTab(tab).sortKey}
      sortDesc={viewOfTab(tab).sortDesc}
      selection={tab.selection}
      renaming={isFocused ? renaming : ''}
      cut={cutPaths}
      {tileSize}
      {columns}
      drives={tab.path === THIS_PC && !tab.search ? driveMap : undefined}
      folderSizes={tab.path !== THIS_PC ? tab.sizes : null}
      searchRoot={tab.search?.root ?? ''}
      themed={themedIcons}
      bin={isBin(tab)}
      loading={tab.loading}
      error={tab.error}
      emptyText={tab.search
        ? tab.search.running
          ? 'Searching…'
          : `Nothing found for "${tab.search.query}".`
        : tab.filter
          ? `Nothing here matches "${tab.filter}". Press Enter in the filter to search subfolders.`
          : tab.path === THIS_PC
            ? 'No drives found.'
            : isBin(tab)
              ? 'The Recycle Bin is empty.'
              : 'This folder is empty.'}
      onselection={(selection) => (tab.selection = selection)}
      oncommand={runCommand}
      onmenu={showListMenu}
      onrename={finishRename}
      onsort={(key) => sortBy(tab, key)}
      oncolumns={(value) => (columns = value)}
      ontilesize={(size) => (tileSize = size)}
      onfilterkey={(key) => focusFilter(pane, key)}
      onretry={() => navigate(tab, tab.path, 'none')}
      ondropfiles={transfer}
    />
    {#if tab.loading || tab.search?.running}
      <div class="loading" aria-hidden="true"></div>
    {/if}
    {#if splitZone?.paneId === pane.id}
      <div class="split-zone {splitZone.side}">
        <span>{splitZone.side === 'all' ? `Move to this side` : 'Open to the side'}</span>
      </div>
    {/if}
  </div>

  <footer>
    {#if tab.search}
      <span class="accent">
        {tab.search.running ? 'Searching…' : 'Found'}
        {tab.search.results.length.toLocaleString('en-US')}{tab.search.truncated ? '+ (stopped at the limit)' : ''}
      </span>
      {#if tab.search.running}
        <button type="button" class="link" onclick={() => tab.search && (api.CancelSearch(tab.id), (tab.search.running = false))}>Stop</button>
      {/if}
    {:else}
      <span>{list.length} {list.length === 1 ? 'item' : 'items'}</span>
    {/if}
    {#if isFocused && selectedEntries.length > 0}
      <span>
        {selectedEntries.length} selected{#if selectedEntries.some((entry) => !entry.dir) && tab.path !== THIS_PC}
          , {formatSize(selectedEntries.reduce((sum, entry) => sum + (entry.dir ? 0 : entry.size), 0))}{/if}
      </span>
    {/if}
    {#if tab.filter && !tab.search}
      <span class="accent">Filtered by "{tab.filter}"</span>
    {/if}
    {#if !showHidden && !tab.search && tab.entries.some((entry) => entry.hidden)}
      <span>{tab.entries.filter((entry) => entry.hidden).length} hidden</span>
    {/if}
  </footer>
  </div>
  {#if last && previewOpen && active}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="splitter" title="Drag to resize, double-click for the first size" onpointerdown={(event) => startResize(event, 'preview')} ondblclick={() => resetSize('preview')}></div>
    <div class="preview" style:width={`${previewWidth}px`}>
      <PreviewPane
        folder={active.search ? active.search.root : active.path}
        folderLabel={titleOf(active.search ? active.search.root : active.path)}
        selected={selectedEntries}
        count={visible.length}
        types={active.types}
        sizes={active.sizes}
        {drives}
        {driveLabel}
        themed={themedIcons}
        bin={isBin(active) ? recycleBin : null}
      />
    </div>
  {/if}
  </div>
{/snippet}

<div class="layout" style:grid-template-columns={`${sidebarWidth}px minmax(0, 1fr)`}>
  <div class="side">
    <Sidebar
      places={placeList}
      {recent}
      current={active && !active.search ? active.path : ''}
      {terminalName}
      {recentOpen}
      ontogglerecent={() => (recentOpen = !recentOpen)}
      {recentHeight}
      onrecentheight={(height) => (recentHeight = height)}
      {folderPaths}
      onopen={openPlace}
      onaddplaces={addPlaces}
      onmoveplace={movePlace}
      onremoveplace={confirmRemovePlace}
      onpickplace={pickPlace}
      onremoverecent={(path) => (recent = recent.filter((item) => item !== path))}
      onclearrecent={confirmClearRecent}
      onterminal={(path) => openTerminal(path)}
      onproperties={(path) => api.Properties([path]).catch((e) => showToast(errorText(e)))}
      ondropfiles={transfer}
      onsettings={() => (settings = 'appearance')}
      {recycleBin}
      onopenbin={(newTab) => openPlace(RECYCLE_BIN, newTab)}
      onbinexplorer={openBinInExplorer}
      onemptybin={emptyRecycleBin}
      ondropbin={(paths) => dropOnBin(paths)}
    />
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="splitter side-splitter" title="Drag to resize, double-click for the first size" onpointerdown={(event) => startResize(event, 'sidebar')} ondblclick={() => resetSize('sidebar')}></div>
  </div>

  <div class="work">
    <div class="panes" bind:this={panesElement}>
      {#each panes as pane, index (pane.id)}
        {@const tab = activeOf(pane)}
        {#if index > 0}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="splitter pane-splitter" title="Drag to resize, double-click for halves" onpointerdown={startSplitResize} ondblclick={() => resetSize('split')}></div>
        {/if}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <main
          class:focused-pane={panes.length > 1 && pane.id === focused?.id}
          style:flex={panes.length > 1 ? paneFlex(index) : '1 1 0'}
          onmousedowncapture={(event) => !inPreview(event) && (focusedId = pane.id)}
          onfocusin={(event) => !inPreview(event) && (focusedId = pane.id)}
        >
          {#if tab}
            {@render paneView(pane, tab, index === panes.length - 1)}
          {/if}
        </main>
      {/each}
    </div>
  </div>
</div>

{#if menu}
  <Menu config={menu} onclose={() => (menu = null)} />
{/if}

{#if confirm}
  <ConfirmDialog
    config={confirm}
    onclose={() => {
      confirm = null
      answer?.(false)
      answer = null
      if (focused) focusList(focused)
    }}
  />
{/if}

{#if settings}
  <SettingsDialog
    bind:theme
    bind:themedIcons
    bind:showHidden
    bind:previewOpen
    bind:keepWindowSize
    bind:keepMaximized
    bind:checkUpdates
    bind:custom={customTerminals}
    bind:defaultTerminal
    effectiveTerminal={terminal?.id ?? ''}
    found={foundTerminals}
    bind:hidden={hiddenTerminals}
    {version}
    section={settings}
    oncheck={() => checkForUpdate(false)}
    onclose={() => {
      settings = null
      if (focused) focusList(focused)
    }}
  />
{/if}

{#if release}
  <div class="update" role="status">
    <span>SashaExplorer {release.version} is available.</span>
    <button type="button" class="primary" onclick={() => release && api.OpenURL(release.url)}>What's new</button>
    <button type="button" onclick={skipRelease}>Skip this version</button>
    <button type="button" class="icon" aria-label="Later" title="Later" onclick={() => (release = null)}>
      <Icon name="close" size={12} />
    </button>
  </div>
{/if}

{#if toast}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
  <div class="toast" class:info={!toastError} role="alert" onclick={() => (toast = '')}>{toast}</div>
{/if}

<style>
  .layout {
    display: grid;
    height: 100%;
  }

  .side {
    position: relative;
    display: flex;
    min-height: 0;
    border-right: 1px solid var(--border);
  }

  .side > :global(aside) {
    flex: 1;
  }

  .splitter {
    flex: none;
    width: 5px;
    margin: 0 -2px;
    z-index: 3;
    cursor: col-resize;
  }

  .splitter:hover {
    background: var(--accent-soft);
  }

  .side-splitter {
    position: absolute;
    top: 0;
    right: -3px;
    bottom: 0;
    margin: 0;
  }

  .pane-splitter {
    border-left: 1px solid var(--border);
  }

  .work {
    display: flex;
    min-width: 0;
    min-height: 0;
  }

  .panes {
    display: flex;
    flex: 1;
    min-width: 0;
  }

  main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    container-type: inline-size;
  }

  /* A narrow pane keeps the buttons and drops their words. */
  @container (max-width: 640px) {
    .labeled span,
    .search-chip {
      display: none;
    }

    .labeled {
      width: 28px;
      padding: 0;
      justify-content: center;
    }

    .toolbar .group .icon:nth-child(4),
    .divider {
      display: none;
    }

    .spacer {
      flex: 0;
    }
  }

  .toolbar,
  .commands {
    display: flex;
    flex: none;
    align-items: center;
    gap: 6px;
    padding: 5px 8px 3px;
    background: var(--bg);
  }

  .commands {
    flex-wrap: wrap;
    gap: 1px;
    padding: 1px 8px 4px;
    border-bottom: 1px solid var(--border);
  }

  .group {
    display: flex;
    flex: none;
    align-items: center;
    gap: 2px;
  }

  .divider {
    flex: none;
    width: 1px;
    height: 20px;
    margin: 0 6px;
    background: var(--border);
  }

  .spacer {
    flex: 1;
  }

  .flat {
    flex: none;
    height: 28px;
    border-color: transparent;
    background: transparent;
    color: var(--text-muted);
  }

  .flat:hover:not(:disabled) {
    border-color: transparent;
    background: var(--surface-raised);
    color: var(--text);
  }

  .flat.on {
    color: var(--accent);
  }

  .flat.icon {
    width: 28px;
  }

  .labeled {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 0 8px;
    font-size: 13px;
  }

  .danger-button:hover:not(:disabled) {
    color: var(--danger);
  }

  .lucy:hover:not(:disabled) {
    color: var(--cyan);
  }

  .segmented {
    display: flex;
    flex: none;
    margin-right: 6px;
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .segmented button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 22px;
    padding: 0;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-muted);
  }

  .segmented button.on {
    background: var(--selected);
    color: var(--accent);
  }

  .filter {
    display: flex;
    flex: 0 1 220px;
    align-items: center;
    gap: 6px;
    min-width: 90px;
    height: 30px;
    margin: 0;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
    color: var(--text-muted);
  }

  .filter:focus-within,
  .filter.active-filter {
    border-color: var(--accent);
  }

  .filter input {
    flex: 1;
    min-width: 0;
    height: 26px;
    padding: 0;
    border: none;
    background: transparent;
  }

  .filter input:focus-visible {
    outline: none;
  }

  .search-chip {
    flex: none;
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--selected);
    color: var(--accent);
    font-size: 11px;
    white-space: nowrap;
  }

  .clear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-muted);
  }

  .content {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  .column {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }

  .list {
    position: relative;
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
  }

  .loading {
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    height: 2px;
    overflow: hidden;
    background: linear-gradient(90deg, transparent, var(--accent), var(--cyan), transparent);
    background-size: 50% 100%;
    background-repeat: no-repeat;
    animation: sweep 1s linear infinite;
  }

  @keyframes sweep {
    from {
      background-position: -50% 0;
    }
    to {
      background-position: 150% 0;
    }
  }

  .split-zone {
    position: absolute;
    z-index: 5;
    top: 6px;
    bottom: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px dashed var(--accent);
    border-radius: var(--radius);
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
    pointer-events: none;
  }

  .split-zone.right {
    right: 6px;
    left: 50%;
  }

  .split-zone.left {
    right: 50%;
    left: 6px;
  }

  .split-zone.all {
    right: 6px;
    left: 6px;
  }

  .preview {
    flex: none;
    min-width: 0;
    border-left: 1px solid var(--border);
  }

  footer {
    display: flex;
    flex: none;
    gap: 16px;
    height: 26px;
    align-items: center;
    padding: 0 12px;
    border-top: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-muted);
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
  }

  .focused-pane footer {
    box-shadow: inset 0 1px 0 var(--accent-soft);
  }

  .accent {
    color: var(--accent);
  }

  .link {
    height: 20px;
    padding: 0 4px;
    border: none;
    background: transparent;
    color: var(--accent);
    font-size: 12px;
  }

  .toast {
    position: fixed;
    z-index: 60;
    right: 20px;
    bottom: 36px;
    max-width: 460px;
    padding: 10px 14px;
    border: 1px solid var(--danger);
    border-radius: var(--radius);
    background: var(--surface-raised);
    color: var(--danger);
    overflow-wrap: anywhere;
  }

  .split-button {
    display: inline-flex;
    flex: none;
    align-items: center;
  }

  .split-button .lucy {
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }

  .more {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    padding: 0;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
  }

  .update {
    position: fixed;
    z-index: 55;
    right: 20px;
    bottom: 36px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 8px 8px 14px;
    border: 1px solid var(--accent);
    border-radius: var(--radius);
    background: var(--surface-raised);
    box-shadow: 0 8px 24px var(--shadow);
  }

  .update button {
    height: 30px;
    font-size: 13px;
  }

  .update .icon {
    width: 30px;
    border-color: transparent;
    background: transparent;
  }

  .toast.info {
    border-color: var(--accent);
    color: var(--text);
  }
</style>
