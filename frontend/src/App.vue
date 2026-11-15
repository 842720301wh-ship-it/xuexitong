<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import UiIcon from './components/UiIcon.vue'
import AccountSheet from './components/AccountSheet.vue'
import LoginSheet from './components/LoginSheet.vue'
import AboutView from './views/AboutView.vue'
import ModelsView from './views/ModelsView.vue'
import LearningHome from './views/LearningHome.vue'
import TaskView from './views/TaskView.vue'
import RecordsView from './views/RecordsView.vue'
import { beginTask, taskDraft, type Task } from './stores/tasks'
import { aboutPages, type Page } from './config/about'
import { state, currentAccount, maskAccount, refresh } from './stores/settings'
const knownPages = ['home', 'records', 'my', 'task', ...aboutPages.map(p => p.id)]
function route(): Page { const hash = location.hash.slice(1); return knownPages.includes(hash) ? hash as Page : 'my' }
const page = ref<Page>(route())
function navigate(p: Page) { location.hash = p }
function sync() { page.value = route(); window.scrollTo(0, 0) }
window.addEventListener('hashchange', sync)
onUnmounted(() => window.removeEventListener('hashchange', sync))
const subpage = computed(() => !['home', 'records', 'my'].includes(page.value))
const title = computed(() => page.value === 'task' ? taskDraft.id ? '编辑学习任务' : '新建学习任务' : page.value === 'records' ? '运行记录' : aboutPages.find(p => p.id === page.value)?.title || (page.value === 'home' ? '天天向上' : '我的'))
const modelReturnToTask = ref(false)
const backLabel = computed(() => page.value === 'task' ? '返回首页' : page.value === 'models' && modelReturnToTask.value ? '返回任务' : '返回我的')
const accountOpen = ref(false), loginOpen = ref(false)
function createTask(task?: Task) { beginTask(task); navigate('task') }
function back() { navigate(page.value === 'task' ? 'home' : page.value === 'models' && modelReturnToTask.value ? 'task' : 'my') }
function manageTaskModels() { modelReturnToTask.value = true; navigate('models') }
function openMenu(target: Page) { modelReturnToTask.value = false; navigate(target) }
onMounted(refresh)
</script>
<template>
  <div class="app-frame" :class="{ 'my-view': page === 'my', 'detail-view': subpage, 'task-view': page === 'task' }">
    <header class="app-navbar"><button v-if="subpage" class="icon-button back-button" :aria-label="backLabel" :disabled="state.busy" @click="back"><UiIcon name="back" /></button><h1>{{ title }}</h1><button v-if="page === 'home'" class="icon-button task-create-button" aria-label="新建学习任务" :disabled="state.loading || !!state.error" @click="createTask()">＋</button></header>
    <main>
      <div v-if="state.error" class="connection-error" role="alert">{{ state.error }}<button @click="refresh">重试连接</button></div>
      <div v-if="state.loading && !state.accounts.length && !state.models.length" class="loading-note" role="status">正在读取本机配置…</div>
      <section v-if="page === 'my'" class="my-page">
        <button class="profile-header" aria-label="账号资料" :disabled="state.loading || !!state.error" @click="accountOpen = true">
          <span class="profile-avatar"><UiIcon name="user" :size="38" /></span><span class="profile-copy"><strong>{{ currentAccount ? currentAccount.name || '姓名未获取' : '未连接账号' }}</strong><span>{{ currentAccount ? '账号：' + maskAccount(currentAccount.username) : '点击添加你的学习账号' }}</span><small v-if="currentAccount"><i />{{ currentAccount.status === 'connected' ? '已连接' : '待检查连接' }} · 任务默认账号</small><small v-else>多账号保存在本机，随时切换</small></span><UiIcon name="chevron" :size="20" />
        </button>
        <nav class="my-menu" aria-label="我的菜单"><button v-for="item in aboutPages" :key="item.id" class="my-menu-item" @click="openMenu(item.id)"><span class="menu-icon"><UiIcon :name="item.icon" :size="20" /></span><span class="menu-copy"><strong>{{ item.title }}</strong><small>{{ item.subtitle }}</small></span><UiIcon name="chevron" :size="17" /></button></nav>
        <p class="my-version">天天向上 · v0.2.0</p>
      </section>
      <ModelsView v-else-if="page === 'models'" />
      <TaskView v-else-if="page === 'task'" @accounts="accountOpen = true" @models="manageTaskModels" @saved="navigate('home')" />
      <LearningHome v-else-if="page === 'home'" @create="createTask()" @edit="createTask" @records="navigate('records')" />
      <RecordsView v-else-if="page === 'records'" @home="navigate('home')" />
      <AboutView v-else :page="page" @open="navigate" />
    </main>
    <nav v-if="!subpage" class="bottom-nav" aria-label="主导航"><button :class="{ active: page === 'home' }" :aria-current="page === 'home' ? 'page' : undefined" @click="navigate('home')"><UiIcon name="home" :size="25" />学习</button><button :class="{ active: page === 'records' }" :aria-current="page === 'records' ? 'page' : undefined" @click="navigate('records')"><UiIcon name="clock" :size="25" />记录</button><button :class="{ active: page === 'my' }" :aria-current="page === 'my' ? 'page' : undefined" @click="navigate('my')"><UiIcon name="user" :size="25" />我的</button></nav>
    <AccountSheet v-model="accountOpen" @add="loginOpen = true" /><LoginSheet v-model="loginOpen" />
  </div>
</template>

