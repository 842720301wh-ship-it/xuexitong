<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import UiIcon from '../components/UiIcon.vue'
import { androidHost, backgroundStatus, refreshBackground } from '../stores/background'
import { state, maskAccount, api, act } from '../stores/settings'
import { tasks, tasksLoading, tasksError, loadTasks, taskIssue, type Task } from '../stores/tasks'
import { latestProgress, progressLabel, progressText, progressBrief, progressClock, watchProgress } from '../stores/progress'
const emit = defineEmits<{ create: []; edit: [task: Task]; records: [] }>()
function accountLabel(task: Task) { const account = state.accounts.find(a => a.id === task.account_id); return account ? maskAccount(account.username) : '账号已移除' }
function courseLabel(task: Task) { const names = task.courses.map(c => c.name?.trim() || '课程名称待获取'); return names.length > 1 ? `${names[0]} 等 ${names.length} 门课程` : names[0] || '未选择课程' }
const actionsOpen = ref(false), actionTask = ref<Task>()
let holdTimer: ReturnType<typeof setTimeout> | undefined
let pressX = 0, pressY = 0
let serviceTimer: ReturnType<typeof setInterval> | undefined
let stopProgress: (() => void) | undefined
const runningCount = computed(() => tasks.value.filter(task => ['starting', 'running', 'retrying'].includes(latestProgress.value[task.id]?.state || '')).length)
function taskRunning(task: Task) { return ['starting', 'running', 'retrying'].includes(latestProgress.value[task.id]?.state || '') }
function cancelHold() { if (holdTimer !== undefined) { clearTimeout(holdTimer); holdTimer = undefined } }
function openActions(task: Task) { cancelHold(); if (state.busy) return; actionTask.value = task; actionsOpen.value = true }
function startHold(event: PointerEvent, task: Task) {
  cancelHold()
  if (event.button !== 0 || state.busy || (event.target as HTMLElement).closest('button, a, input')) return
  pressX = event.clientX; pressY = event.clientY
  holdTimer = setTimeout(() => openActions(task), 500)
}
function moveHold(event: PointerEvent) { if (Math.hypot(event.clientX - pressX, event.clientY - pressY) > 10) cancelHold() }
function keyboardActions(event: KeyboardEvent, task: Task) { if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10') || event.key === 'Enter') { event.preventDefault(); openActions(task) } }
function editSelected() { const task = actionTask.value; actionsOpen.value = false; if (task) emit('edit', task) }
function removeSelected() { const task = actionTask.value; actionsOpen.value = false; if (task) void remove(task) }
function clearClosedAction() { if (!actionsOpen.value) actionTask.value = undefined }
async function remove(task: Task) {
  try { await ElMessageBox.confirm(`删除“${task.name}”的任务配置？账号和模型配置会保留。`, '删除任务', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }) } catch { return }
  await act(async () => { await api(`/tasks/${task.id}`, 'DELETE'); tasks.value = tasks.value.filter(t => t.id !== task.id); ElMessage.success('任务已删除') })
}
async function toggleRun(task: Task) {
  const host = androidHost()
  if (!host) { ElMessage.info('请在 Android 应用中启动，任务将在手机本地后台执行'); return }
  if (taskRunning(task)) { host.stopTask(task.id); ElMessage.info('正在停止，请等待当前请求结束') }
  else {
    if (taskIssue(task)) { ElMessage.warning(taskIssue(task)); return }
    refreshBackground()
    if (backgroundStatus.running) { ElMessage.warning('已有任务在执行，请先停止当前任务'); return }
    host.startTask(task.id); ElMessage.info('正在启动手机后台服务…')
  }
}
onMounted(() => { refreshBackground(); serviceTimer = setInterval(refreshBackground, 2000); void loadTasks(); stopProgress = watchProgress(); window.addEventListener('scroll', cancelHold, { capture: true, passive: true }) })
onUnmounted(() => { if (serviceTimer) clearInterval(serviceTimer); cancelHold(); stopProgress?.(); window.removeEventListener('scroll', cancelHold, true) })
</script>
<template>
  <section class="home-page learning-home">
    <div class="learning-summary"><div class="task-panel-heading"><h2><UiIcon name="book" :size="19" />学习概览</h2></div><div class="learning-stats"><div><span>进行中</span><strong>{{ runningCount }}<small>项</small></strong></div><div><span>已保存</span><strong>{{ tasks.length }}<small>项</small></strong></div></div><div class="learning-service"><i />{{ runningCount ? '手机后台服务运行中' : backgroundStatus.available ? '手机后台执行已就绪' : '请在手机端启动后台任务' }}<button @click="emit('records')">查看<UiIcon name="chevron" :size="12" /></button></div></div>
    <div class="task-section-heading"><h2>学习任务</h2><button class="task-text-button" :disabled="state.loading || !!state.error" @click="emit('create')">新建任务 ＋</button></div>
    <p v-if="tasks.length" class="learning-manage-hint">长按卡片可编辑或删除任务</p>
    <div v-if="tasksError" class="task-error" role="alert">{{ tasksError }}<button class="inline-link" @click="loadTasks">重试</button></div>
    <div v-else-if="tasksLoading" class="task-hint" role="status">正在读取任务…</div>
    <div v-else-if="!tasks.length" class="task-empty"><UiIcon name="home" :size="32" /><strong>创建你的第一个学习任务</strong><p>选择已保存的账号、课程与 AI 模型配置。</p><button class="primary-button" :disabled="state.loading || !!state.error" @click="emit('create')">新建任务</button></div>
    <article v-for="task in tasks" :key="task.id" class="saved-task-card learning-task-card" tabindex="0" aria-haspopup="dialog" :aria-label="task.name + '，长按管理任务'" @pointerdown="startHold($event, task)" @pointermove="moveHold" @pointerup="cancelHold" @pointercancel="cancelHold" @pointerleave="cancelHold" @contextmenu.prevent="openActions(task)" @keydown="keyboardActions($event, task)"><div class="learning-task-content"><div class="saved-task-status">{{ taskIssue(task) ? '配置待修复' : progressLabel(task.id) }}</div><h3 :title="task.name">{{ task.name }}</h3><p class="learning-course-name" :title="task.courses.map(c => c.name || '课程名称待获取').join('、')">{{ latestProgress[task.id]?.course_name || courseLabel(task) }}</p><p>学习账号　{{ accountLabel(task) }}</p><div class="learning-task-meta">{{ state.models.find(m => m.id === task.model_config_id)?.name || '模型配置已移除' }} · 自动重试 {{ task.retries }} 次</div><p class="learning-latest-progress" :title="progressText(task.id)" aria-live="polite" aria-atomic="true"><UiIcon name="clock" :size="12" /><span>{{ progressBrief(task.id) }}</span><time v-if="progressClock(task.id)">{{ progressClock(task.id) }}</time></p><p v-if="taskIssue(task)" class="task-error">{{ taskIssue(task) }}</p></div><div class="learning-task-control"><button class="learning-round-action" :disabled="state.busy || (!!taskIssue(task) && !taskRunning(task))" @click.stop="toggleRun(task)" :aria-label="(taskRunning(task) ? '停止' : '启动') + task.name" :title="taskRunning(task) ? '停止手机后台任务' : '启动手机后台任务'"><UiIcon :name="taskRunning(task) ? 'stop' : 'play'" :size="25" /></button><span>{{ taskRunning(task) ? '停止' : '启动' }}</span></div></article>
    <button class="learning-history-link" @click="emit('records')"><UiIcon name="clock" :size="15" />查看运行记录<UiIcon name="arrow" :size="14" /></button>
    <el-drawer v-model="actionsOpen" title="任务操作" direction="btt" size="auto" append-to-body modal-class="task-picker-mask" class="bottom-sheet task-actions-sheet" @closed="clearClosedAction"><template v-if="actionTask"><div class="task-action-summary"><strong>{{ actionTask.name }}</strong><p>{{ actionTask.courses.map(c => c.name || '课程名称待获取').join('、') }}</p></div><button class="task-action-row" :disabled="state.busy" @click="editSelected"><UiIcon name="settings" :size="19" />编辑任务<UiIcon name="chevron" :size="16" /></button><button class="task-action-row danger" :disabled="state.busy" @click="removeSelected"><UiIcon name="trash" :size="19" />删除任务<UiIcon name="chevron" :size="16" /></button><button class="outline-button full-width task-action-cancel" @click="actionsOpen = false">取消</button></template></el-drawer>
  </section>
</template>




