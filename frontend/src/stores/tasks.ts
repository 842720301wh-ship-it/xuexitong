import { reactive, ref } from 'vue'
import { api, state } from './settings'

export interface TaskCourse { id: string; name: string }
export interface Task {
  id: string
  name: string
  account_id: string
  model_config_id: string
  courses: TaskCourse[]
  ai_retries: number
  retries: number
  retry_delay: number
  speed: number
  jobs: number
  sequential: boolean
  notopen_action: 'retry' | 'skip' | 'stop'
  auto_submit: boolean
  created_at?: string
  updated_at?: string
}
export const tasks = ref<Task[]>([])
export const tasksLoading = ref(false)
export const tasksError = ref('')
export const draftReady = ref(false)
export const draftStep = ref(0)
export const blankTask = (): Task => ({
  id: '', name: '', account_id: state.default_account_id, model_config_id: '', courses: [],
  ai_retries: 3, retries: 2, retry_delay: 60, speed: 1, jobs: 1,
  sequential: true, notopen_action: 'retry', auto_submit: true,
})
export const taskDraft = reactive<Task>(blankTask())
export function beginTask(task?: Task) {
  Object.assign(taskDraft, task ? JSON.parse(JSON.stringify(task)) as Task : blankTask())
  draftStep.value = 0
  draftReady.value = true
}
export async function loadTasks() {
  tasksLoading.value = true
  try { tasks.value = await api<Task[]>('/tasks'); tasksError.value = '' }
  catch (e) { tasksError.value = e instanceof Error ? e.message : '任务列表读取失败' }
  finally { tasksLoading.value = false }
}
export async function saveTask() {
  const saved = await api<Task>(taskDraft.id ? `/tasks/${taskDraft.id}` : '/tasks', taskDraft.id ? 'PUT' : 'POST', taskDraft)
  const index = tasks.value.findIndex(t => t.id === saved.id)
  if (index < 0) tasks.value.push(saved)
  else tasks.value[index] = saved
  draftReady.value = false
  return saved
}
export function taskIssue(task: Task) {
  if (!state.accounts.some(a => a.id === task.account_id)) return '账号已移除，请重新选择'
  if (!state.models.some(m => m.id === task.model_config_id)) return '模型配置已移除，请重新选择'
  return ''
}
