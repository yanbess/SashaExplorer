/** Converts a rejected backend call into a message for the user. */
export function errorText(error: unknown): string {
  const message =
    typeof error === 'string' ? error : error instanceof Error ? error.message : ''
  if (!message) return 'Something went wrong'
  return message[0].toUpperCase() + message.slice(1)
}

/** The address of the "This PC" view, which lists the drives. */
export const THIS_PC = ''

/** The drop address of the Recycle Bin in the sidebar. It is no real path. */
export const RECYCLE_BIN = '::recycle-bin'

export function samePath(a: string, b: string): boolean {
  return trimSlash(a).toLowerCase() === trimSlash(b).toLowerCase()
}

function trimSlash(path: string): string {
  return isDriveRoot(path) ? path.slice(0, 2) : path.replace(/[\\/]+$/, '')
}

export function isDriveRoot(path: string): boolean {
  return /^[a-z]:\\?$/i.test(path)
}

/** The last part of a path, or the drive for a drive root. */
export function baseName(path: string): string {
  if (path === THIS_PC) return 'This PC'
  if (isDriveRoot(path)) return path.slice(0, 2)
  return path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || path
}

/**
 * A path with its middle left out when it is long, such as
 * "C:\Users\...\wp\sp": the start says where, the end says what.
 */
export function shortPath(path: string): string {
  const unc = path.startsWith('\\\\')
  const parts = path.replace(/^\\\\/, '').split('\\').filter(Boolean)
  const prefix = unc ? '\\\\' : ''
  if (parts.length <= 4) return path
  return `${prefix}${parts[0]}\\${parts[1]}\\...\\${parts.at(-2)}\\${parts.at(-1)}`
}

/** The parent folder, or THIS_PC above a drive. */
export function parentPath(path: string): string {
  if (path === THIS_PC || isDriveRoot(path)) return THIS_PC
  const trimmed = path.replace(/[\\/]+$/, '')
  const index = trimmed.lastIndexOf('\\')
  if (index < 0) return THIS_PC
  const parent = trimmed.slice(0, index)
  if (/^[a-z]:$/i.test(parent)) return parent + '\\'
  // \\server\share is the top of a network path.
  if (/^\\\\[^\\]+$/.test(parent) || parent === '\\') return THIS_PC
  return parent
}

/** Whether child is inside folder, at any depth. */
export function isInside(child: string, folder: string): boolean {
  const base = trimSlash(folder).toLowerCase() + '\\'
  return child.toLowerCase().startsWith(base)
}

export interface Crumb {
  label: string
  path: string
}

/** Splits a path into the steps of the address bar, from This PC down. */
export function crumbs(path: string): Crumb[] {
  if (path === RECYCLE_BIN) return [{ label: 'Recycle Bin', path: RECYCLE_BIN }]
  const list: Crumb[] = [{ label: 'This PC', path: THIS_PC }]
  if (path === THIS_PC) return list
  const unc = path.match(/^(\\\\[^\\]+\\[^\\]+)(.*)$/)
  let current: string
  let rest: string
  if (unc) {
    current = unc[1]
    rest = unc[2]
    list.push({ label: current, path: current })
  } else {
    current = path.slice(0, 2) + '\\'
    rest = path.slice(3)
    list.push({ label: path.slice(0, 2), path: current })
  }
  for (const part of rest.split('\\').filter(Boolean)) {
    current = current.endsWith('\\') ? current + part : current + '\\' + part
    list.push({ label: part, path: current })
  }
  return list
}

export function joinPath(folder: string, name: string): string {
  return folder.endsWith('\\') ? folder + name : folder + '\\' + name
}

const UNITS = ['bytes', 'KB', 'MB', 'GB', 'TB', 'PB']

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} ${bytes === 1 ? 'byte' : 'bytes'}`
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024
    unit++
  }
  const digits = value < 10 ? 1 : 0
  return `${value.toFixed(digits)} ${UNITS[unit]}`
}

export function formatExactSize(bytes: number): string {
  return `${formatSize(bytes)} (${bytes.toLocaleString('en-US')} bytes)`
}

const dateFormat = new Intl.DateTimeFormat('en-GB', { dateStyle: 'short', timeStyle: 'short' })
const longDateFormat = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'medium' })

export function formatDate(millis: number): string {
  return millis > 0 ? dateFormat.format(new Date(millis)) : ''
}

export function formatLongDate(millis: number): string {
  return millis > 0 ? longDateFormat.format(new Date(millis)) : ''
}

export function formatDuration(seconds: number): string {
  if (!isFinite(seconds) || seconds <= 0) return ''
  const total = Math.round(seconds)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`
}

/** Images the window can show itself. Others get the Windows thumbnail. */
export const WEB_IMAGES = new Set(['.jpg', '.jpeg', '.jfif', '.png', '.gif', '.webp', '.bmp', '.ico', '.svg', '.avif'])
export const WEB_VIDEOS = new Set(['.mp4', '.m4v', '.webm', '.mov', '.ogv', '.mkv'])
export const WEB_AUDIO = new Set(['.mp3', '.m4a', '.aac', '.wav', '.ogg', '.oga', '.opus', '.flac'])
export const IMAGES = new Set([
  ...WEB_IMAGES,
  '.tif', '.tiff', '.heic', '.heif', '.raw', '.cr2', '.cr3', '.nef', '.arw', '.dng', '.psd', '.jxl',
])
export const VIDEOS = new Set([...WEB_VIDEOS, '.avi', '.wmv', '.flv', '.mpg', '.mpeg', '.3gp', '.ts', '.m2ts'])
export const AUDIO = new Set([...WEB_AUDIO, '.wma', '.aiff', '.aif', '.ape', '.wv', '.mka', '.ac3', '.dts', '.amr', '.mid', '.midi'])

/** Bits per second as people read them: "320 kb/s", "12.4 Mb/s". */
export function formatBitrate(bps: number): string {
  if (!bps) return ''
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(bps >= 10_000_000 ? 0 : 1)} Mb/s`
  return `${Math.round(bps / 1000)} kb/s`
}

/** Files that are worth a text preview even without a known type. */
export const TEXT_FILES = new Set([
  '.txt', '.md', '.markdown', '.log', '.csv', '.tsv', '.json', '.jsonc', '.xml', '.yaml', '.yml', '.toml',
  '.ini', '.cfg', '.conf', '.env', '.properties', '.gitignore', '.gitattributes', '.editorconfig',
  '.js', '.mjs', '.cjs', '.ts', '.tsx', '.jsx', '.svelte', '.vue', '.css', '.scss', '.less', '.html', '.htm',
  '.go', '.mod', '.sum', '.py', '.rb', '.php', '.java', '.kt', '.kts', '.c', '.h', '.cpp', '.hpp', '.cc', '.cs',
  '.rs', '.swift', '.sh', '.bash', '.zsh', '.ps1', '.psm1', '.bat', '.cmd', '.sql', '.lua', '.pl', '.r',
  '.dockerfile', '.gradle', '.lock', '.srt', '.vtt', '.reg', '.nfo', '.tex', '.rst', '.adoc',
])

/** Address of a file served to the page, for pictures and players. */
export function fileUrl(path: string): string {
  return `/_sasha/file?p=${encodeURIComponent(path)}`
}

/** Changing it gives thumbnails new addresses, so the window drops ones it
 *  kept before. Addresses without it may hold icons that stood in for
 *  thumbnails Windows failed to make. */
const THUMB_VERSION = 2

export function thumbUrl(path: string, size: number, modified: number): string {
  return `/_sasha/thumb?p=${encodeURIComponent(path)}&s=${size}&m=${modified}&v=${THUMB_VERSION}`
}

/** Types whose files each have their own icon. Others share one per type. */
const OWN_ICONS = new Set(['.exe', '.lnk', '.ico', '.url', '.msi', '.cur', '.ani', '.appref-ms', '.scr', '.cpl'])

/** Programs and shortcuts are known by their own icons, so they keep them. */
export function hasOwnIcon(entry: { dir: boolean; ext: string }): boolean {
  return !entry.dir && OWN_ICONS.has(entry.ext)
}

/** Files whose picture is their content, so a thumbnail beats any icon. */
export function hasPicture(entry: { dir: boolean; ext: string }): boolean {
  return !entry.dir && (IMAGES.has(entry.ext) || VIDEOS.has(entry.ext))
}
const iconUrls = new Map<string, string>()

/**
 * Address of the Windows icon of an item. Items of one type share an address, so
 * the window loads that icon once.
 */
export function iconUrl(entry: { path: string; dir: boolean; ext: string; modified?: number }, size = 32): string {
  let key = ''
  if (entry.dir) key = isDriveRoot(entry.path) ? '' : 'folder'
  else if (!OWN_ICONS.has(entry.ext)) key = entry.ext || 'none'
  const own = `/_sasha/icon?p=${encodeURIComponent(entry.path)}&s=${size}&m=${entry.modified ?? 0}`
  if (!key) return own
  const cacheKey = `${key}|${size}`
  let url = iconUrls.get(cacheKey)
  if (!url) {
    url = own + `&k=${encodeURIComponent(key)}`
    iconUrls.set(cacheKey, url)
  }
  return url
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

export function compareNames(a: string, b: string): number {
  return collator.compare(a, b)
}

/** A name that is not taken yet, like "New folder (2)". */
export function uniqueName(base: string, ext: string, taken: Iterable<string>): string {
  const names = new Set([...taken].map((name) => name.toLowerCase()))
  if (!names.has((base + ext).toLowerCase())) return base + ext
  for (let i = 2; ; i++) {
    const candidate = `${base} (${i})${ext}`
    if (!names.has(candidate.toLowerCase())) return candidate
  }
}

/** Svelte action that focuses an element when it is mounted. */
export function focusOnMount(element: HTMLElement): void {
  element.focus()
}
