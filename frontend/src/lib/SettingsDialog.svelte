<script lang="ts" module>
  export type Theme = 'system' | 'light' | 'dark'

  export interface TerminalInfo {
    id: string
    name: string
    custom: boolean
    path: string
    args: string
  }

  export type SettingsSection = 'appearance' | 'window' | 'terminal' | 'explorer' | 'updates' | 'about'
</script>

<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import * as api from '../../wailsjs/go/main/App'
  import ConfirmDialog, { type ConfirmConfig } from './ConfirmDialog.svelte'
  import Icon from './Icon.svelte'
  import { errorText, formatSize } from './util'

  interface Props {
    theme: Theme
    themedIcons: boolean
    showHidden: boolean
    previewOpen: boolean
    keepWindowSize: boolean
    keepMaximized: boolean
    checkUpdates: boolean
    /** Terminals found on the computer. */
    found: TerminalInfo[]
    /** IDs of found terminals deleted from the list. */
    hidden: string[]
    /** Terminals added by hand. */
    custom: TerminalInfo[]
    /** The terminal chosen as the default, or empty. */
    defaultTerminal: string
    /** The terminal the button opens now, chosen or not. */
    effectiveTerminal: string
    version: string
    section?: SettingsSection
    /** Asks GitHub now and returns what to tell. */
    oncheck: () => Promise<string>
    onclose: () => void
  }

  let {
    theme = $bindable(),
    themedIcons = $bindable(),
    showHidden = $bindable(),
    previewOpen = $bindable(),
    keepWindowSize = $bindable(),
    keepMaximized = $bindable(),
    checkUpdates = $bindable(),
    found,
    hidden = $bindable(),
    custom = $bindable(),
    defaultTerminal = $bindable(),
    effectiveTerminal,
    version,
    section = 'appearance',
    oncheck,
    onclose,
  }: Props = $props()

  const SECTIONS: { id: SettingsSection; label: string; icon: import('./Icon.svelte').IconName }[] = [
    { id: 'appearance', label: 'Appearance', icon: 'palette' },
    { id: 'window', label: 'Window', icon: 'panel' },
    { id: 'terminal', label: 'Terminal', icon: 'terminal' },
    { id: 'explorer', label: 'Explorer', icon: 'folder' },
    { id: 'updates', label: 'Updates', icon: 'reload' },
    { id: 'about', label: 'About', icon: 'info' },
  ]

  const THEMES: { id: Theme; label: string }[] = [
    { id: 'system', label: 'System' },
    { id: 'light', label: 'Light' },
    { id: 'dark', label: 'Dark' },
  ]

  let current = $state<SettingsSection>(untrack(() => section))
  let explorerMenu = $state(false)
  let foldersDefault = $state(false)
  let cacheSize = $state(0)
  let message = $state('')
  let error = $state('')
  let checking = $state(false)
  let windowReset = $state(false)

  /** The terminal being added or changed, or null. */
  let editing = $state<TerminalInfo | null>(null)
  let editError = $state('')

  let shown = $derived(found.filter((item) => !hidden.includes(item.id)))
  let all = $derived([...shown, ...custom])

  onMount(async () => {
    try {
      ;[explorerMenu, foldersDefault, cacheSize] = await Promise.all([
        api.ExplorerMenu(),
        api.FoldersDefault(),
        api.ThumbnailCacheSize(),
      ])
    } catch {
      // Shown as off.
    }
  })

  function open(element: HTMLDialogElement) {
    element.showModal()
  }

  function say(text: string, failed = false) {
    message = failed ? '' : text
    error = failed ? text : ''
  }

  function show(id: SettingsSection) {
    current = id
    message = ''
    error = ''
  }

  async function setExplorerMenu(enabled: boolean) {
    try {
      await api.SetExplorerMenu(enabled)
      explorerMenu = enabled
      say(enabled ? 'Added to the Explorer menu, under "Show more options" on Windows 11.' : 'Removed from the Explorer menu.')
    } catch (e) {
      say(errorText(e), true)
    }
  }

  async function setFoldersDefault(enabled: boolean) {
    try {
      await api.SetFoldersDefault(enabled)
      foldersDefault = enabled
      say(
        enabled
          ? 'Folders and Win+E now open here. "Show in folder" in other programs still opens Explorer.'
          : 'Folders open in Explorer again.',
      )
    } catch (e) {
      say(errorText(e), true)
    }
  }

  async function clearCache() {
    try {
      await api.ClearThumbnailCache()
      cacheSize = 0
      say('Saved thumbnails were removed. They are made again as folders are opened.')
    } catch (e) {
      say(errorText(e), true)
    }
  }

  async function resetWindow() {
    await api.ResetWindow()
    windowReset = true
  }

  async function checkNow() {
    checking = true
    try {
      say(await oncheck())
    } catch (e) {
      say(errorText(e), true)
    } finally {
      checking = false
    }
  }

  function addTerminal() {
    editError = ''
    editing = { id: `custom:${crypto.randomUUID()}`, name: '', custom: true, path: '', args: '{folder}' }
  }

  function editTerminal(terminal: TerminalInfo) {
    editError = ''
    editing = { ...terminal }
  }

  async function browse() {
    if (!editing) return
    try {
      const path = await api.PickProgram()
      if (!path) return
      editing.path = path
      if (!editing.name.trim()) {
        editing.name = (path.split('\\').pop() ?? path).replace(/\.(exe|cmd|bat)$/i, '')
      }
    } catch (e) {
      editError = errorText(e)
    }
  }

  function saveTerminal(event: SubmitEvent) {
    event.preventDefault()
    if (!editing) return
    const value = { ...editing, name: editing.name.trim(), path: editing.path.trim(), args: editing.args.trim() }
    if (!value.path) {
      editError = 'Choose the program to start.'
      return
    }
    if (!value.name) value.name = value.path
    const index = custom.findIndex((item) => item.id === value.id)
    custom = index >= 0 ? custom.map((item) => (item.id === value.id ? value : item)) : [...custom, value]
    editing = null
  }

  /** The question before a terminal is deleted, or null. */
  let confirm = $state<ConfirmConfig | null>(null)

  /** Deletes a terminal from the list for good, after asking. A found one is
   *  remembered as deleted, so it is not found again. */
  function removeTerminal(terminal: TerminalInfo) {
    confirm = {
      title: `Delete ${terminal.name}?`,
      message: terminal.custom
        ? 'It leaves the list of terminals. You can add it again later.'
        : 'It leaves the list of terminals and will not be found again. You can still add it by hand.',
      confirmLabel: 'Delete',
      danger: true,
      onconfirm: () => {
        if (terminal.custom) custom = custom.filter((item) => item.id !== terminal.id)
        else hidden = [...hidden, terminal.id]
        if (defaultTerminal === terminal.id) defaultTerminal = ''
      },
    }
  }

  function isDefault(terminal: TerminalInfo): boolean {
    return terminal.id === effectiveTerminal
  }
</script>

<dialog use:open oncancel={onclose} aria-label="Settings">
  <div class="frame">
    <nav>
      <h2>Settings</h2>
      {#each SECTIONS as item (item.id)}
        <button type="button" class="nav" class:on={current === item.id} aria-current={current === item.id} onclick={() => show(item.id)}>
          <Icon name={item.icon} size={15} />
          <span>{item.label}</span>
        </button>
      {/each}
    </nav>

    <div class="page">
      <button type="button" class="icon close" aria-label="Close settings" title="Close (Esc)" onclick={onclose}>
        <Icon name="close" size={14} />
      </button>

      {#if current === 'appearance'}
        <h3>Appearance</h3>
        <div class="row">
          <div class="text">
            <span class="label">Theme</span>
            <span class="hint">System follows the colors Windows uses for apps.</span>
          </div>
          <div class="segmented" role="radiogroup" aria-label="Theme">
            {#each THEMES as item (item.id)}
              <button type="button" role="radio" aria-checked={theme === item.id} class:on={theme === item.id} onclick={() => (theme = item.id)}>
                {item.label}
              </button>
            {/each}
          </div>
        </div>
        <label class="row switch">
          <span class="text">
            <span class="label">Show hidden files</span>
            <span class="hint">Hidden files and folders appear dimmed in the list.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={showHidden} />
        </label>
        <label class="row switch">
          <span class="text">
            <span class="label">Sasha icons</span>
            <span class="hint">Files and folders in the style of the app. Programs, shortcuts, pictures and videos keep their own look.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={themedIcons} />
        </label>
        <label class="row switch">
          <span class="text">
            <span class="label">Preview panel</span>
            <span class="hint">Shows the selected item on the right.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={previewOpen} />
        </label>
      {:else if current === 'window'}
        <h3>Window</h3>
        <label class="row switch">
          <span class="text">
            <span class="label">Remember size and position</span>
            <span class="hint">The window opens where it was closed and as large.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={keepWindowSize} />
        </label>
        <label class="row switch">
          <span class="text">
            <span class="label">Remember maximized</span>
            <span class="hint">A window closed maximized opens maximized.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={keepMaximized} />
        </label>
        <div class="row">
          <div class="text">
            <span class="label">Window size</span>
            <span class="hint">{windowReset ? 'The window is back to its first size.' : 'Back to the first size, in the middle of the screen.'}</span>
          </div>
          <button type="button" onclick={resetWindow}>Reset</button>
        </div>
      {:else if current === 'terminal'}
        <h3>Terminal</h3>
        <p class="intro">
          The terminal button opens the folder in the default terminal; its arrow lists the others. SashaExplorer finds the
          terminals on this computer by itself, LucyShell when it is next to the program. Delete the ones you do not use, and
          add your own.
        </p>
        <ul class="terminals" role="radiogroup" aria-label="Default terminal">
          {#each all as terminal (terminal.id)}
            <li>
              <label class="choice">
                <input type="radio" name="default-terminal" checked={isDefault(terminal)} onchange={() => (defaultTerminal = terminal.id)} />
                <span class="text">
                  <span class="label">{terminal.name} <span class="origin">{terminal.custom ? 'added by you' : 'found'}</span></span>
                  <span class="hint path" title={terminal.path}>{terminal.path}{terminal.custom && terminal.args ? ` ${terminal.args}` : ''}</span>
                </span>
              </label>
              {#if terminal.custom}
                <button type="button" class="icon small" aria-label={`Change ${terminal.name}`} title="Change" onclick={() => editTerminal(terminal)}>
                  <Icon name="edit" size={14} />
                </button>
              {/if}
              <button
                type="button"
                class="icon small"
                aria-label={`Remove ${terminal.name}`}
                title="Delete"
                onclick={() => removeTerminal(terminal)}
              >
                <Icon name="trash" size={14} />
              </button>
            </li>
          {/each}
          {#if all.length === 0}
            <li class="hint">No terminal was found. Add one below.</li>
          {/if}
        </ul>

        {#if editing}
          <form class="editor" onsubmit={saveTerminal}>
            <h4>{custom.some((item) => item.id === editing?.id) ? 'Change terminal' : 'Add a terminal'}</h4>
            <div>
              <label for="terminal-name">Name</label>
              <input id="terminal-name" bind:value={editing.name} placeholder="My shell" spellcheck="false" autocomplete="off" />
            </div>
            <div>
              <label for="terminal-path">Program</label>
              <div class="pick">
                <input id="terminal-path" bind:value={editing.path} placeholder="C:\Tools\shell.exe" spellcheck="false" autocomplete="off" />
                <button type="button" onclick={browse}>Browse…</button>
              </div>
            </div>
            <div>
              <label for="terminal-args">Arguments</label>
              <input id="terminal-args" bind:value={editing.args} placeholder={'{folder}'} spellcheck="false" autocomplete="off" />
              <p class="hint">
                <code>{'{folder}'}</code> becomes the folder, quoted. The program also starts in that folder, so many shells need
                no arguments at all.
              </p>
            </div>
            {#if editError}
              <p class="error" role="alert">{editError}</p>
            {/if}
            <div class="buttons">
              <button type="button" onclick={() => (editing = null)}>Cancel</button>
              <button type="submit" class="primary">Save</button>
            </div>
          </form>
        {:else}
          <button type="button" class="add" onclick={addTerminal}><Icon name="plus" size={14} /> Add a terminal</button>
        {/if}
      {:else if current === 'explorer'}
        <h3>Explorer</h3>
        <label class="row switch">
          <span class="text">
            <span class="label">"Open in SashaExplorer" in Explorer</span>
            <span class="hint">
              Adds the item to the menu of folders and drives, in the current user's part of the registry. On Windows 11 it is
              under "Show more options". Moving the program means turning it on again.
            </span>
          </span>
          <input type="checkbox" role="switch" checked={explorerMenu} onchange={(event) => setExplorerMenu(event.currentTarget.checked)} />
        </label>
        <label class="row switch">
          <span class="text">
            <span class="label">Open folders and Win+E here</span>
            <span class="hint">
              Folders opened from the desktop or by other programs open here, and so does Win+E. What was there before comes
              back when this is turned off.
            </span>
          </span>
          <input type="checkbox" role="switch" checked={foldersDefault} onchange={(event) => setFoldersDefault(event.currentTarget.checked)} />
        </label>
        <div class="row">
          <div class="text">
            <span class="label">Saved thumbnails</span>
            <span class="hint">{formatSize(cacheSize)} in %LOCALAPPDATA%\SashaExplorer, encrypted for you.</span>
          </div>
          <button type="button" disabled={cacheSize === 0} onclick={clearCache}>Clear</button>
        </div>
      {:else if current === 'updates'}
        <h3>Updates</h3>
        <label class="row switch">
          <span class="text">
            <span class="label">Check for new versions</span>
            <span class="hint">Once a day, SashaExplorer asks GitHub for the latest release. Nothing is installed by itself.</span>
          </span>
          <input type="checkbox" role="switch" bind:checked={checkUpdates} />
        </label>
        <div class="row">
          <div class="text">
            <span class="label">Version {version}</span>
            <span class="hint">{version === 'dev' ? 'A development build, which is not compared with releases.' : 'The version running now.'}</span>
          </div>
          <button type="button" disabled={checking} onclick={checkNow}>{checking ? 'Checking…' : 'Check now'}</button>
        </div>
      {:else}
        <h3>About</h3>
        <p class="intro">
          SashaExplorer {version}: a file manager for Windows, made for fans of Cyberpunk: Edgerunners, with tabs, split
          panes, a gallery, and previews.
        </p>
        <div class="links">
          <button type="button" onclick={() => api.OpenURL('https://github.com/yanbess/SashaExplorer')}>Project page</button>
          <button type="button" onclick={() => api.OpenURL('https://github.com/yanbess/SashaExplorer/releases')}>Releases</button>
          <button type="button" onclick={() => api.OpenURL('https://github.com/yanbess/SashaExplorer/issues')}>Report a problem</button>
        </div>
        <p class="hint">
          Released under the MIT License. An unofficial fan project, not affiliated with CD PROJEKT RED or Studio Trigger.
        </p>
      {/if}

      {#if message}
        <p class="message" role="status">{message}</p>
      {/if}
      {#if confirm}
        <ConfirmDialog config={confirm} onclose={() => (confirm = null)} />
      {/if}
      {#if error}
        <p class="error" role="alert">{error}</p>
      {/if}
    </div>
  </div>
</dialog>

<style>
  dialog {
    width: min(820px, calc(100vw - 48px));
    height: min(560px, calc(100vh - 48px));
    padding: 0;
    overflow: hidden;
  }

  .frame {
    display: grid;
    grid-template-columns: 190px minmax(0, 1fr);
    height: 100%;
  }

  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 18px 10px;
    border-right: 1px solid var(--border);
    background: var(--bg);
  }

  h2 {
    margin: 0 8px 12px;
    font-size: 17px;
  }

  .nav {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 34px;
    padding: 0 10px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    text-align: left;
  }

  .nav:hover {
    background: var(--surface-raised);
    color: var(--text);
  }

  .nav.on {
    background: var(--selected);
    color: var(--text);
  }

  .nav.on :global(svg) {
    color: var(--accent);
  }

  .page {
    position: relative;
    padding: 20px 26px 24px;
    overflow-y: auto;
  }

  .close {
    position: absolute;
    top: 12px;
    right: 12px;
    width: 30px;
    height: 30px;
    border-color: transparent;
    background: transparent;
    color: var(--text-muted);
  }

  h3 {
    margin: 0 0 14px;
    font-size: 15px;
  }

  h4 {
    margin: 0 0 4px;
    font-size: 14px;
  }

  .intro {
    margin-bottom: 14px;
    color: var(--text-muted);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
    margin: 0;
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
    color: var(--text);
    font-size: 14px;
  }

  .row.switch {
    cursor: pointer;
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .label {
    color: var(--text);
  }

  .hint {
    color: var(--text-muted);
    font-size: 12px;
  }

  .origin {
    margin-left: 6px;
    padding: 0 6px;
    border-radius: 8px;
    background: var(--surface-raised);
    color: var(--text-muted);
    font-size: 11px;
  }

  .path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* A checkbox drawn as a switch. */
  input[role='switch'] {
    position: relative;
    flex: none;
    width: 38px;
    height: 22px;
    margin: 0;
    border: 1px solid var(--border);
    border-radius: 11px;
    background: var(--surface-raised);
    appearance: none;
    cursor: pointer;
    transition: background 0.15s;
  }

  input[role='switch']::after {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text-muted);
    content: '';
    transition: transform 0.15s;
  }

  input[role='switch']:checked {
    border-color: var(--accent);
    background: var(--accent);
  }

  input[role='switch']:checked::after {
    background: var(--on-accent);
    transform: translateX(16px);
  }

  .segmented {
    display: flex;
    flex: none;
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .segmented button {
    height: 28px;
    padding: 0 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-muted);
    font-size: 13px;
  }

  .segmented button.on {
    background: var(--selected);
    color: var(--accent);
  }

  .row > button,
  .links button,
  .add {
    flex: none;
    height: 32px;
    font-size: 13px;
  }

  .terminals {
    margin: 0 0 12px;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    list-style: none;
  }

  .terminals li {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    border-bottom: 1px solid var(--border);
  }

  .terminals li:last-child {
    border-bottom: none;
  }

  .choice {
    display: flex;
    flex: 1;
    align-items: center;
    gap: 10px;
    min-width: 0;
    margin: 0;
    padding: 4px 0;
    cursor: pointer;
  }

  .choice input {
    flex: none;
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }

  .small {
    width: 28px;
    height: 28px;
    border-color: transparent;
    background: transparent;
    color: var(--text-muted);
  }

  .small:hover:not(:disabled) {
    border-color: transparent;
    color: var(--text);
  }

  .add {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .editor {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
  }

  .editor input {
    height: 34px;
  }

  .pick {
    display: flex;
    gap: 6px;
  }

  .pick button {
    flex: none;
    height: 34px;
  }

  .editor .hint {
    margin-top: 4px;
  }

  code {
    padding: 0 4px;
    border-radius: 4px;
    background: var(--surface-raised);
    font-family: Consolas, monospace;
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  .buttons button {
    height: 34px;
  }

  .links {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 14px;
  }

  .message {
    margin-top: 14px;
    color: var(--success);
    font-size: 13px;
  }

  .error {
    margin-top: 14px;
    font-size: 13px;
  }
</style>
