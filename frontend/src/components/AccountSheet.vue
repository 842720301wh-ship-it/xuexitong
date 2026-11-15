<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import UiIcon from './UiIcon.vue'
import { state, currentAccount, maskAccount, act, mutate, refresh } from '../stores/settings'
const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ add: [] }>()
const checking = ref(false)
const accountList = ref<HTMLElement>()
async function revealSelected() { await nextTick(); accountList.value?.querySelector('[aria-pressed="true"]')?.scrollIntoView({ block: 'nearest' }) }
async function select(id: string) { await act(async () => { await mutate('/accounts/default', 'PUT', { id }); visible.value = false; ElMessage.success('已设为任务默认账号') }) }
async function check() {
  const id = currentAccount.value?.id
  if (!id) return
  checking.value = true
  try { await act(async () => { try { await mutate(`/accounts/${id}/check`, 'POST', {}); ElMessage.success('账号连接正常') } finally { await refresh() } }) } finally { checking.value = false }
}
async function remove() {
  const account = currentAccount.value
  if (!account) return
  try { await ElMessageBox.confirm(`移除 ${maskAccount(account.username)} 的本机凭据？${state.accounts.length > 1 ? '将自动使用列表中的下一个账号作为任务默认账号。' : '移除后新任务需要重新选择账号。'}`, '移除账号', { confirmButtonText: '移除', cancelButtonText: '取消', type: 'warning' }) } catch { return }
  await act(async () => { await mutate(`/accounts/${account.id}`, 'DELETE'); if (!state.accounts.length) visible.value = false; ElMessage.success('已移除本机账号') })
}
</script>
<template>
  <el-drawer v-model="visible" direction="btt" size="auto" title="账号" class="bottom-sheet account-sheet" :close-on-click-modal="!state.busy" :close-on-press-escape="!state.busy" :show-close="!state.busy" @opened="revealSelected">
    <p class="sheet-note top-note">切换后作为新建任务的默认账号</p>
    <div ref="accountList" class="account-list" role="group" aria-label="选择默认账号" tabindex="0">
      <button v-for="account in state.accounts" :key="account.id" class="account-choice" :aria-pressed="account.id === state.default_account_id" :disabled="state.busy" @click="select(account.id)">
        <UiIcon name="user" /><span><strong>{{ account.name || '姓名未获取' }}</strong><small>{{ maskAccount(account.username) }}<template v-if="account.id === state.default_account_id"> · 任务默认账号</template></small></span><UiIcon v-if="account.id === state.default_account_id" name="check" :size="19" />
      </button>
    </div>
    <p v-if="!state.accounts.length" class="sheet-note">还没有保存账号，连接你的第一个学习账号。</p>
    <button class="sheet-row" :disabled="!currentAccount || state.busy" @click="check">{{ checking ? '正在检查连接…' : '检查连接' }}<UiIcon name="chevron" :size="17" /></button>
    <button class="sheet-row" :disabled="state.busy" @click="visible = false; emit('add')">连接其他账号<UiIcon name="chevron" :size="17" /></button>
    <button class="sheet-row danger" :disabled="!currentAccount || state.busy" @click="remove">移除本机账号</button>
  </el-drawer>
</template>
