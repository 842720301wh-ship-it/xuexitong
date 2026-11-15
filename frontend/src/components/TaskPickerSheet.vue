<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import UiIcon from './UiIcon.vue'
const props = defineProps<{ title: string; hint: string; icon: string; manageLabel: string; options: { id: string; title: string; subtitle: string; badge?: string; disabled?: boolean }[] }>()
const visible = defineModel<boolean>('visible', { required: true })
const selected = defineModel<string>({ required: true })
const emit = defineEmits<{ manage: [] }>()
const query = ref(''), list = ref<HTMLElement>()
const filtered = computed(() => { const q = query.value.trim().toLowerCase(); return props.options.filter(o => !q || `${o.title} ${o.subtitle}`.toLowerCase().includes(q)) })
async function reveal() { await nextTick(); list.value?.querySelector('[aria-checked="true"]')?.scrollIntoView({ block: 'nearest' }) }
function choose(id: string) { selected.value = id; visible.value = false }
function manage() { visible.value = false; emit('manage') }
</script>
<template>
  <el-drawer v-model="visible" :title="title" direction="btt" size="auto" append-to-body modal-class="task-picker-mask" class="bottom-sheet task-picker-sheet" @opened="reveal" @closed="query = ''">
    <p class="picker-hint">{{ hint }}</p>
    <el-input v-if="options.length > 4" v-model="query" :aria-label="'搜索' + title" placeholder="搜索名称或账号" clearable class="picker-search" />
    <div ref="list" class="picker-list" role="radiogroup" :aria-label="title" tabindex="0">
      <button v-for="option in filtered" :key="option.id" type="button" role="radio" :aria-checked="selected === option.id" :disabled="option.disabled" class="picker-option" @click="choose(option.id)"><span class="picker-icon"><UiIcon :name="icon" :size="23" /></span><span class="picker-copy"><strong>{{ option.title }}<small v-if="option.badge" class="picker-badge">{{ option.badge }}</small></strong><span>{{ option.subtitle }}</span></span><span class="picker-radio" :class="{ checked: selected === option.id }"><UiIcon v-if="selected === option.id" name="check" :size="13" /></span></button>
      <p v-if="!filtered.length" class="picker-empty">{{ options.length ? '没有匹配的配置' : '还没有可选配置，请先添加' }}</p>
    </div>
    <div class="picker-footer"><button class="outline-button full-width" @click="manage"><UiIcon name="settings" :size="17" />{{ manageLabel }}<UiIcon name="chevron" :size="14" /></button></div>
  </el-drawer>
</template>
