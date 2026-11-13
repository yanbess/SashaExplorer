<script lang="ts" module>
  /** Kinds of items that get their own drawing and color. */
  type Kind =
    | 'folder'
    | 'drive'
    | 'image'
    | 'video'
    | 'audio'
    | 'archive'
    | 'code'
    | 'text'
    | 'pdf'
    | 'doc'
    | 'sheet'
    | 'slides'
    | 'data'
    | 'config'
    | 'disc'
    | 'font'
    | 'file'

  const EXTENSIONS: Record<string, Kind> = {}
  const groups: [Kind, string][] = [
    ['image', 'jpg jpeg jfif png gif webp bmp ico svg avif tif tiff heic heif raw cr2 cr3 nef arw dng psd jxl'],
    ['video', 'mp4 m4v webm mov ogv mkv avi wmv flv mpg mpeg 3gp ts m2ts'],
    ['audio', 'mp3 m4a aac wav ogg oga opus flac wma mid midi'],
    ['archive', 'zip rar 7z tar gz tgz bz2 xz zst cab'],
    [
      'code',
      'js mjs cjs ts tsx jsx svelte vue go py rb php java kt kts c h cpp hpp cc cs rs swift sh bash zsh ps1 psm1 bat cmd lua pl r sql html htm css scss less',
    ],
    ['text', 'txt md markdown log rst adoc nfo srt vtt tex'],
    ['pdf', 'pdf'],
    ['doc', 'doc docx odt rtf pages'],
    ['sheet', 'xls xlsx ods csv tsv numbers'],
    ['slides', 'ppt pptx odp key'],
    ['data', 'json jsonc xml yaml yml toml db sqlite sqlite3 mdb accdb parquet'],
    ['config', 'ini cfg conf env properties reg gitignore gitattributes editorconfig lock mod sum'],
    ['disc', 'iso img vhd vhdx dmg'],
    ['font', 'ttf otf woff woff2'],
  ]
  for (const [kind, list] of groups) for (const ext of list.split(' ')) EXTENSIONS['.' + ext] = kind

  /** Neon colors that sit with the pink accent of the app. */
  const COLORS: Record<Kind, string> = {
    folder: '#e58cf0',
    drive: '#6fdcf0',
    image: '#b48cff',
    video: '#ff6fb5',
    audio: '#ff9f6f',
    archive: '#f2d06b',
    code: '#6fdcf0',
    text: '#c3cbd4',
    pdf: '#ff6b81',
    doc: '#7aa2ff',
    sheet: '#6fe3a0',
    slides: '#ffb86c',
    data: '#8be9fd',
    config: '#a6b3c2',
    disc: '#9fa8ff',
    font: '#f0aef6',
    file: '#98a3ad',
  }

  /** The mark drawn inside a file, around the middle of its lower part. */
  const MARKS: Partial<Record<Kind, string>> = {
    image: 'M8.5 18.5l2.6-3.2 1.9 2 1.4-1.5 2.1 2.7zM14.6 12.6h.01',
    video: 'M10.3 11.6v6l4.8-3z',
    audio: 'M10.6 17.6a1.4 1.4 0 1 1-1.4-1.4h1.4v-5.4l4.6-1.1v5.4a1.4 1.4 0 1 1-1.4-1.4h1.4',
    archive: 'M12 7v1.2M12 9.4v1.2M12 11.8v1.2M10.6 14.4h2.8v3.2h-2.8z',
    code: 'M10.4 12l-2.2 2.6 2.2 2.6M13.6 12l2.2 2.6-2.2 2.6',
    text: 'M8.8 11.5h6.4M8.8 14.2h6.4M8.8 16.9h4',
    pdf: 'M8.8 11.5h6.4M8.8 14.2h4.6M8.8 16.9h6.4',
    doc: 'M8.6 11.6l1.2 5.4 1.4-4 1.4 4 1.2-5.4M15.2 11.6',
    sheet: 'M8.6 11.2h6.8v6.6H8.6zM8.6 14.5h6.8M12 11.2v6.6',
    slides: 'M8.4 11.4h7.2v4.8H8.4zM12 16.2v2.2M10.4 18.4h3.2',
    data: 'M12 10.8c1.9 0 3.4.5 3.4 1.2v5c0 .7-1.5 1.2-3.4 1.2s-3.4-.5-3.4-1.2v-5c0-.7 1.5-1.2 3.4-1.2zM8.6 12c0 .7 1.5 1.2 3.4 1.2s3.4-.5 3.4-1.2M8.6 14.5c0 .7 1.5 1.2 3.4 1.2s3.4-.5 3.4-1.2',
    config: 'M12 16.2a1.8 1.8 0 1 0 0-3.6 1.8 1.8 0 0 0 0 3.6zM12 10.6v1.2M12 17v1.2M9.2 14.4h1.2M13.6 14.4h1.2',
    disc: 'M12 18.4a3.8 3.8 0 1 0 0-7.6 3.8 3.8 0 0 0 0 7.6zM12 15.3a.7.7 0 1 0 0-1.4.7.7 0 0 0 0 1.4z',
    font: 'M9.4 18l2.6-6.6 2.6 6.6M10.4 15.8h3.2',
    file: 'M8.8 13.5h6.4M8.8 16.2h4',
  }

  export function iconKind(entry: { dir: boolean; ext: string }, drive = false): Kind {
    if (drive) return 'drive'
    if (entry.dir) return 'folder'
    return EXTENSIONS[entry.ext] ?? 'file'
  }
</script>

<script lang="ts">
  interface Props {
    entry: { dir: boolean; ext: string }
    drive?: boolean
    size?: number
  }

  let { entry, drive = false, size = 18 }: Props = $props()
  let kind = $derived(iconKind(entry, drive))
  let color = $derived(COLORS[kind])
  /** Large icons glow a little, like the neon of the app icon. */
  let glow = $derived(size >= 40)
</script>

<svg
  class="file-icon"
  class:glow
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke={color}
  stroke-width={size >= 40 ? 1.1 : 1.6}
  stroke-linecap="round"
  stroke-linejoin="round"
  style:--glow={color}
  aria-hidden="true"
>
  {#if kind === 'folder'}
    <path d="M2.8 6.2a1.2 1.2 0 0 1 1.2-1.2h5.1l2 2.4h8.9a1.2 1.2 0 0 1 1.2 1.2v9.9a1.2 1.2 0 0 1-1.2 1.2H4a1.2 1.2 0 0 1-1.2-1.2z" fill={color} fill-opacity="0.16" />
    <path d="M2.8 10.2h18.4" stroke-opacity="0.75" />
  {:else if kind === 'drive'}
    <path d="M3.4 13.4h17.2v5.4a1.2 1.2 0 0 1-1.2 1.2H4.6a1.2 1.2 0 0 1-1.2-1.2z" fill={color} fill-opacity="0.16" />
    <path d="M3.4 13.4l2.8-7.6h11.6l2.8 7.6" />
    <path d="M6.4 16.7h.01M16 16.7h2" stroke-width="2" />
  {:else}
    <path d="M6 2.6h8.4l4.2 4.2v13.4a1.2 1.2 0 0 1-1.2 1.2H6a1.2 1.2 0 0 1-1.2-1.2V3.8A1.2 1.2 0 0 1 6 2.6z" fill={color} fill-opacity="0.13" />
    <path d="M14.4 2.6v4.2h4.2" />
    <path d={MARKS[kind]} />
  {/if}
</svg>

<style>
  .file-icon {
    flex: none;
    filter: var(--icon-filter);
  }

  .glow {
    filter: var(--icon-filter) drop-shadow(0 0 6px color-mix(in srgb, var(--glow) 45%, transparent));
  }
</style>
