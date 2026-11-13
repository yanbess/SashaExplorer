<script lang="ts" module>
  export interface MenuItem {
    label: string
    hint?: string
    disabled?: boolean
    checked?: boolean
    danger?: boolean
    action: () => void
  }

  export type MenuEntry = MenuItem | 'separator'

  export interface MenuConfig {
    x: number
    y: number
    items: MenuEntry[]
  }
</script>

<script lang="ts">
  import { onMount, tick } from 'svelte'

  interface Props {
    config: MenuConfig
    onclose: () => void
  }

  let { config, onclose }: Props = $props()
  let element = $state<HTMLElement>()
  let left = $state(0)
  let top = $state(0)

  /** Separators next to each other or at an end are left out. */
  let entries = $derived(
    config.items.filter(
      (item, index, all) =>
        item !== 'separator' || (index > 0 && index < all.length - 1 && all[index - 1] !== 'separator'),
    ),
  )
  let hasChecks = $derived(config.items.some((item) => item !== 'separator' && item.checked !== undefined))

  onMount(async () => {
    left = config.x
    top = config.y
    await tick()
    if (!element) return
    const box = element.getBoundingClientRect()
    left = Math.max(4, Math.min(config.x, window.innerWidth - box.width - 8))
    top = Math.max(4, Math.min(config.y, window.innerHeight - box.height - 8))
    element.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
  })

  function choose(item: MenuItem) {
    onclose()
    item.action()
  }

  function outsideClick(event: MouseEvent) {
    if (!element?.contains(event.target as Node)) onclose()
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      onclose()
      return
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    event.stopPropagation()
    const items = [...(element?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])]
    const index = items.indexOf(document.activeElement as HTMLButtonElement)
    const step = event.key === 'ArrowDown' ? 1 : -1
    items[(index + step + items.length) % items.length]?.focus()
  }
</script>

<svelte:window onmousedowncapture={outsideClick} onblur={onclose} onresize={onclose} />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions, a11y_interactive_supports_focus -->
<div
  class="menu"
  role="menu"
  tabindex="-1"
  style={`left: ${left}px; top: ${top}px`}
  bind:this={element}
  onkeydown={keydown}
  oncontextmenu={(event) => event.preventDefault()}
>
  {#each entries as item, index (index)}
    {#if item === 'separator'}
      <hr />
    {:else}
      <button
        type="button"
        role={item.checked !== undefined ? 'menuitemradio' : 'menuitem'}
        aria-checked={item.checked}
        class:danger={item.danger}
        disabled={item.disabled}
        onclick={() => choose(item)}
      >
        {#if hasChecks}
          <span class="check" aria-hidden="true">{item.checked ? '●' : ''}</span>
        {/if}
        <span class="label">{item.label}</span>
        {#if item.hint}<span class="hint">{item.hint}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 50;
    display: flex;
    flex-direction: column;
    min-width: 220px;
    max-height: calc(100vh - 16px);
    padding: 4px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-raised);
    box-shadow: 0 8px 24px var(--shadow);
  }

  .menu:focus {
    outline: none;
  }

  button {
    display: flex;
    flex: none;
    align-items: center;
    gap: 8px;
    height: 30px;
    padding: 0 10px;
    border: none;
    border-radius: 5px;
    background: transparent;
    text-align: left;
    white-space: nowrap;
  }

  button:hover:not(:disabled),
  button:focus-visible {
    outline: none;
    background: var(--selected);
  }

  .label {
    flex: 1;
  }

  .check {
    width: 10px;
    color: var(--accent);
    font-size: 9px;
  }

  .danger {
    color: var(--danger);
  }

  .hint {
    margin-left: 16px;
    color: var(--text-muted);
    font-size: 12px;
  }

  hr {
    width: 100%;
    margin: 4px 0;
    border: none;
    border-top: 1px solid var(--border);
  }
</style>
