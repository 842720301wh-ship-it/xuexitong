<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import UiIcon from '../components/UiIcon.vue'
import { aboutImages, groupNumber, projectURL, type AboutPage } from '../config/about'
import { copyGroupNumber, saveAboutImage } from '../services/phone'

defineProps<{ page: AboutPage }>()
const emit = defineEmits<{ open: [page: AboutPage] }>()
const saving = ref(false)
const native = false
const rules = [
  {
    title: '软件免费，禁止收费',
    text: '本软件免费提供。禁止售卖、付费分发，或以激活、解锁功能等名义收取费用。',
  },
  {
    title: '禁止恶意篡改',
    text: '禁止植入广告、恶意代码、窃取隐私，或冒充本项目发布恶意修改版本。',
  },
  {
    title: '遵守法律与平台规则',
    text: '请合法、合理使用软件，不得用于侵权、欺诈或其他违法违规活动。',
  },
  {
    title: '注意来源与使用风险',
    text: '请从项目仓库获取软件，谨慎安装第三方修改版本，妥善保护个人账号与信息。',
  },
]
async function save(image: keyof typeof aboutImages) {
  if (saving.value) return
  saving.value = true
  try {
    await saveAboutImage(image)
    ElMessage.success(native ? '图片已保存' : '已开始下载图片')
  } catch (error) {
    if (error instanceof Error && error.message === 'save_cancelled') return
    ElMessage.error(error instanceof Error ? error.message : '图片保存失败，请重试')
  } finally {
    saving.value = false
  }
}
async function copyGroup() {
  try {
    await copyGroupNumber()
    ElMessage.success(`群号已复制：${groupNumber}`)
  } catch {
    ElMessage.error(`复制失败，请手动复制群号：${groupNumber}`)
  }
}
</script>

<template>
  <section class="about-page" :class="`about-page--${page}`" :aria-label="`${page}详情`">
    <template v-if="page === 'license'">
      <div class="about-hero">
        <span class="about-hero-icon"><UiIcon name="code" :size="28" /></span>
        <h2>让代码保持开放</h2>
        <p>了解项目，查看源码与许可说明</p>
      </div>
      <div class="about-card">
        <p class="about-label">项目仓库</p>
        <div class="about-repo-title"><strong>xuexitong</strong><span>GitHub</span></div>
        <p class="about-repo-path">842720301wh-ship-it / xuexitong</p>
        <hr class="about-divider" />
        <p class="about-paragraph">
          在项目仓库中查看源代码、更新记录，<br />也欢迎提交问题反馈与改进建议。
        </p>
        <a
          class="about-button about-button--primary"
          :href="projectURL"
          :target="native ? undefined : '_blank'"
          rel="noopener noreferrer"
          ><UiIcon name="external" :size="17" />访问开源仓库</a
        >
      </div>
      <div class="about-card">
        <h3 class="about-card-title">许可说明</h3>
        <p class="about-paragraph">
          使用、修改或分发本项目前，请先阅读项目仓库中公示的许可条款与相关说明。
        </p>
      </div>
      <p class="about-footnote">源码及许可信息以项目仓库公示为准</p>
    </template>
    <template v-else-if="page === 'sponsor'">
      <div class="about-hero">
        <span class="about-hero-icon"><UiIcon name="heart" :size="28" /></span>
        <h2>如果它帮到了你</h2>
        <p>你的支持，是持续维护的一份动力</p>
      </div>
      <div class="about-card about-qr-card">
        <p class="about-qr-label">微信赞赏</p>
        <img
          class="about-sponsor-image"
          :src="aboutImages.sponsor.url"
          alt="微信赞赏码"
          width="1053"
          height="1053"
        />
        <p class="about-qr-help">使用微信扫一扫，或保存后在微信中识别</p>
        <button class="about-button" :disabled="saving" @click="save('sponsor')">
          <UiIcon name="download" :size="17" />{{ saving ? '正在保存…' : '保存赞赏码' }}
        </button>
      </div>
      <div class="about-note">
        <strong>自愿赞助，量力而行</strong>
        <p>软件免费使用。赞助不解锁额外功能，<br />也不影响任何现有功能的使用。</p>
      </div>
      <p class="about-footnote">感谢每一份支持，也感谢每一次使用</p>
    </template>
    <template v-else-if="page === 'disclaimer'">
      <div class="about-statement">
        <UiIcon name="shield" :size="29" />
        <div>
          <h2>免费使用，规范使用</h2>
          <p>使用前，请了解以下说明</p>
        </div>
      </div>
      <div class="about-card about-rules">
        <article v-for="(rule, index) in rules" :key="rule.title" class="about-rule">
          <div class="about-rule-title">
            <span>0{{ index + 1 }}</span>
            <h3>{{ rule.title }}</h3>
          </div>
          <p>{{ rule.text }}</p>
        </article>
      </div>
      <p class="about-footnote">
        如遇收费售卖或恶意修改版本，<br />可通过<button
          class="about-text-button"
          @click="emit('open', 'contact')"
        >
          「联系我们」</button
        >反馈。
      </p>
    </template>
    <template v-else>
      <div class="about-hero">
        <span class="about-hero-icon"><UiIcon name="chat" :size="28" /></span>
        <h2>一起把它变得更好</h2>
        <p>使用问题、功能建议，都欢迎来聊聊</p>
      </div>
      <div class="about-card about-qr-card about-contact-card">
        <h3 class="about-group-name">太阳当空照</h3>
        <p class="about-group-id">
          QQ 交流群 · <span class="about-group-number">{{ groupNumber }}</span>
        </p>
        <div class="about-contact-code">
          <img
            :src="aboutImages.contact.url"
            alt="太阳当空照 QQ 群二维码，群号 345353411"
            width="1174"
            height="2087"
          />
        </div>
        <p class="about-qr-help">使用 QQ 扫一扫，或搜索群号加入</p>
        <div class="about-button-row">
          <button class="about-button" :disabled="saving" @click="save('contact')">
            <UiIcon name="download" :size="17" />{{ saving ? '正在保存…' : '保存二维码' }}</button
          ><button class="about-button about-button--primary" @click="copyGroup">
            <UiIcon name="copy" :size="17" />复制群号
          </button>
        </div>
      </div>
      <p class="about-contact-note">
        <strong>反馈小提示</strong
        ><br />描述问题时，可附上软件版本、操作步骤与截图。<br />请勿发送密码、验证码等敏感信息。
      </p>
    </template>
  </section>
</template>

