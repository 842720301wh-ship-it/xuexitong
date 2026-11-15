export const projectURL = 'https://github.com/842720301wh-ship-it/xuexitong.git'
export const groupNumber = '345353411'
export const aboutPages = [
  { id: 'models', title: 'AI 模型', subtitle: '保存与管理多个模型配置', icon: 'settings' },
  { id: 'license', title: '开源许可', subtitle: '查看源码与许可说明', icon: 'code' },
  { id: 'sponsor', title: '赞助', subtitle: '自愿支持项目维护', icon: 'heart' },
  { id: 'disclaimer', title: '免责声明', subtitle: '免费使用与使用约定', icon: 'shield' },
  { id: 'contact', title: '联系我们', subtitle: '问题反馈与交流', icon: 'chat' },
] as const
export type AboutPage = Exclude<(typeof aboutPages)[number]['id'], 'models'>
export type Page = 'home' | 'records' | 'my' | 'models' | 'task' | AboutPage
export const aboutImages = {
  sponsor: { url: '/about/sponsor.jpg', filename: '天天向上-赞赏码.jpg' },
  contact: { url: '/about/contact.jpg', filename: '天天向上-QQ群.jpg' },
} as const
