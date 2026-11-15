import { computed, reactive } from 'vue'
import { ElMessage } from 'element-plus'
export interface Account { id: string; name: string; username: string; status: string; checked_at?: string }
export interface Model { id: string; name: string; endpoint: string; protocol?: 'chat' | 'responses' | 'anthropic'; models: string[]; primary: string; fallback: string; has_key: boolean }
export interface Config { accounts: Account[]; default_account_id: string; models: Model[] }
export const state = reactive({ accounts: [] as Account[], default_account_id: '', models: [] as Model[], loading: true, busy: false, error: '' })
export const currentAccount = computed(() => state.accounts.find(a => a.id === state.default_account_id))
export function maskAccount(value: string) { return value.length > 6 ? value.slice(0, 3) + '****' + value.slice(-4) : value.slice(0, 1) + '***' }
export async function api<T>(path: string, method = 'GET', body?: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch('/api' + path, { method, headers: body === undefined ? {} : { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body), signal: signal ?? AbortSignal.timeout(75000) })
  const data = await res.json().catch(() => { throw new Error('服务响应异常，请检查本机服务是否启动') })
  if (!res.ok) throw new Error(data.error || '请求失败，请重试')
  return data as T
}
export function applyConfig(c: Config) { state.accounts = c.accounts; state.default_account_id = c.default_account_id; state.models = c.models; state.error = '' }
export async function refresh() {
  state.loading = true
  try { applyConfig(await api<Config>('/settings')) } catch(e) { state.error = e instanceof Error ? e.message : '无法连接本机服务' } finally { state.loading = false }
}
export async function mutate(path: string, method: string, body?: unknown) { applyConfig(await api<Config>(path, method, body)) }
export async function act(action: () => Promise<void>) {
  if (state.busy) return
  state.busy = true
  try { await action() } catch(e) { ElMessage.error(e instanceof Error ? e.message : '操作失败，请重试') } finally { state.busy = false }
}
// The selected account is only a default for new tasks. Existing tasks retain their own account_id.
export async function getTaskDefaults() { return api<{ account_id: string }>('/task-defaults') }
