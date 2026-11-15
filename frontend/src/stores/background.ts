import { reactive } from 'vue'
interface AndroidHost { status(): string; startTask(id: string): void; stopTask(id: string): void; notificationSettings(): void; batterySettings(): void }
export function androidHost(): AndroidHost | undefined { return (window as unknown as { TiantianAndroid?: AndroidHost }).TiantianAndroid }
export const backgroundStatus = reactive({ available: false, notifications: false, battery_exempt: false, running: false })
export function refreshBackground() { try { Object.assign(backgroundStatus, androidHost() ? { ...JSON.parse(androidHost()!.status()), available: true } : { available: false }) } catch { backgroundStatus.available = false } }
