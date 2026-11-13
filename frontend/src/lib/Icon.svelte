<script lang="ts" module>
  const PATHS = {
    plus: 'M12 5v14M5 12h14',
    minus: 'M5 12h14',
    close: 'M6 6l12 12M18 6L6 18',
    folder: 'M3 6h6l2 3h10v10H3z',
    file: 'M6 3h8l4 4v14H6zM14 3v4h4',
    newFolder: 'M3 6h6l2 3h10v10H3zM12 11.5v5M9.5 14h5',
    newFile: 'M6 3h8l4 4v14H6zM14 3v4h4M12 11v6M9 14h6',
    trash: 'M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5M14 11v5',
    bin: 'M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13',
    binFull: 'M4 9h16M6 9l1 11h10l1-11M7.5 9l2-5 3.5 2.5M12.5 9l3.5-5.5 2 3M10 12.5v5M14 12.5v5',
    terminal: 'M3 5h18v14H3zM7 9l3 3-3 3M13 15h4',
    clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5l3 2',
    reload: 'M20 11a8 8 0 0 0-14.3-4L3 10M3 4v6h6M4 13a8 8 0 0 0 14.3 4L21 14M21 20v-6h-6',
    back: 'M19 12H5M11 6l-6 6 6 6',
    forward: 'M5 12h14M13 6l6 6-6 6',
    up: 'M12 19V5M6 11l6-6 6 6',
    down: 'M12 5v14M6 13l6 6 6-6',
    chevron: 'M9 6l6 6-6 6',
    chevronDown: 'M6 9l6 6 6-6',
    list: 'M8 6h13M8 12h13M8 18h13M3.5 6h.01M3.5 12h.01M3.5 18h.01',
    grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
    sort: 'M3 6h11M3 12h8M3 18h5M18 5v14M15 16l3 3 3-3',
    drive: 'M3 13h18v6H3zM5 13l2.5-8h9L19 13M7 16h.01',
    desktop: 'M3 4h18v12H3zM8 20h8M12 16v4',
    download: 'M12 4v11M7 10l5 5 5-5M5 20h14',
    home: 'M3 11l9-8 9 8M5 9.5V21h14V9.5',
    image: 'M3 5h18v14H3zM3 16l5-5 4 4 3-3 6 6M15.5 9.5h.01',
    eye: 'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
    eyeOff:
      'M3 3l18 18M10.6 5.1A10 10 0 0 1 12 5c6.5 0 10 7 10 7a17 17 0 0 1-3.3 4.3M6.6 6.6C3.8 8.4 2 12 2 12s3.5 7 10 7c1.9 0 3.6-.6 5-1.4M9.9 9.9a3 3 0 0 0 4.2 4.2',
    panel: 'M3 4h18v16H3zM15 4v16',
    pin: 'M9 3h6l-1 7 4 3H6l4-3zM12 13v8',
    bookmark: 'M6 3h12v18l-6-4.5L6 21z',
    search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14zM21 21l-5-5',
    pc: 'M3 4h18v12H3zM8 20h8M12 16v4',
    icons: 'M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z',
    undo: 'M9 14L4 9l5-5M4 9h11a5 5 0 0 1 0 10h-3',
    redo: 'M15 14l5-5-5-5M20 9H9a5 5 0 0 0 0 10h3',
    split: 'M3 4h18v16H3zM12 4v16',
    info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v6M12 7.5h.01',
    edit: 'M4 20h4L19 9l-4-4L4 16zM13.5 6.5l4 4',
    palette: 'M12 3a9 9 0 0 0 0 18c1.1 0 1.5-.8 1.5-1.6 0-1.4-1-1.6-1-2.9 0-.9.7-1.5 1.6-1.5H17a4 4 0 0 0 4-4c0-4.4-4-8-9-8zM7.5 12h.01M9.5 7.5h.01M14.5 7.5h.01',
    settings:
      'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z',
  }

  export type IconName = keyof typeof PATHS
</script>

<script lang="ts">
  interface Props {
    name: IconName
    size?: number
  }

  let { name, size = 16 }: Props = $props()
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.8"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={PATHS[name]} />
</svg>
