<script lang="ts">
  import { untrack } from 'svelte'
  import * as api from '../../wailsjs/go/main/App'
  import type { files, media } from '../../wailsjs/go/models'
  import FileIcon from './FileIcon.svelte'
  import Icon from './Icon.svelte'
  import FolderThumb from './FolderThumb.svelte'
  import type { Entry } from './FileView.svelte'
  import {
    AUDIO,
    baseName,
    formatBitrate,
    errorText,
    fileUrl,
    formatDuration,
    formatExactSize,
    formatLongDate,
    formatSize,
    iconUrl,
    IMAGES,
    TEXT_FILES,
    thumbUrl,
    VIDEOS,
    WEB_AUDIO,
    WEB_IMAGES,
    WEB_VIDEOS,
    samePath,
    hasOwnIcon,
    parentPath,
  } from './util'

  interface Props {
    /** The folder shown in the list. */
    folder: string
    folderLabel: string
    selected: Entry[]
    /** Items shown in the list, for the summary when nothing is selected. */
    count: number
    types: Record<string, string>
    /** Folder totals the list already has. */
    sizes: Record<string, number> | null
    drives: files.Drive[]
    driveLabel: (path: string) => string
    /** Icons drawn in the style of the app instead of the Windows ones. */
    themed: boolean
    /** What the Recycle Bin holds, when the list shows it. */
    bin: { items: number; bytes: number } | null
  }

  let { folder, folderLabel, selected, count, types, sizes, drives, driveLabel, themed, bin }: Props = $props()

  type Kind = 'image' | 'video' | 'audio' | 'text' | 'thumb'

  let single = $derived(selected.length === 1 ? selected[0] : null)
  /** A drive chosen in This PC, or the drive whose root the list shows. */
  let drive = $derived(
    single
      ? drives.find((item) => samePath(item.path, single.path))
      : selected.length === 0 && folder
        ? drives.find((item) => samePath(item.path, folder))
        : undefined,
  )

  const DRIVE_KINDS: Record<string, string> = {
    fixed: 'Local disk',
    removable: 'Removable disk',
    network: 'Network drive',
    cd: 'CD drive',
    ram: 'RAM disk',
  }

  function usedShare(item: files.Drive): number {
    return item.total > 0 ? (item.total - item.free) / item.total : 0
  }
  let kind = $derived<Kind>(single ? kindOf(single) : 'thumb')
  let details = $state<files.Details | null>(null)
  let text = $state<files.TextPreview | null>(null)
  let size = $state<files.Size | null>(null)
  let sizing = $state(false)
  let sizeError = $state('')
  let mediaFailed = $state(false)
  let dimensions = $state('')
  let duration = $state('')
  let info = $state<media.Info | null>(null)
  let job = 0

  function kindOf(entry: Entry): Kind {
    if (entry.dir) return 'thumb'
    if (WEB_IMAGES.has(entry.ext)) return 'image'
    if (WEB_VIDEOS.has(entry.ext)) return 'video'
    if (WEB_AUDIO.has(entry.ext)) return 'audio'
    if (IMAGES.has(entry.ext) || VIDEOS.has(entry.ext)) return 'thumb'
    if (TEXT_FILES.has(entry.ext) || (!entry.ext && entry.size < 1024 * 1024)) return 'text'
    return 'thumb'
  }

  /** The item the details are about: the selection, or the folder itself. */
  let subject = $derived(single ? single.path : selected.length === 0 && !bin ? folder : '')

  /** The list reloads with new objects when a folder changes, so the preview
   *  follows this text instead and keeps playing when nothing it shows changed. */
  let previewKey = $derived(`${subject}|${single?.modified ?? ''}|${kind}`)

  $effect(() => {
    void previewKey
    untrack(load)
  })

  function load() {
    const path = subject
    const entry = single
    const currentKind = kind
    const id = ++job
    details = null
    text = null
    size = null
    sizing = false
    sizeError = ''
    mediaFailed = false
    dimensions = ''
    duration = ''
    info = null
    api.CancelFolderSize('preview')
    if (!path) return
    if (entry && !entry.dir && (VIDEOS.has(entry.ext) || AUDIO.has(entry.ext) || IMAGES.has(entry.ext))) {
      api
        .MediaInfo(entry.path, IMAGES.has(entry.ext))
        .then((value) => id === job && (info = value))
        .catch(() => {})
    }
    api
      .Describe(path)
      .then((value) => id === job && (details = value))
      .catch(() => {})
    if (entry && currentKind === 'text') {
      api
        .ReadText(entry.path)
        .then((value) => id === job && (text = value))
        .catch(() => {})
    }
  }

  async function calculateSize(path: string) {
    const id = job
    sizing = true
    sizeError = ''
    try {
      const value = await api.FolderSize('preview', path)
      if (id === job) size = value
    } catch (e) {
      if (id === job && errorText(e) !== 'Cancelled') sizeError = errorText(e)
    } finally {
      if (id === job) sizing = false
    }
  }

  // Items of the Recycle Bin know their size, folders included.
  let totalSize = $derived(selected.reduce((sum, entry) => sum + (entry.dir && !entry.original ? 0 : entry.size), 0))
  let folderCount = $derived(selected.filter((entry) => entry.dir).length)

  function typeName(entry: { dir: boolean; ext: string }): string {
    if (entry.dir) return 'File folder'
    return types[entry.ext] || (entry.ext ? `${entry.ext.slice(1).toUpperCase()} file` : 'File')
  }

  function attributes(value: files.Details): string {
    const list = []
    if (value.readOnly) list.push('Read-only')
    if (value.hidden) list.push('Hidden')
    if (value.system) list.push('System')
    if (value.archive) list.push('Archive')
    if (value.link) list.push('Link')
    return list.join(', ') || 'None'
  }

  function imageLoaded(event: Event) {
    const image = event.currentTarget as HTMLImageElement
    if (image.naturalWidth) dimensions = `${image.naturalWidth} × ${image.naturalHeight}`
  }

  function videoLoaded(event: Event) {
    const video = event.currentTarget as HTMLVideoElement
    if (video.videoWidth) dimensions = `${video.videoWidth} × ${video.videoHeight}`
    duration = formatDuration(video.duration)
  }

  function audioLoaded(event: Event) {
    duration = formatDuration((event.currentTarget as HTMLAudioElement).duration)
  }

  /** "29.97", "25", "23.976": frame rates with only the digits they need. */
  function formatFps(fps: number): string {
    return String(Math.round(fps * 1000) / 1000)
  }

  const CHANNEL_NAMES: Record<number, string> = { 1: 'mono', 2: 'stereo', 6: '5.1', 8: '7.1' }

  function channelText(audio: media.Audio): string {
    const layout = audio.layout || CHANNEL_NAMES[audio.channels]
    return layout ? `${audio.channels} (${layout})` : String(audio.channels)
  }

  function sampleRateText(rate: number): string {
    return rate ? `${(rate / 1000).toLocaleString('en-US', { maximumFractionDigits: 2 })} kHz` : ''
  }

  function streamName(kind: string, index: number, count: number, language: string, title: string): string {
    const parts = [count > 1 ? `${kind} ${index + 1}` : kind]
    const extra = [language.toUpperCase(), title].filter(Boolean).join(', ')
    return extra ? `${parts[0]} · ${extra}` : parts[0]
  }

  const TAG_LABELS: [string, string][] = [
    ['title', 'Title'],
    ['artist', 'Artist'],
    ['album', 'Album'],
    ['album_artist', 'Album artist'],
    ['track', 'Track'],
    ['year', 'Year'],
    ['genre', 'Genre'],
    ['composer', 'Composer'],
    ['comment', 'Comment'],
    ['encoder', 'Encoder'],
  ]

  function exposureText(seconds: number): string {
    if (!seconds) return ''
    return seconds >= 1 ? `${seconds} s` : `1/${Math.round(1 / seconds)} s`
  }

  function iconFallback(event: Event, entry: { path: string; dir: boolean; ext: string }) {
    const image = event.currentTarget as HTMLImageElement
    const fallback = iconUrl(entry, 256)
    if (!image.src.endsWith(fallback)) image.src = fallback
  }
</script>

<section class="preview">
  {#if drive}
    {@const share = usedShare(drive)}
    <div class="stage">
      {#if themed}
        <FileIcon entry={{ dir: true, ext: '' }} drive size={128} />
      {:else}
        <img class="icon-large" src={iconUrl({ path: drive.path, dir: true, ext: '' }, 256)} alt="" />
      {/if}
    </div>
    <h2>{driveLabel(drive.path)}</h2>
    {#if drive.total > 0}
      <div class="capacity" class:full={share > 0.9} title={`${Math.round(share * 100)}% used`}>
        <span style:width={`${share * 100}%`}></span>
      </div>
      <p class="capacity-text">{formatSize(drive.free)} free of {formatSize(drive.total)}</p>
    {/if}
    <dl>
      <dt>Type</dt>
      <dd>{DRIVE_KINDS[drive.kind] ?? 'Drive'}</dd>
      {#if drive.fileSystem}
        <dt>File system</dt>
        <dd>{drive.fileSystem}</dd>
      {/if}
      {#if drive.total > 0}
        <dt>Used</dt>
        <dd>{formatExactSize(drive.total - drive.free)} ({Math.round(share * 100)}%)</dd>
        <dt>Free</dt>
        <dd>{formatExactSize(drive.free)}</dd>
        <dt>Capacity</dt>
        <dd>{formatExactSize(drive.total)}</dd>
      {:else}
        <dt>State</dt>
        <dd>{drive.ready ? 'Ready' : 'Not ready'}</dd>
      {/if}
      {#if drive.serial}
        <dt>Serial number</dt>
        <dd>{drive.serial}</dd>
      {/if}
      <dt>Path</dt>
      <dd class="path">{drive.path}</dd>
    </dl>
  {:else if bin && selected.length === 0}
    <div class="stage">
      <span class="bin-icon" class:full={bin.items > 0}><Icon name={bin.items > 0 ? 'binFull' : 'bin'} size={112} /></span>
    </div>
    <h2>Recycle Bin</h2>
    <dl>
      <dt>Items</dt>
      <dd>{bin.items.toLocaleString('en-US')}</dd>
      <dt>Size</dt>
      <dd>{formatExactSize(bin.bytes)}</dd>
    </dl>
    <p class="note">Select items to restore them where they were, or delete them for good.</p>
  {:else if !folder && selected.length === 0}
    <h2>This PC</h2>
    <ul class="drives">
      {#each drives as item (item.path)}
        {@const share = usedShare(item)}
        <li>
          <span class="drive-name">{driveLabel(item.path)}</span>
          {#if item.total > 0}
            <div class="capacity small" class:full={share > 0.9}><span style:width={`${share * 100}%`}></span></div>
            <span class="sub">{formatSize(item.free)} free of {formatSize(item.total)}</span>
          {:else}
            <span class="sub">Not ready</span>
          {/if}
        </li>
      {/each}
    </ul>
  {:else if single}
    <div class="stage" class:checker={kind === 'image'}>
      {#key single.path + single.modified}
        {#if kind === 'image' && !mediaFailed}
          <img src={fileUrl(single.path)} alt={single.name} onload={imageLoaded} onerror={() => (mediaFailed = true)} />
        {:else if kind === 'video' && !mediaFailed}
          <!-- svelte-ignore a11y_media_has_caption -->
          <video
            src={fileUrl(single.path)}
            controls
            preload="metadata"
            onloadedmetadata={videoLoaded}
            onerror={() => (mediaFailed = true)}
          ></video>
        {:else if kind === 'audio' && !mediaFailed}
          <div class="audio">
            <img class="icon-large" src={thumbUrl(single.path, 256, single.modified)} alt="" />
            <audio src={fileUrl(single.path)} controls preload="metadata" onloadedmetadata={audioLoaded} onerror={() => (mediaFailed = true)}></audio>
          </div>
        {:else if kind === 'text' && text && !text.binary}
          <pre class="text">{text.text}{text.truncated ? '\n…' : ''}</pre>
        {:else if themed && single.dir}
          <FolderThumb folder={single} size={160} />
        {:else if themed && (single.dir || (!IMAGES.has(single.ext) && !VIDEOS.has(single.ext) && !hasOwnIcon(single) && single.ext !== '.pdf'))}
          <FileIcon entry={single} size={128} />
        {:else}
          <img
            class="shell-thumb"
            src={thumbUrl(single.path, 512, single.modified)}
            alt=""
            onerror={(event) => iconFallback(event, single)}
          />
        {/if}
      {/key}
    </div>
    <h2 title={single.name}>{single.name}</h2>
    <dl>
      <dt>Type</dt>
      <dd>{typeName(single)}</dd>
      {#if !single.dir || single.original}
        <dt>Size</dt>
        <dd>{formatExactSize(single.size)}</dd>
      {/if}
      {#if dimensions || info?.photo?.width}
        <dt>Dimensions</dt>
        <dd>{dimensions || `${info?.photo?.width} × ${info?.photo?.height}`}</dd>
      {/if}
      {#if duration || info?.duration}
        <dt>Length</dt>
        <dd>{duration || formatDuration(info?.duration ?? 0)}</dd>
      {/if}
      {#if single.dir && !single.original}
        {@render folderSize(single.path)}
      {/if}
      {#if single.deleted}
        <dt>Deleted</dt>
        <dd>{formatLongDate(single.deleted)}</dd>
      {/if}
      <dt>Modified</dt>
      <dd>{formatLongDate(single.modified)}</dd>
      <dt>Created</dt>
      <dd>{formatLongDate(single.created)}</dd>
      {#if details}
        <dt>Accessed</dt>
        <dd>{formatLongDate(details.accessed)}</dd>
        <dt>Attributes</dt>
        <dd>{attributes(details)}</dd>
      {/if}
      {#if single.original}
        <dt>Original location</dt>
        <dd class="path">{parentPath(single.original)}</dd>
      {:else}
        <dt>Location</dt>
        <dd class="path">{folder}</dd>
      {/if}
    </dl>
    {#if info}
      {@render mediaDetails(info)}
    {/if}
  {:else if selected.length > 1}
    <div class="stage stack">
      {#each selected.slice(0, 3) as entry, index (entry.path)}
        {#if themed && !hasOwnIcon(entry)}
          <span
            class="stacked"
            style:transform={`translate(${(index - 1) * 18}px, ${(index - 1) * -10}px) rotate(${(index - 1) * 6}deg)`}
          >
            <FileIcon {entry} size={112} />
          </span>
        {:else}
          <img
            src={iconUrl(entry, 256)}
            alt=""
            style:transform={`translate(${(index - 1) * 18}px, ${(index - 1) * -10}px) rotate(${(index - 1) * 6}deg)`}
          />
        {/if}
      {/each}
    </div>
    <h2>{selected.length} items selected</h2>
    <dl>
      {#if folderCount > 0}
        <dt>Folders</dt>
        <dd>{folderCount}</dd>
      {/if}
      {#if selected.length > folderCount || bin}
        <dt>Files</dt>
        <dd>{selected.length - folderCount}</dd>
        <dt>{bin ? 'Size' : 'Size of files'}</dt>
        <dd>{formatExactSize(totalSize)}</dd>
      {/if}
      <dt>Location</dt>
      <dd class="path">{bin ? 'Recycle Bin' : folder}</dd>
    </dl>
  {:else if folder}
    <div class="stage">
      {#if themed}
        <FileIcon entry={{ dir: true, ext: '' }} size={128} />
      {:else}
        <img class="icon-large" src={iconUrl({ path: folder, dir: true, ext: '' }, 256)} alt="" />
      {/if}
    </div>
    <h2 title={folder}>{folderLabel || baseName(folder)}</h2>
    <dl>
      <dt>Items</dt>
      <dd>{count}</dd>
      {@render folderSize(folder)}
      {#if details}
        <dt>Modified</dt>
        <dd>{formatLongDate(details.modified)}</dd>
        <dt>Created</dt>
        <dd>{formatLongDate(details.created)}</dd>
      {/if}
      <dt>Path</dt>
      <dd class="path">{folder}</dd>
    </dl>
  {:else}
    <p class="empty">Select a file to see it here.</p>
  {/if}
</section>

{#snippet row(label: string, value: string | number | undefined | null)}
  {#if value}
    <dt>{label}</dt>
    <dd>{value}</dd>
  {/if}
{/snippet}

{#snippet mediaDetails(value: media.Info)}
  {#if value.format || value.bitrate}
    <h3>File</h3>
    <dl>
      {@render row('Container', value.format)}
      {@render row('Bitrate', formatBitrate(value.bitrate))}
    </dl>
  {/if}
  {#each value.video ?? [] as video, index}
    <h3>{streamName('Video', index, value.video.length, video.language, video.title)}</h3>
    <dl>
      {@render row('Codec', [video.codec, video.profile].filter(Boolean).join(', '))}
      {@render row('Resolution', video.width ? `${video.width} × ${video.height}` : '')}
      {@render row('Aspect ratio', video.aspect)}
      {@render row('Frame rate', video.fps ? `${formatFps(video.fps)} fps` : '')}
      {@render row('Bitrate', formatBitrate(video.bitrate))}
      {@render row('Bit depth', video.bitDepth ? `${video.bitDepth} bit` : '')}
      {@render row('Pixel format', video.pixelFormat)}
      {@render row('Dynamic range', video.hdr)}
      {@render row('Scan', video.scan)}
      {@render row('Rotation', video.rotation ? `${video.rotation}°` : '')}
    </dl>
  {/each}
  {#each value.audio ?? [] as audio, index}
    <h3>{streamName('Audio', index, value.audio.length, audio.language, audio.title)}</h3>
    <dl>
      {@render row('Codec', [audio.codec, audio.profile].filter(Boolean).join(', '))}
      {@render row('Channels', audio.channels ? channelText(audio) : '')}
      {@render row('Sample rate', sampleRateText(audio.sampleRate))}
      {@render row('Bitrate', formatBitrate(audio.bitrate))}
      {@render row('Bit depth', audio.bitDepth ? `${audio.bitDepth} bit` : '')}
    </dl>
  {/each}
  {#if value.subtitles?.length}
    <h3>Subtitles</h3>
    <dl>
      {#each value.subtitles as subtitle, index}
        {@render row(
          `${index + 1}`,
          [subtitle.language.toUpperCase(), subtitle.title, subtitle.codec].filter(Boolean).join(' · '),
        )}
      {/each}
    </dl>
  {/if}
  {#if value.photo && (value.photo.camera || value.photo.taken || value.photo.bitDepth)}
    <h3>Photo</h3>
    <dl>
      {@render row('Camera', value.photo.camera)}
      {@render row('Lens', value.photo.lens)}
      {@render row('Taken', formatLongDate(value.photo.taken))}
      {@render row('Exposure', exposureText(value.photo.exposure))}
      {@render row('Aperture', value.photo.fNumber ? `f/${Math.round(value.photo.fNumber * 10) / 10}` : '')}
      {@render row('ISO', value.photo.iso)}
      {@render row('Focal length', value.photo.focalLength ? `${Math.round(value.photo.focalLength)} mm` : '')}
      {@render row('Flash', value.photo.camera ? (value.photo.flash ? 'Fired' : 'Did not fire') : '')}
      {@render row('Bit depth', value.photo.bitDepth ? `${value.photo.bitDepth} bit` : '')}
      {@render row('Resolution', value.photo.dpi ? `${Math.round(value.photo.dpi)} dpi` : '')}
    </dl>
  {/if}
  {#if TAG_LABELS.some(([key]) => value.tags?.[key])}
    <h3>Tags</h3>
    <dl>
      {#each TAG_LABELS as [key, label] (key)}
        {@render row(label, value.tags?.[key])}
      {/each}
    </dl>
  {/if}
  {#if value.source === 'windows' && (value.video?.length || value.audio?.length)}
    <p class="note">From Windows. Install ffprobe (part of FFmpeg) for every codec and stream.</p>
  {/if}
{/snippet}

{#snippet folderSize(path: string)}
  <dt>Size</dt>
  <dd>
    {#if size}
      {formatExactSize(size.bytes)}
      <span class="sub">
        {size.files.toLocaleString('en-US')} files, {size.folders.toLocaleString('en-US')} folders{size.unreadable
          ? `, ${size.unreadable} unreadable`
          : ''}
      </span>
    {:else if sizes?.[path] !== undefined && !sizing}
      {formatExactSize(sizes[path])}
      <button type="button" class="link" onclick={() => calculateSize(path)}>Count files</button>
    {:else if sizing}
      <span class="sub">Calculating…</span>
      <button type="button" class="link" onclick={() => api.CancelFolderSize('preview')}>Stop</button>
    {:else}
      <button type="button" class="link" onclick={() => calculateSize(path)}>Calculate</button>
      {#if sizeError}<span class="error">{sizeError}</span>{/if}
    {/if}
  </dd>
{/snippet}

<style>
  .preview {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 14px;
    overflow-y: auto;
    background: var(--surface);
    user-select: text;
  }

  .stage {
    position: relative;
    display: flex;
    flex: none;
    align-items: center;
    justify-content: center;
    height: min(46vh, 420px);
    min-height: 160px;
    margin-bottom: 14px;
    overflow: hidden;
    border-radius: var(--radius);
    background: var(--bg);
  }

  .stage.checker {
    background:
      repeating-conic-gradient(var(--checker-a) 0% 25%, var(--checker-b) 0% 50%) 50% / 20px 20px;
  }

  .stage img,
  .stage video {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }

  .stage video {
    width: 100%;
    background: #000;
  }

  .stack img {
    position: absolute;
    width: 45%;
    max-width: 128px;
    filter: drop-shadow(0 4px 10px rgb(0 0 0 / 45%));
  }

  .stacked {
    position: absolute;
    display: flex;
  }

  .icon-large {
    width: 45%;
    max-width: 128px;
  }

  .shell-thumb {
    min-width: 96px;
  }

  .audio {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    width: 100%;
    padding: 12px;
  }

  .audio audio {
    width: 100%;
  }

  .text {
    align-self: stretch;
    width: 100%;
    margin: 0;
    padding: 10px;
    overflow: auto;
    color: var(--text);
    font: 12px/1.5 Consolas, 'Cascadia Mono', monospace;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  h2 {
    margin-bottom: 10px;
    font-size: 15px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 6px 12px;
    margin: 0;
    font-size: 13px;
  }

  dt {
    color: var(--text-muted);
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
  }

  .path {
    font-size: 12px;
  }

  h3 {
    margin: 16px 0 8px;
    padding-top: 10px;
    border-top: 1px solid var(--border);
    color: var(--accent);
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.02em;
  }

  .bin-icon {
    display: flex;
    color: var(--text-muted);
  }

  .bin-icon.full {
    color: var(--accent);
    filter: drop-shadow(0 0 8px var(--accent-soft));
  }

  .note {
    margin-top: 14px;
    color: var(--text-muted);
    font-size: 11px;
  }

  .sub {
    display: block;
    color: var(--text-muted);
    font-size: 12px;
  }

  .link {
    height: auto;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--accent);
    font-size: 13px;
  }

  .link:hover {
    text-decoration: underline;
  }

  .capacity {
    flex: none;
    height: 10px;
    margin-bottom: 6px;
    overflow: hidden;
    border-radius: 5px;
    background: var(--surface-raised);
  }

  .capacity.small {
    height: 6px;
    margin: 4px 0;
  }

  .capacity span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, var(--accent), var(--cyan));
  }

  .capacity.full span {
    background: var(--danger);
  }

  .capacity-text {
    margin-bottom: 12px;
    color: var(--text-muted);
    font-size: 13px;
  }

  .drives {
    display: flex;
    flex-direction: column;
    gap: 14px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .drive-name {
    font-size: 13px;
  }

  .empty {
    margin-top: 40px;
    color: var(--text-muted);
    text-align: center;
  }
</style>
