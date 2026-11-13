<script lang="ts">
  import FileIcon from './FileIcon.svelte'
  import { folderPictures } from './folderPictures'
  import { thumbUrl } from './util'

  interface Props {
    folder: { path: string; modified: number }
    size: number
  }

  let { folder, size }: Props = $props()

  const COLOR = '#e58cf0'
  let pictures = $derived(folderPictures(folder) ?? [])
  /** Thumbnails come in two sizes, so the window can reuse them. */
  let thumbSize = $derived(size * devicePixelRatio > 160 ? 256 : 128)

  /** Cards fan out from the middle: one stands straight, three lean apart. */
  function card(index: number, count: number) {
    const offset = index - (count - 1) / 2
    return {
      left: 50 + offset * 17,
      top: 40 - Math.abs(offset) * 3,
      rotate: offset * 9,
    }
  }
</script>

{#if pictures.length === 0}
  <FileIcon entry={{ dir: true, ext: '' }} {size} />
{:else}
  <span class="folder" style:width={`${size}px`} style:height={`${size}px`} style:--glow={COLOR}>
    <svg class="back" viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M2.8 6.2a1.2 1.2 0 0 1 1.2-1.2h5.1l2 2.4h8.9a1.2 1.2 0 0 1 1.2 1.2v9.9a1.2 1.2 0 0 1-1.2 1.2H4a1.2 1.2 0 0 1-1.2-1.2z"
        fill={COLOR}
        fill-opacity="0.16"
        stroke={COLOR}
        stroke-width="1.1"
        stroke-linejoin="round"
      />
    </svg>
    {#each pictures as picture, index (picture.path)}
      {@const place = card(index, pictures.length)}
      <img
        class="card"
        class:single={pictures.length === 1}
        src={thumbUrl(picture.path, thumbSize, picture.modified)}
        alt=""
        loading="lazy"
        draggable="false"
        style:left={`${place.left}%`}
        style:top={`${place.top}%`}
        style:transform={`translate(-50%, -50%) rotate(${place.rotate}deg)`}
        style:z-index={index === Math.floor(pictures.length / 2) ? 2 : 1}
        onerror={(event) => ((event.currentTarget as HTMLImageElement).style.display = 'none')}
      />
    {/each}
    <svg class="front" viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M2.8 11.4a1.2 1.2 0 0 1 1.2-1.2h16a1.2 1.2 0 0 1 1.2 1.2v7.1a1.2 1.2 0 0 1-1.2 1.2H4a1.2 1.2 0 0 1-1.2-1.2z"
        style:fill="color-mix(in srgb, var(--bg) 62%, #e58cf0)"
        fill-opacity="0.94"
        stroke={COLOR}
        stroke-width="1.1"
        stroke-linejoin="round"
      />
    </svg>
  </span>
{/if}

<style>
  .folder {
    position: relative;
    display: inline-block;
    flex: none;
    filter: drop-shadow(0 0 6px color-mix(in srgb, var(--glow) 40%, transparent));
  }

  svg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .front {
    z-index: 3;
  }

  .card {
    position: absolute;
    width: 40%;
    height: 34%;
    border: 1px solid rgb(255 255 255 / 75%);
    border-radius: 3px;
    background: var(--surface-raised);
    box-shadow: 0 1px 4px rgb(0 0 0 / 45%);
    object-fit: cover;
  }

  .card.single {
    width: 56%;
    height: 40%;
  }
</style>
