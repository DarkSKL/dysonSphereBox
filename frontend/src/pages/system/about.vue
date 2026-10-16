<script setup lang="ts">
import {Copy, ExternalLink} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {onMounted} from 'vue'
import {BrowserOpenURL, ClipboardSetText} from '../../../wailsjs/runtime/runtime'
import {useAppVersion} from '../../composables/useAppVersion'

definePage({ meta: { layout: 'main', title: '关于我们' } })

interface AboutLink {
  readonly label: string
  readonly url: string
}

interface AboutContact {
  readonly label: string
  readonly value: string
}

interface AboutCredit {
  readonly role: string
  readonly names: string
  readonly url?: string
}

const PROJECT_URL = 'https://gitee.com/Dark-Skl/dyson-sphere-box'

const projectLinks: readonly AboutLink[] = [
  {label: '项目主页', url: PROJECT_URL},
  {label: 'Gitee Issues', url: `${PROJECT_URL}/issues`},
]

const contacts: readonly AboutContact[] = [
  {label: '微信号', value: 'DarkSKL'},
  {label: 'QQ 群', value: '779182569'},
]

const credits: readonly AboutCredit[] = [
  {role: '模组社区', names: 'Thunderstore', url: 'https://thunderstore.io/c/dyson-sphere-program/'},
  {role: '联机框架', names: 'Nebula Multiplayer 团队', url: 'https://github.com/NebulaModTeam/nebula'},
  {role: '模组框架', names: 'BepInEx 项目', url: 'https://github.com/BepInEx/BepInEx'},
  {role: '模组作者', names: 'AssemblerUI、BottleneckUI、DSPCalculator、FactoryLocator、CAAP_N'},
  {role: '蓝图生成器', names: 'B站 UP：神不在的黄昏'},
  {role: '蓝图预览器', names: '灰灰'},
]

const message = useMessage()
const {appVersion, loadAppVersion} = useAppVersion()

onMounted(() => {
  void loadAppVersion()
})

// =============== 外链与复制 ===============
// 说明：外链交给系统浏览器打开；联系方式复制到剪贴板，便于粘贴到微信/QQ 搜索。
// 边界：复制失败只提示，不影响页面其它内容。
const openUrl = (url: string): void => {
  BrowserOpenURL(url)
}

const copyContact = async (contact: AboutContact): Promise<void> => {
  try {
    await ClipboardSetText(contact.value)
    message.success(`已复制${contact.label}`)
  } catch {
    message.error('复制失败，请手动记录')
  }
}
</script>

<template>
  <!-- ~ 关于我们页面 -->
  <!-- * 说明：展示应用版本、开发者、项目主页与反馈入口、联系方式、鸣谢与免责声明 -->
  <!-- * 交互：链接通过系统浏览器打开；联系方式可一键复制 -->
  <!-- * 依赖：依赖 useAppVersion 读取构建版本，依赖 Wails BrowserOpenURL / ClipboardSetText 运行时 -->
  <!-- * 边界：页面只展示静态信息，不触发更新检查；检查更新入口在全局设置页 -->
  <section class="about-page" aria-labelledby="about-title">
    <h1 id="about-title" class="about-title">关于我们</h1>
    <dl class="about-list">
      <div class="about-row">
        <dt>版本</dt>
        <dd class="about-mono">{{ appVersion }}</dd>
      </div>
      <div class="about-row">
        <dt>开发者</dt>
        <dd>DarkSKL</dd>
      </div>
      <div class="about-row">
        <dt>项目主页与反馈</dt>
        <dd class="about-actions">
          <n-button v-for="link in projectLinks" :key="link.url" text @click="openUrl(link.url)">
            <template #icon>
              <ExternalLink :size="14" :stroke-width="1.8" />
            </template>
            {{ link.label }}
          </n-button>
        </dd>
      </div>
      <div class="about-row">
        <dt>联系方式</dt>
        <dd class="about-actions">
          <n-button
            v-for="contact in contacts"
            :key="contact.label"
            text
            :aria-label="`复制${contact.label} ${contact.value}`"
            @click="copyContact(contact)"
          >
            <template #icon>
              <Copy :size="14" :stroke-width="1.8" />
            </template>
            {{ contact.label }}：<span class="about-mono">{{ contact.value }}</span>
          </n-button>
        </dd>
      </div>
    </dl>

    <!-- ~ 鸣谢 -->
    <!-- * 说明：按角色列出依赖的社区、框架与作者；有公开主页的条目可点击打开 -->
    <section class="about-section" aria-labelledby="about-credits-title">
      <h2 id="about-credits-title" class="about-section-title">鸣谢</h2>
      <dl class="about-list">
        <div v-for="credit in credits" :key="credit.role" class="about-row">
          <dt>{{ credit.role }}</dt>
          <dd>
            <n-button v-if="credit.url" text @click="openUrl(credit.url)">
              {{ credit.names }}
              <template #icon>
                <ExternalLink :size="14" :stroke-width="1.8" />
              </template>
            </n-button>
            <template v-else>{{ credit.names }}</template>
          </dd>
        </div>
      </dl>
    </section>

    <!-- ~ 免责声明 -->
    <section class="about-section" aria-labelledby="about-disclaimer-title">
      <h2 id="about-disclaimer-title" class="about-section-title">免责声明</h2>
      <p class="about-disclaimer">本工具仅提供模组安装便利，模组版权归原作者所有。使用模组可能影响游戏稳定性，请自行承担风险。</p>
    </section>
  </section>
</template>

<style scoped lang="scss">
// 内容区宽于 --ds-page-max-width 时水平居中，与全局设置页保持相同的页面边界
.about-page {
  width: 100%;
  max-width: var(--ds-page-max-width);
  margin-inline: auto;
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
}

.about-title {
  margin: 0 0 20px;
  color: var(--ds-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.3;
}

.about-section {
  margin-top: 32px;
}

.about-section-title {
  margin: 0 0 8px;
  color: var(--ds-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.about-list {
  margin: 0;
  border-top: 1px solid var(--ds-border-subtle);
}

.about-row {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  min-height: 44px;
  padding: 8px 0;
  border-bottom: 1px solid var(--ds-border-subtle);

  dt {
    color: var(--ds-text-muted);
    font-size: 12px;
  }

  dd {
    margin: 0;
    color: var(--ds-text-primary);
    font-size: 13px;
    line-height: 1.6;
    overflow-wrap: anywhere;
  }
}

.about-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
}

.about-mono {
  font-family: var(--ds-font-mono);
}

.about-disclaimer {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 1.7;
}

@media (max-width: 560px) {
  .about-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 4px;
  }
}
</style>
