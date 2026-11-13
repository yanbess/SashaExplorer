<script lang="ts" module>
  export interface ConfirmConfig {
    title: string
    message?: string
    /** When set, the dialog asks for text with this label. */
    inputLabel?: string
    inputValue?: string
    /** Selects the name without its extension, as for a new file. */
    selectBase?: boolean
    confirmLabel: string
    danger?: boolean
    onconfirm: (value: string) => void | Promise<void>
  }
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import { errorText } from './util'

  interface Props {
    config: ConfirmConfig
    onclose: () => void
  }

  let { config, onclose }: Props = $props()
  let error = $state('')
  let value = $state(untrack(() => config.inputValue ?? ''))

  function open(element: HTMLDialogElement) {
    element.showModal()
    const input = element.querySelector('input')
    if (!input) return
    const dot = input.value.lastIndexOf('.')
    if (config.selectBase && dot > 0) input.setSelectionRange(0, dot)
    else input.select()
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    try {
      await config.onconfirm(value.trim())
      onclose()
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

<dialog use:open oncancel={onclose}>
  <form onsubmit={submit}>
    <h2>{config.title}</h2>
    {#if config.message}
      <p class="message">{config.message}</p>
    {/if}
    {#if config.inputLabel !== undefined}
      <div>
        <label for="confirm-input">{config.inputLabel}</label>
        <input id="confirm-input" autocomplete="off" spellcheck="false" bind:value />
      </div>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Cancel</button>
      <button
        type="submit"
        class={config.danger ? 'danger' : 'primary'}
        disabled={config.inputLabel !== undefined && !value.trim()}
      >
        {config.confirmLabel}
      </button>
    </div>
  </form>
</dialog>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
    width: 400px;
  }

  h2 {
    font-size: 16px;
    font-weight: 600;
  }

  .message {
    color: var(--text-muted);
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
