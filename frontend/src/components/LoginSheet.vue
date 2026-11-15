<script setup lang="ts">
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { act, state, mutate } from '../stores/settings'
const visible = defineModel<boolean>({ required: true })
const form = reactive({ username: '', password: '' })
function clear() { Object.assign(form, { username: '', password: '' }) }
async function save() {
  if (!form.username.trim() || !form.password) { ElMessage.warning('请填写账号和密码'); return }
  await act(async () => { await mutate('/accounts', 'POST', form); visible.value = false; ElMessage.success('账号已连接并保存') })
}
</script>
<template>
  <el-drawer v-model="visible" title="连接学习账号" direction="btt" size="auto" class="bottom-sheet" :close-on-click-modal="!state.busy" :close-on-press-escape="!state.busy" :show-close="!state.busy" @closed="clear">
    <form class="editor-form" @submit.prevent="save">
      <label for="account-username">手机号 / 学号</label><el-input id="account-username" v-model="form.username" autocomplete="username" maxlength="256" placeholder="请输入学习通账号" :disabled="state.busy" />
      <label for="account-password">登录密码</label><el-input id="account-password" v-model="form.password" type="password" show-password autocomplete="current-password" maxlength="4096" placeholder="请输入登录密码" :disabled="state.busy" />
      <p class="sheet-note">登录后自动获取真实姓名，凭据加密保存在本机，不会发送给 AI 模型。重复连接会更新资料与密码，已有默认账号保持不变。</p>
      <button class="primary-button full-width" type="submit" :disabled="state.busy">{{ state.busy ? '正在验证并连接…' : '连接并保存' }}</button>
    </form>
  </el-drawer>
</template>
