<script lang="ts">
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'
  import { dropTarget, type DropHandler } from './drag'
  import { crumbs, RECYCLE_BIN } from './util'

  interface Props {
    path: string
    /** Display names for paths, such as drive labels. */
    labelFor: (path: string, fallback: string) => string
    onnavigate: (path: string) => void
    onsubmit: (text: string) => void
    ondropfiles: DropHandler
  }

  let { path, labelFor, onnavigate, onsubmit, ondropfiles }: Props = $props()
  let editing = $state(false)
  let text = $state('')
  let input = $state<HTMLInputElement>()
  let bar = $state<HTMLElement>()
  let parts = $derived(crumbs(path))

  export async function edit() {
    text = path === RECYCLE_BIN ? 'Recycle Bin' : path
    editing = true
    await tick()
    input?.select()
  }

  function submit(event: SubmitEvent) {
    event.preventDefault()
    editing = false
    const value = text.trim()
    if (value && value !== path && !(path === RECYCLE_BIN && value === 'Recycle Bin')) onsubmit(value)
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      editing = false
    }
  }

  /** Clicks on the empty part of the bar switch to typing a path. */
  function barClick(event: MouseEvent) {
    if ((event.target as HTMLElement).closest('button')) return
    edit()
  }

  /** The end of a long path matters most, so the bar starts scrolled there. */
  $effect(() => {
    void parts
    if (!editing && bar) bar.scrollLeft = bar.scrollWidth
  })
</script>

{#if editing}
  <form class="address" onsubmit={submit}>
    <input
      bind:this={input}
      bind:value={text}
      aria-label="Address"
      spellcheck="false"
      autocomplete="off"
      onkeydown={keydown}
      onblur={() => (editing = false)}
    />
  </form>
{:else}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="address crumbs" bind:this={bar} onclick={barClick} title="Click an empty spot to type a path (Ctrl+L)">
    {#each parts as part, index (part.path)}
      {#if index > 0}
        <span class="separator" aria-hidden="true"><Icon name="chevron" size={12} /></span>
      {/if}
      <button
        type="button"
        class="crumb"
        class:last={index === parts.length - 1}
        onclick={() => onnavigate(part.path)}
        use:dropTarget={{ path: part.path, ondrop: ondropfiles }}
      >
        {index === 1 ? labelFor(part.path, part.label) : part.label}
      </button>
    {/each}
    <span class="filler"></span>
  </div>
{/if}

<style>
  .address {
    display: flex;
    flex: 1;
    align-items: center;
    min-width: 140px;
    height: 30px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
  }

  .address input {
    height: 28px;
    border: none;
    background: transparent;
  }

  .address input:focus-visible {
    outline: none;
  }

  form.address:focus-within {
    border-color: var(--accent);
  }

  .crumbs {
    padding: 0 4px;
    overflow-x: auto;
    cursor: text;
    scrollbar-width: none;
  }

  .crumbs::-webkit-scrollbar {
    display: none;
  }

  .crumb {
    flex: none;
    height: 24px;
    padding: 0 6px;
    border: none;
    border-radius: 5px;
    background: transparent;
    color: var(--text-muted);
    white-space: nowrap;
    cursor: pointer;
  }

  .crumb:hover {
    background: var(--surface-raised);
    color: var(--text);
  }

  .crumb.last {
    color: var(--text);
  }

  .separator {
    display: inline-flex;
    flex: none;
    color: var(--text-muted);
    opacity: 0.6;
  }

  .filler {
    flex: 1;
    align-self: stretch;
    min-width: 24px;
  }
</style>
