<script setup lang="ts">
defineProps<{
  name: string
  size?: number
}>()
const additions: Record<string, string> = {
  back: 'm15 5-7 7 7 7',
  code: 'm7 7-5 5 5 5m10-10 5 5-5 5M14 4l-4 16',
  heart:
    'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
  chat: 'M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-2 2v-10A8.5 8.5 0 0 1 10.5 3h2a8.5 8.5 0 0 1 8.5 8.5ZM7 11h.01M12 11h.01M17 11h.01',
  external: 'M14 3h7v7m0-7L10 14M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5',
  download: 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
  copy: 'M9 9h12v12H9ZM15 5V3H3v12h2',
  scan: 'M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M21 16v3a2 2 0 0 1-2 2h-3M8 21H5a2 2 0 0 1-2-2v-3M3 12h18',
  chevron: 'm9 5 7 7-7 7',
  activity:
    'M3 12h4l2-5 4 10 2-5h6M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2',
  water: 'M12 3s-7 7-7 12a7 7 0 0 0 14 0c0-5-7-12-7-12ZM8 15a4 4 0 0 0 3 4',
  shower: 'M5 9a7 7 0 0 1 14 0M3 9h18M12 2v3M5 13v1m7-1v1m7-1v1M4 18v1m8-1v1m8-1v1',
  play: 'M7 5.072 19 12 7 18.928Z',
  stop: 'M6 6h12v12H6Z',
  ticket: 'M3 5h18v5a2 2 0 0 0 0 4v5H3v-5a2 2 0 0 0 0-4ZM15 5v2m0 4v2m0 4v2',
}
const paths: Record<string, string> = {
  trash: 'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7m4-7v7',
  book: 'M12 5C9 3 5 3 3 4v15c3-1 6-1 9 1 3-2 6-2 9-1V4c-2-1-6-1-9 1Zm0 0v15',
  clock: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0ZM12 6v6l4 2',
  home: 'M3 10 12 3l9 7v11h-6v-7H9v7H3Z',
  user: 'M20 21v-2a6 6 0 0 0-6-6h-4a6 6 0 0 0-6 6v2M16 6a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
  settings:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M9 3h6l1 3 3 1 2 5-2 5-3 1-1 3H9l-1-3-3-1-2-5 2-5 3-1Z',
  lock: 'M6 10V7a6 6 0 0 1 12 0v3M5 10h14v11H5ZM12 14v3',
  shield: 'M12 2 3 6v6c0 5 9 10 9 10s9-5 9-10V6ZM8 12l3 3 5-6',
  arrow: 'M4 12h15m-6-6 6 6-6 6',
  refresh: 'M20 7v5h-5M4 17v-5h5M6 6a8 8 0 0 1 14 6M18 18a8 8 0 0 1-14-6',
  phone: 'M7 2h10v20H7Zm3 17h4',
  check: 'm5 12 4 4L19 6',
}
</script>

<template>
  <svg
    :width="size ?? 22"
    :height="size ?? 22"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path
      v-if="name === 'wechat'"
      fill="currentColor"
      stroke="none"
      d="M9.5 3C4.8 3 1 6 1 9.7c0 2 1 3.7 2.8 4.9l-.6 2.2 2.6-1.3c1.1.4 2.3.7 3.7.7h.6a6.7 6.7 0 0 1-.8-3.2c0-3.6 3.4-6.6 7.6-6.6h.2C15.8 4.4 12.9 3 9.5 3Zm-3 4h1v1h-1Zm6 0h1v1h-1ZM17 8c-3.3 0-6 2.3-6 5s2.7 5 6 5c.9 0 1.7-.1 2.5-.4l2 1-.5-1.7c1.3-.9 2-2.3 2-3.9 0-2.7-2.7-5-6-5Zm-2 3h1v1h-1Zm4 0h1v1h-1Z"
    />
    <path
      v-else
      :d="additions[name] ?? paths[name] ?? paths.home"
      :fill="name === 'play' || name === 'stop' ? 'currentColor' : 'none'"
    />
  </svg>
</template>
