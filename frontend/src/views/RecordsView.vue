<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import UiIcon from '../components/UiIcon.vue'
import { api } from '../stores/settings'
import type { TaskProgress } from '../stores/progress'
defineEmits<{home: []}>()
const records = ref<(TaskProgress & { task_id: string; task_name: string; run_id: string })[]>([]), error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined, stopped = false, request: AbortController | undefined
async function refresh() {
  if (request || document.hidden) return
  const controller = new AbortController(); request = controller
  try { const result = await api<typeof records.value>('/records', 'GET', undefined, controller.signal); if (!stopped) { records.value = result; error.value = '' } }
  catch (e) { if (!stopped) error.value = e instanceof Error ? e.message : '记录读取失败' }
  finally { request = undefined; if (!stopped) timer = setTimeout(refresh, 3000) }
}
function visibility() { if (timer) clearTimeout(timer); if (document.hidden) request?.abort(); else void refresh() }
onMounted(() => { void refresh(); document.addEventListener('visibilitychange', visibility) })
onUnmounted(() => { stopped = true; if (timer) clearTimeout(timer); request?.abort(); document.removeEventListener('visibilitychange', visibility) })
</script>
<template><section class="records-page"><div v-if="error" class="task-error" role="alert">{{ error }}</div><template v-if="records.length"><div class="records-heading"><h2>本次服务运行记录</h2><button class="task-text-button" @click="refresh">刷新</button></div><p class="task-hint">最新记录在前，保留最近 500 条；服务重启后清空。</p><article v-for="(record, index) in records" :key="index" class="record-event"><strong>{{ record.task_name }}</strong><p>{{ [record.course_name, record.chapter_name, record.message].filter(Boolean).join(' · ') }}</p><small>{{ record.updated_at?.replace('T', ' ').replace('Z', '') }}<template v-if="record.duration_seconds"> · {{ record.position_seconds }} / {{ record.duration_seconds }} 秒</template></small></article></template><div v-else class="empty-state"><UiIcon name="clock" :size="38" /><p>暂无运行记录</p><small>任务开始执行后，会在这里显示运行状态。</small><button class="outline-button" @click="$emit('home')">返回学习</button></div></section></template>
