import { nextTick } from 'vue'

export async function revealField(id: string) {
  await nextTick()
  const node = document.getElementById(id)
  const target = node?.matches('input,textarea,button,[tabindex]') ? node : node?.querySelector<HTMLElement>('input:not(:disabled),textarea,button:not(:disabled),[tabindex]')
  target?.focus({ preventScroll: true })
  node?.scrollIntoView({ block: 'center', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
}
