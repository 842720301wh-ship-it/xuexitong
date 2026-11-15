import { ref } from 'vue'
import { api } from './settings'
export interface TaskProgress {
  state: 'starting' | 'running' | 'queued' | 'retrying' | 'paused' | 'completed' | 'failed' | 'cancelled'
  course_name?: string
  chapter_name?: string
  position_seconds?: number
  duration_seconds?: number
  message?: string
  updated_at?: string
}
export const latestProgress = ref<Record<string, TaskProgress>>({})
const labels: Record<TaskProgress['state'], string> = { starting: '正在启动', running: '学习中', queued: '等待执行', retrying: '等待重试', paused: '已暂停', completed: '已完成', failed: '运行失败', cancelled: '已停止' }
function mediaTime(value: number) { const seconds = Math.max(0, Math.floor(value)); const h = Math.floor(seconds / 3600), m = Math.floor(seconds / 60) % 60, s = seconds % 60; return (h ? String(h).padStart(2, '0') + ':' : '') + String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0') }
export function progressLabel(id: string) { const event = latestProgress.value[id]; return event ? labels[event.state] || '状态待确认' : '待运行' }
export function progressBrief(id: string) { const event = latestProgress.value[id]; if (!event) return '等待启动'; return [labels[event.state], event.chapter_name, event.message?.split(/[\r\n]+/).map(s => s.trim()).filter(Boolean).pop()].filter(Boolean).join(' · ') }
export function progressClock(id: string) { const event = latestProgress.value[id]; if (!event) return ''; return event.duration_seconds && event.duration_seconds > 0 ? `${mediaTime(event.position_seconds || 0)} / ${mediaTime(event.duration_seconds)}` : event.position_seconds && event.position_seconds > 0 ? mediaTime(event.position_seconds) : '' }
export function progressText(id: string) {
  const event = latestProgress.value[id]
  if (!event) return '等待启动'
  const message = event.message?.split(/[\r\n]+/).map(s => s.trim()).filter(Boolean).pop()
  const parts = [labels[event.state] || '状态待确认', event.course_name, event.chapter_name]
  if (message && !parts.includes(message)) parts.push(message)
  if (typeof event.duration_seconds === 'number' && event.duration_seconds > 0) parts.push(`${mediaTime(event.position_seconds || 0)} / ${mediaTime(event.duration_seconds)}`)
  return parts.filter(Boolean).join(' · ')
}
// Poll only while this view is visible; requests never overlap or block UI actions.
export function watchProgress() {
  let stopped = false, timer: ReturnType<typeof setTimeout> | undefined, current: AbortController | undefined
  async function poll() {
    if (stopped || document.hidden || current) return
    const controller = new AbortController(); current = controller
    const timeout = setTimeout(() => controller.abort(), 10000)
    try { const snapshot = await api<Record<string, TaskProgress>>('/task-progress', 'GET', undefined, controller.signal); if (!stopped && current === controller) latestProgress.value = snapshot }
    catch { /* Preserve the last known event during temporary transport errors. */ }
    finally { clearTimeout(timeout); if (current === controller) current = undefined; if (!stopped && !document.hidden) timer = setTimeout(poll, 2000) }
  }
  function visibility() { if (timer) clearTimeout(timer); if (document.hidden) current?.abort(); else void poll() }
  document.addEventListener('visibilitychange', visibility)
  void poll()
  return () => { stopped = true; if (timer) clearTimeout(timer); current?.abort(); document.removeEventListener('visibilitychange', visibility) }
}
