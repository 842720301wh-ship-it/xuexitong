<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import UiIcon from '../components/UiIcon.vue'
import TaskPickerSheet from '../components/TaskPickerSheet.vue'
import { revealField } from '../composables/fieldValidation'
import BackgroundSettings from '../components/BackgroundSettings.vue'
import { api, state, maskAccount, act } from '../stores/settings'
import { beginTask, draftReady, draftStep as step, saveTask, taskDraft as draft } from '../stores/tasks'

const emit = defineEmits<{ accounts: []; models: []; saved: [] }>()
interface Course { id: string; name: string; teacher?: string; unavailable?: boolean }
const courses = ref<Course[]>([])
const coursesLoading = ref(false), coursesError = ref(''), coursesLoaded = ref(false)
const search = ref(''), courseMode = ref<'list' | 'manual'>('list'), manualIDs = ref(draft.courses.map(c => c.id).join(', '))
let request: AbortController | undefined
const selectedAccount = computed(() => state.accounts.find(a => a.id === draft.account_id))
const selectedModel = computed(() => state.models.find(m => m.id === draft.model_config_id))
const accountPicker = ref(false), modelPicker = ref(false)
const accountOptions = computed(() => state.accounts.map(a => ({ id: a.id, title: a.name || '姓名未获取', subtitle: maskAccount(a.username), badge: a.id === state.default_account_id ? '默认账号' : '' })))
const modelOptions = computed(() => state.models.map(m => ({ id: m.id, title: m.name, subtitle: m.has_key ? m.primary + (m.fallback ? ' · 备用 ' + m.fallback : '') : '缺少 API Key，请先完善配置', disabled: !m.has_key })))
const availableCourses = computed<Course[]>(() => {
  const all: Course[] = [...courses.value, ...draft.courses.filter(c => !courses.value.some(o => o.id === c.id))]
  const query = search.value.trim().toLowerCase()
  return all.filter(c => !query || `${c.name} ${c.id}`.toLowerCase().includes(query))
})
const manualError = computed(() => courseMode.value === 'manual' && manualIDs.value.split(/[,，\s]+/).filter(Boolean).some(id => !/^\d{1,30}$/.test(id)) ? '课程 ID 只支持数字，多个 ID 请用逗号或空格分隔' : '')
const attempted = ref([false, false, false])
const invalid = computed<Record<string, string>[]>(() => {
  const first: Record<string, string> = {}, second: Record<string, string> = {}, third: Record<string, string> = {}
  if (!draft.name.trim()) first['task-name'] = '请填写任务名称'
  if (!selectedAccount.value) first['task-account'] = '请选择学习账号；还没有账号时请先添加'
  if (manualError.value || draft.courses.some(c => !/^\d{1,30}$/.test(c.id))) first['task-courses'] = manualError.value || '课程 ID 只支持数字，请检查已选课程'
  else if (!draft.courses.length || draft.courses.length > 100) first['task-courses'] = '请选择 1 至 100 门课程'
  if (!selectedModel.value?.has_key) second['task-model-config'] = '请选择一个已保存且包含密钥的 AI 配置'
  const integer = (v: unknown, min: number, max: number) => typeof v === 'number' && Number.isInteger(v) && v >= min && v <= max
  if (!integer(draft.ai_retries, 0, 10)) second['task-ai-retries'] = '请填写 0 至 10 的整数'
  if (!integer(draft.retries, 0, 10)) third['task-retries'] = '请填写 0 至 10 的整数'
  if (!integer(draft.retry_delay, 1, 3600)) third['task-retry-delay'] = '请填写 1 至 3600 的整数秒数'
  if (typeof draft.speed !== 'number' || !Number.isFinite(draft.speed) || draft.speed < 0.5 || draft.speed > 4) third['task-speed'] = '请填写 0.5 至 4 的视频速度'
  if (!integer(draft.jobs, 1, 10) || (draft.sequential && draft.jobs !== 1)) third['task-jobs'] = '请填写 1 至 10 的整数；顺序解锁时为 1'
  if (!['retry', 'skip', 'stop'].includes(draft.notopen_action)) third['task-notopen'] = '请选择未开放章节的处理方式'
  return [first, second, third]
})
const errors = computed<Record<string, string>>(() => attempted.value[step.value] ? invalid.value[step.value] || {} : {})

watch(() => state.loading, loading => { if (!loading && !draftReady.value) beginTask() }, { immediate: true })
watch(() => draft.account_id, () => {
  request?.abort()
  draft.courses = []
  manualIDs.value = ''
  courses.value = []
  coursesLoaded.value = false
  coursesError.value = ''
  search.value = ''
  if (selectedAccount.value) void loadCourses()
}, { flush: 'sync' })
watch(() => draft.sequential, value => { if (value) draft.jobs = 1 })

async function loadCourses() {
  request?.abort()
  if (!selectedAccount.value) return
  const id = draft.account_id
  const controller = new AbortController()
  request = controller
  const timeout = setTimeout(() => controller.abort(), 75000)
  coursesLoading.value = true
  coursesError.value = ''
  try {
    const result = await api<{ courses: Course[] }>(`/accounts/${id}/courses`, 'GET', undefined, controller.signal)
    if (request !== controller || draft.account_id !== id) return
    courses.value = result.courses
    draft.courses = draft.courses.map(course => ({ ...course, name: result.courses.find(item => item.id === course.id)?.name || course.name }))
    coursesLoaded.value = true
  } catch(e) {
    if (request === controller && draft.account_id === id) coursesError.value = controller.signal.aborted ? '课程读取超时，请重新获取' : e instanceof Error ? e.message : '课程读取失败'
  } finally {
    clearTimeout(timeout)
    if (request === controller) { coursesLoading.value = false; request = undefined }
  }
}
function toggleCourse(course: Course) {
  const index = draft.courses.findIndex(c => c.id === course.id)
  if (index >= 0) draft.courses.splice(index, 1)
  else draft.courses.push({ id: course.id, name: course.name })
}
function updateManual(value: string) {
  manualIDs.value = value
  const ids = [...new Set(value.split(/[,，\s]+/).filter(Boolean))]
  draft.courses = ids.map(id => ({ id, name: courses.value.find(c => c.id === id)?.name || draft.courses.find(c => c.id === id)?.name || '' }))
}
function setCourseMode(mode: 'list' | 'manual') {
  manualIDs.value = draft.courses.map(c => c.id).join(', ')
  courseMode.value = mode
}
async function next() {
  attempted.value[step.value] = true
  const first = Object.keys(invalid.value[step.value] || {})[0]
  if (first) { await revealField(first); return }
  step.value++
  window.scrollTo(0, 0)
}
function previous() { step.value--; window.scrollTo(0, 0) }
async function submit() {
  attempted.value = [true, true, true]
  for (let i = 0; i < 3; i++) {
    const first = Object.keys(invalid.value[i] || {})[0]
    if (first) { step.value = i; await revealField(first); return }
  }
  await act(async () => { await saveTask(); ElMessage.success('任务已保存'); emit('saved') })
}
onMounted(() => { if (selectedAccount.value && step.value === 0) void loadCourses() })
onUnmounted(() => { request?.abort(); request = undefined })
</script>

<template>
  <section class="task-page" :class="{ 'task-run': step === 2 }">
    <ol class="task-steps" aria-label="任务配置步骤">
      <li v-for="(label, index) in ['账号与课程', 'AI 模型', '运行设置']" :key="label" :class="{ active: step === index, done: step > index }" :aria-current="step === index ? 'step' : undefined"><span class="step-index">{{ step > index ? '✓' : index + 1 }}</span><span class="step-label">{{ label }}</span></li>
    </ol>
    <template v-if="step === 0">
      <div class="task-field"><label for="task-name">任务名称</label><el-input id="task-name" v-model="draft.name" maxlength="100" placeholder="例如：本学期学习计划" :disabled="state.busy" :class="{ 'field-invalid': errors['task-name'] }" :aria-invalid="!!errors['task-name']" aria-describedby="error-task-name" /><p v-if="errors['task-name']" id="error-task-name" class="field-error" role="alert">{{ errors['task-name'] }}</p></div>
      <article class="task-panel">
        <div class="task-panel-heading"><h2>学习账号</h2><button class="task-text-button" type="button" @click="emit('accounts')">管理账号<UiIcon name="chevron" :size="13" /></button></div>
        <label class="field-label" for="task-account">选择已保存的账号</label>
        <button id="task-account" class="task-picker-trigger" type="button" aria-haspopup="dialog" :aria-expanded="accountPicker" :disabled="state.loading || state.busy" @click="accountPicker = true" :class="{ 'field-invalid': errors['task-account'] }" :aria-invalid="!!errors['task-account']" aria-describedby="error-task-account"><span class="picker-trigger-icon"><UiIcon name="user" :size="21" /></span><span class="picker-trigger-copy"><strong>{{ selectedAccount?.name || (draft.account_id ? '原账号已移除' : '请选择学习账号') }}</strong><small v-if="selectedAccount">{{ maskAccount(selectedAccount.username) }}</small></span><UiIcon name="chevron" :size="17" /></button><p v-if="errors['task-account']" id="error-task-account" class="field-error" role="alert">{{ errors['task-account'] }}</p>
        <p v-if="!state.accounts.length" class="task-hint">还没有学习账号，<button class="inline-link" @click="emit('accounts')">先去添加账号</button>。</p>
        <p v-else-if="!selectedAccount && draft.account_id" class="task-error" role="alert">此任务原来的账号已被移除，请重新选择。</p>
        <p v-else class="task-hint">{{ selectedAccount?.id === state.default_account_id ? '已选中当前默认账号。' : '仅用于本任务，不改变默认账号。' }}切换账号后将重新获取课程。</p>
      </article>
      <div class="task-section-heading"><h2>选择课程</h2><button class="task-text-button" type="button" :disabled="!selectedAccount || coursesLoading" @click="loadCourses"><UiIcon name="refresh" :size="14" />{{ coursesLoading ? '读取中…' : '刷新课程' }}</button></div>
      <div id="task-courses" class="course-validation-group" tabindex="-1" :class="{ 'field-invalid': errors['task-courses'] }" :aria-invalid="!!errors['task-courses']" aria-describedby="error-task-courses">
      <div class="task-segment" aria-label="课程选择方式"><button :class="{ active: courseMode === 'list' }" :aria-pressed="courseMode === 'list'" @click="setCourseMode('list')">课程列表</button><button :class="{ active: courseMode === 'manual' }" :aria-pressed="courseMode === 'manual'" @click="setCourseMode('manual')">手动填写 ID</button></div>
      <template v-if="courseMode === 'list'">
        <el-input v-model="search" aria-label="搜索课程" placeholder="搜索课程名称或 ID" clearable :disabled="!selectedAccount" />
        <p v-if="coursesLoading" class="task-hint" role="status">正在获取该账号的课程…</p>
        <div v-if="coursesError" class="task-error" role="alert">{{ coursesError }}<button class="inline-link" @click="loadCourses">重新获取</button></div>
        <div class="task-course-list">
          <label v-for="course in availableCourses" :key="course.id" class="task-course" :class="{ selected: draft.courses.some(c => c.id === course.id) }">
            <input type="checkbox" :checked="draft.courses.some(c => c.id === course.id)" :disabled="!selectedAccount || (course.unavailable && !draft.courses.some(c => c.id === course.id))" @change="toggleCourse(course)" />
            <span><strong>{{ course.name || '课程 ' + course.id }}</strong><small>ID {{ course.id }}<template v-if="course.teacher"> · {{ course.teacher }}</template><template v-if="course.unavailable"> · 暂未开放</template></small></span>
          </label>
        </div>
        <p v-if="!availableCourses.length && !coursesLoading && !coursesError" class="task-hint">{{ !selectedAccount ? '请先选择一个学习账号。' : coursesLoaded ? search ? '未找到匹配的课程。' : '该账号暂时没有课程，也可以手动填写课程 ID。' : '选择账号后读取课程。' }}</p>
      </template>
      <template v-else><label class="field-label" for="task-course-ids">课程 ID</label><el-input id="task-course-ids" :model-value="manualIDs" type="textarea" :rows="3" placeholder="例如：247370145, 24810562" :disabled="!selectedAccount" @update:model-value="updateManual" /><p class="task-hint">多个课程 ID 用逗号或空格分隔。</p><p v-if="manualError" class="task-error" role="alert">{{ manualError }}</p></template>
      <p class="task-hint selected-count">已选择 {{ draft.courses.length }} 门课程</p></div><p v-if="errors['task-courses']" id="error-task-courses" class="field-error" role="alert">{{ errors['task-courses'] }}</p>
    </template>
    <template v-else-if="step === 1">
      <article class="task-panel">
        <div class="task-panel-heading"><h2>模型配置</h2><button class="task-text-button" type="button" @click="emit('models')">管理配置<UiIcon name="chevron" :size="13" /></button></div>
        <label class="field-label" for="task-model-config">选择已保存的 AI 配置</label>
        <button id="task-model-config" class="task-picker-trigger" type="button" aria-haspopup="dialog" :aria-expanded="modelPicker" :disabled="state.busy" @click="modelPicker = true" :class="{ 'field-invalid': errors['task-model-config'] }" :aria-invalid="!!errors['task-model-config']" aria-describedby="error-task-model-config"><span class="picker-trigger-icon"><UiIcon name="settings" :size="21" /></span><span class="picker-trigger-copy"><strong>{{ selectedModel?.name || (draft.model_config_id ? '原模型配置已移除' : '请选择 AI 模型配置') }}</strong><small v-if="selectedModel">{{ selectedModel.primary }}</small></span><UiIcon name="chevron" :size="17" /></button><p v-if="errors['task-model-config']" id="error-task-model-config" class="field-error" role="alert">{{ errors['task-model-config'] }}</p>
        <p v-if="!state.models.length" class="task-hint">还没有模型配置，<button class="inline-link" @click="emit('models')">先去添加模型配置</button>。</p>
        <p v-else-if="draft.model_config_id && !selectedModel" class="task-error" role="alert">原模型配置已被移除，请重新选择。</p>
        <template v-if="selectedModel"><dl class="task-model-summary"><div><dt>主模型</dt><dd>{{ selectedModel.primary }}</dd></div><div><dt>备用模型</dt><dd>{{ selectedModel.fallback || '未设置' }}</dd></div></dl><p class="task-hint">接口、密钥及模型沿用所选配置，统一在“我的 → AI 模型”维护。</p></template>
      </article>
      <article class="task-panel"><div class="task-setting-row"><label for="task-ai-retries">AI 请求重试</label><el-input-number id="task-ai-retries" v-model="draft.ai_retries" :min="0" :max="10" size="small" :class="{ 'field-invalid': errors['task-ai-retries'] }" :aria-invalid="!!errors['task-ai-retries']" aria-describedby="error-task-ai-retries" /><p v-if="errors['task-ai-retries']" id="error-task-ai-retries" class="field-error" role="alert">{{ errors['task-ai-retries'] }}</p></div><p class="task-hint">主模型调用失败后，按所选配置尝试备用模型。</p></article>
      <div class="task-info-note"><UiIcon name="lock" :size="17" /><p>本任务只引用配置，不重复保存 API Key。</p></div>
    </template>
    <template v-else>
      <BackgroundSettings />
      <div class="task-selection-note"><strong>{{ draft.name }}</strong><p>{{ selectedAccount?.name || '姓名未获取' }} · {{ draft.courses.length }} 门课程 · {{ selectedModel?.name || '配置已移除' }}</p></div>
      <article class="task-panel"><h2>失败自动重试</h2><div class="task-two-fields"><div class="task-field"><label for="task-retries">最多重试（次）</label><el-input-number id="task-retries" v-model="draft.retries" :min="0" :max="10" :class="{ 'field-invalid': errors['task-retries'] }" :aria-invalid="!!errors['task-retries']" aria-describedby="error-task-retries" /><p v-if="errors['task-retries']" id="error-task-retries" class="field-error" role="alert">{{ errors['task-retries'] }}</p></div><div class="task-field"><label for="task-retry-delay">首次等待（秒）</label><el-input-number id="task-retry-delay" v-model="draft.retry_delay" :min="1" :max="3600" :class="{ 'field-invalid': errors['task-retry-delay'] }" :aria-invalid="!!errors['task-retry-delay']" aria-describedby="error-task-retry-delay" /><p v-if="errors['task-retry-delay']" id="error-task-retry-delay" class="field-error" role="alert">{{ errors['task-retry-delay'] }}</p></div></div></article>
      <article class="task-panel"><h2>高级运行选项</h2><div class="task-two-fields"><div class="task-field"><label for="task-speed">视频速度</label><el-input-number id="task-speed" v-model="draft.speed" :min="0.5" :max="4" :step="0.5" :precision="1" :class="{ 'field-invalid': errors['task-speed'] }" :aria-invalid="!!errors['task-speed']" aria-describedby="error-task-speed" /><p v-if="errors['task-speed']" id="error-task-speed" class="field-error" role="alert">{{ errors['task-speed'] }}</p></div><div class="task-field"><label for="task-jobs">章节并发数</label><el-input-number id="task-jobs" v-model="draft.jobs" :min="1" :max="10" :disabled="draft.sequential" :class="{ 'field-invalid': errors['task-jobs'] }" :aria-invalid="!!errors['task-jobs']" aria-describedby="error-task-jobs" /><p v-if="errors['task-jobs']" id="error-task-jobs" class="field-error" role="alert">{{ errors['task-jobs'] }}</p></div></div><div class="task-field"><label for="task-notopen">未开放章节</label><el-select id="task-notopen" v-model="draft.notopen_action" :class="{ 'field-invalid': errors['task-notopen'] }" :aria-invalid="!!errors['task-notopen']" aria-describedby="error-task-notopen"><el-option label="重试当前章节" value="retry" /><el-option label="跳过未开放章节" value="skip" /><el-option label="停止任务" value="stop" /></el-select><p v-if="errors['task-notopen']" id="error-task-notopen" class="field-error" role="alert">{{ errors['task-notopen'] }}</p></div><div class="task-setting-row"><label for="task-sequential">顺序解锁章节</label><el-switch id="task-sequential" v-model="draft.sequential" /></div><div class="task-setting-row"><label for="task-auto-submit">自动提交答题</label><el-switch id="task-auto-submit" v-model="draft.auto_submit" /></div></article>
    </template>
    <TaskPickerSheet v-model="draft.account_id" v-model:visible="accountPicker" title="选择学习账号" hint="仅选择本任务使用的账号，不改变默认账号" icon="user" manage-label="管理学习账号" :options="accountOptions" @manage="emit('accounts')" />
    <TaskPickerSheet v-model="draft.model_config_id" v-model:visible="modelPicker" title="选择 AI 配置" hint="沿用配置中的接口、密钥、主模型与备用模型" icon="settings" manage-label="管理 AI 模型配置" :options="modelOptions" @manage="emit('models')" />
    <footer class="task-footer"><button v-if="step > 0" class="outline-button" type="button" :disabled="state.busy" @click="previous">上一步</button><button v-if="step < 2" class="primary-button" type="button" :disabled="state.loading || state.busy" @click="next">{{ step === 0 ? '下一步 · AI 模型' : '下一步 · 运行设置' }}<UiIcon name="arrow" :size="16" /></button><button v-else class="primary-button" type="button" :disabled="state.busy" @click="submit">{{ state.busy ? '正在保存…' : '保存任务' }}</button></footer>
  </section>
</template>


