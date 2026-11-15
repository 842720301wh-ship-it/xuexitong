<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { androidHost, backgroundStatus, refreshBackground } from '../stores/background'
function notifications() { androidHost()?.notificationSettings() }
function battery() { if (androidHost()) androidHost()!.batterySettings(); else ElMessage.info('请在 Android 手机上设置后台权限') }
onMounted(() => { refreshBackground(); window.addEventListener('focus', refreshBackground); document.addEventListener('visibilitychange', refreshBackground) })
onUnmounted(() => { window.removeEventListener('focus', refreshBackground); document.removeEventListener('visibilitychange', refreshBackground) })
</script>
<template>
  <article class="task-panel background-panel"><div class="task-panel-heading"><h2>手机后台执行</h2><small>{{ backgroundStatus.available ? (backgroundStatus.running ? '服务运行中' : '手机本地执行') : '请使用 Android 应用' }}</small></div>
    <div class="task-setting-row"><div><strong>常驻通知</strong><p class="task-hint">显示运行状态与停止入口</p></div><el-switch aria-label="常驻通知" :model-value="backgroundStatus.notifications" :disabled="!backgroundStatus.available" @change="notifications" /></div>
    <div class="task-setting-row"><div><strong>电池优化</strong><p class="task-hint">允许应用在后台运行</p></div><button class="outline-button battery-button" :disabled="!backgroundStatus.available" @click="battery">{{ backgroundStatus.battery_exempt ? '已允许' : '待设置' }} ›</button></div>
    <p class="task-hint">任务启动后使用前台服务持续执行；通知权限与后台限制由手机系统管理。</p>
  </article>
</template>
