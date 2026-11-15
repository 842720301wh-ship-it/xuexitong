<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import UiIcon from '../components/UiIcon.vue'
import { revealField } from '../composables/fieldValidation'
import { act, api, mutate, state, type Model } from '../stores/settings'
const open = ref(false)
const blank = () => ({ id: '', name: '', endpoint: '', api_key: '', protocol: 'chat' as 'chat' | 'responses' | 'anthropic', primary: '', fallback: '' })
const form = ref(blank()), catalog = ref<string[]>([]), ready = ref(false), attempted = ref(false), connectionAttempted = ref(false), connectionError = ref('')
const busy = computed(() => state.busy)
const probeError = ref(''), verified = ref('')
function fingerprint() { return JSON.stringify([form.value.endpoint, form.value.api_key, form.value.protocol, form.value.primary]) }
const labels = { chat: 'Chat Completions', responses: 'Responses', anthropic: 'Anthropic Messages' }
let savedEndpoint = ''
function canonical(endpoint: string) { return endpoint.trim().replace(/\/+$|\/(chat\/completions|responses|messages|models)\/*$/g, '') }
const credentialErrors = computed<Record<string, string>>(() => {
  const e: Record<string, string> = {}
  try { const u = new URL(form.value.endpoint.trim()); if (u.username || u.password || u.search || u.hash || !(u.protocol === 'https:' || (u.protocol === 'http:' && ['localhost','127.0.0.1','[::1]'].includes(u.hostname)))) throw Error() }
  catch { e['model-endpoint'] = '请填写有效的 HTTPS 接口地址（本机可用 HTTP）' }
  if (!form.value.api_key.trim() && (!form.value.id || canonical(form.value.endpoint) !== canonical(savedEndpoint))) e['model-key'] = '请填写 API Key；更换接口地址后需重新输入'
  return e
})
const errors = computed<Record<string, string>>(() => {
  const e: Record<string, string> = {}
  if (attempted.value && !form.value.name.trim()) e['model-name'] = '请填写配置名称'
  if (attempted.value || connectionAttempted.value) Object.assign(e, credentialErrors.value)
  if (attempted.value) {
    if (!ready.value) e['model-discover'] = connectionError.value || '请先点击连接测试，自动获取模型'
    if (ready.value && !catalog.value.includes(form.value.primary)) e['primary-model'] = '请选择主模型'
    if (form.value.fallback && (!catalog.value.includes(form.value.fallback) || form.value.fallback === form.value.primary)) e['fallback-model'] = '请选择不同于主模型的备用模型，或清空'
  }
  return e
})
watch(() => [form.value.endpoint, form.value.api_key], () => { ready.value = false; catalog.value = []; form.value.primary = ''; form.value.fallback = ''; connectionError.value = '' }, { flush: 'sync' })
watch(() => [form.value.endpoint, form.value.api_key, form.value.protocol, form.value.primary], () => { verified.value = ''; probeError.value = '' }, { flush: 'sync' })
watch(() => form.value.endpoint, endpoint => {
  const path = endpoint.trim().replace(/\/+$/, '')
  if (path.endsWith('/responses')) form.value.protocol = 'responses'
  else if (path.endsWith('/messages')) form.value.protocol = 'anthropic'
  else if (path.endsWith('/chat/completions')) form.value.protocol = 'chat'
})
function edit(m?: Model) {
  form.value = blank()
  Object.assign(form.value, m ? { ...m, protocol: m.protocol || (m.endpoint.endsWith('/responses') ? 'responses' : m.endpoint.endsWith('/messages') ? 'anthropic' : 'chat'), api_key: '' } : blank())
  savedEndpoint = m?.endpoint || ''; catalog.value = m?.models.slice() || []; ready.value = !!m?.models.length
  attempted.value = false; connectionAttempted.value = false; connectionError.value = ''; probeError.value = ''; verified.value = ''; open.value = true
}
async function probe() {
  try {
    const result = await api<{message: string}>('/models/probe', 'POST', { config_id: form.value.id, endpoint: form.value.endpoint, api_key: form.value.api_key, protocol: form.value.protocol, primary: form.value.primary })
    verified.value = fingerprint(); probeError.value = ''; return result.message
  } catch (e) { probeError.value = e instanceof Error ? e.message : '主模型调用失败'; await revealField('model-protocol'); throw e }
}
async function testDraft() {
  if (!form.value.primary) { attempted.value = true; await revealField('primary-model'); return }
  await act(async () => { ElMessage.success(await probe()) })
}
async function discover() {
  connectionAttempted.value = true
  const first = Object.keys(credentialErrors.value)[0]
  if (first) { await revealField(first); return }
  await act(async () => {
    try {
      const result = await api<{ models: string[]; message: string }>('/models/discover', 'POST', { config_id: form.value.id, endpoint: form.value.endpoint, api_key: form.value.api_key, protocol: form.value.protocol })
      catalog.value = result.models; ready.value = true; connectionError.value = ''
      if (!catalog.value.includes(form.value.primary)) form.value.primary = ''
      if (!catalog.value.includes(form.value.fallback) || form.value.fallback === form.value.primary) form.value.fallback = ''
      ElMessage.success(result.message)
    } catch (e) { ready.value = false; connectionError.value = e instanceof Error ? e.message : '连接失败，请重试'; throw e }
    if (form.value.primary) { ElMessage.success(await probe()) }
  })
}
async function test(id: string) { await act(async () => { const r = await api<{ message: string }>(`/models/${id}/test`, 'POST', {}); ElMessage.success(r.message) }) }
async function save() {
  attempted.value = true
  const first = Object.keys(errors.value)[0]
  if (first) { await revealField(first); return }
  await act(async () => {
    await mutate(form.value.id ? `/models/${form.value.id}` : '/models', form.value.id ? 'PUT' : 'POST', { ...form.value, models: catalog.value })
    open.value = false; ElMessage.success('模型配置已保存')
  })
}
async function remove(m: Model) { try { await ElMessageBox.confirm(`删除“${m.name}”及本机保存的 API Key？`, '删除模型配置', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }) } catch { return }; await act(async () => { await mutate(`/models/${m.id}`, 'DELETE'); ElMessage.success('已删除模型配置') }) }
</script>
<template>
  <section class="models-page">
    <div class="page-intro"><h2>你的模型配置</h2><p>保存多组接口、密钥与模型，任务中直接选择。</p></div>
    <button class="primary-button full-width add-model" :disabled="busy" @click="edit()"><UiIcon name="settings" :size="18" />添加模型配置</button>
    <div v-if="!state.models.length" class="empty-state"><UiIcon name="settings" :size="40" /><p>还没有模型配置</p><small>支持 Chat Completions、Responses 和 Anthropic Messages</small></div>
    <article v-for="model in state.models" :key="model.id" class="model-card">
      <div class="model-heading"><span class="menu-icon"><UiIcon name="settings" /></span><div><h3>{{ model.name }}</h3><p>{{ labels[model.protocol || 'chat'] }} · {{ model.models.length }} 个模型 · {{ model.has_key ? '密钥已保存' : '未设置密钥' }}</p></div></div>
      <p class="model-endpoint">{{ model.endpoint }}</p><dl><div><dt>主模型</dt><dd>{{ model.primary }}</dd></div><div><dt>备用模型</dt><dd>{{ model.fallback || '未设置' }}</dd></div></dl>
      <div class="model-actions"><button :disabled="busy" @click="test(model.id)">{{ busy ? '请稍候…' : '测试连接' }}</button><button :disabled="busy" @click="edit(model)">编辑</button><button class="danger" :disabled="busy" @click="remove(model)">删除</button></div>
    </article>
    <p class="sheet-note">API Key 加密保存在本机。测试连接会向主模型发送一条简短消息。</p>
    <el-drawer v-model="open" :title="form.id ? '编辑模型配置' : '添加模型配置'" direction="btt" size="auto" class="bottom-sheet" :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy" @closed="!open && (form = blank())">
      <form class="editor-form" novalidate @submit.prevent="save">
        <label for="model-name">配置名称</label><el-input id="model-name" v-model="form.name" maxlength="100" placeholder="例如：日常学习助手" :disabled="busy" :class="{ 'field-invalid': errors['model-name'] }" :aria-invalid="!!errors['model-name']" aria-describedby="error-model-name" /><p v-if="errors['model-name']" id="error-model-name" class="field-error" role="alert">{{ errors['model-name'] }}</p>
        <label for="model-protocol">接口格式</label><el-select id="model-protocol" v-model="form.protocol" :class="{ 'field-invalid': probeError }" :aria-invalid="!!probeError" class="protocol-select" :disabled="busy"><el-option v-for="(label, value) in labels" :key="value" :label="label" :value="value" /></el-select>
        <label for="model-endpoint">API 接口地址</label><el-input id="model-endpoint" v-model="form.endpoint" placeholder="https://api.example.com/v1" :disabled="busy" :class="{ 'field-invalid': errors['model-endpoint'] }" :aria-invalid="!!errors['model-endpoint']" aria-describedby="error-model-endpoint" /><p v-if="errors['model-endpoint']" id="error-model-endpoint" class="field-error" role="alert">{{ errors['model-endpoint'] }}</p>
        <label for="model-key">API Key</label><el-input id="model-key" v-model="form.api_key" type="password" show-password autocomplete="off" :placeholder="form.id ? '留空沿用已保存的密钥' : '请输入 API Key'" :disabled="busy" :class="{ 'field-invalid': errors['model-key'] }" :aria-invalid="!!errors['model-key']" aria-describedby="error-model-key" /><p v-if="errors['model-key']" id="error-model-key" class="field-error" role="alert">{{ errors['model-key'] }}</p>
        <div class="model-discovery"><button id="model-discover" class="outline-button full-width" type="button" :disabled="busy" :class="{ 'field-invalid': errors['model-discover'] || connectionError }" @click="discover"><UiIcon name="refresh" :size="16" />{{ busy ? '正在连接…' : '连接测试' }}</button><p v-if="connectionError || errors['model-discover']" class="field-error" role="alert">{{ connectionError || errors['model-discover'] }}</p><p v-else class="discovery-status" role="status">{{ ready ? `已获取 ${catalog.length} 个模型，可在下方搜索选择` : '连接后自动获取可用模型，无需手动填写' }}</p></div>
        <div class="form-columns"><div><label for="primary-model">主模型</label><el-select id="primary-model" v-model="form.primary" filterable placeholder="选择主模型" :disabled="busy || !ready" :class="{ 'field-invalid': errors['primary-model'] }" :aria-invalid="!!errors['primary-model']"><el-option v-for="m in catalog" :key="m" :label="m" :value="m" /></el-select><p v-if="errors['primary-model']" class="field-error" role="alert">{{ errors['primary-model'] }}</p></div><div><label for="fallback-model">备用模型（可选）</label><el-select id="fallback-model" v-model="form.fallback" filterable clearable placeholder="不设置" :disabled="busy || !ready" :class="{ 'field-invalid': errors['fallback-model'] }" :aria-invalid="!!errors['fallback-model']"><el-option v-for="m in catalog.filter(id => id !== form.primary)" :key="m" :label="m" :value="m" /></el-select><p v-if="errors['fallback-model']" class="field-error" role="alert">{{ errors['fallback-model'] }}</p></div></div>
        <div v-if="ready" class="model-probe-result"><p v-if="probeError" class="field-error" role="alert">{{ probeError }}</p><p v-else class="discovery-status" role="status">{{ verified ? '主模型调用已验证通过' : '模型列表已获取，请选择主模型并验证调用' }}</p><button class="outline-button full-width" type="button" :disabled="busy || !form.primary" @click="testDraft">{{ busy ? '正在验证…' : '测试主模型' }}</button></div>
        <p class="sheet-note">填写基础地址或完整接口地址，保留服务商提供的路径前缀。连接测试获取模型列表；“测试主模型”验证实际调用，可能消耗少量额度。调用失败仍可保存配置；获取列表不代表模型可用或额度充足。</p>
        <div class="form-buttons"><button class="primary-button" type="submit" :disabled="busy">保存配置</button></div>
      </form>
    </el-drawer>
  </section>
</template>

