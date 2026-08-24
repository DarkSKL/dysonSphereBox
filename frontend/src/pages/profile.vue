<script setup lang="ts">
import {
  Camera,
  Check,
  Eye,
  EyeOff,
  LogOut,
  Mail,
  Pencil,
  Phone,
  RefreshCw,
  Settings,
  Shield,
  User,
  X,
} from '@lucide/vue'
import {computed, ref} from 'vue'

definePage({
  meta: {
    layout: 'main',
    title: '个人',
  },
})

type ProfileView = 'login' | 'force-change-password' | 'profile'
type Gender = 'male' | 'female' | 'unknown'

interface UserProfile {
  nickname: string
  level: number
  role: string
  bio: string
  gender: Gender
  region: string
  email: string
  phone: string
  modsInstalled: number
  blueprints: number
  saves: number
}

const captchaCharacters = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
const createCaptcha = (): string => Array.from(
  {length: 4},
  () => captchaCharacters[Math.floor(Math.random() * captchaCharacters.length)],
).join('')

const profile = ref<UserProfile>({
  nickname: 'DarkSKL_',
  level: 9,
  role: '普通用户',
  bio: '戴森球计划玩家',
  gender: 'male',
  region: '安徽省 宣城市',
  email: 'dark***@example.com',
  phone: '138****8888',
  modsInstalled: 5,
  blueprints: 8,
  saves: 4,
})

const view = ref<ProfileView>('login')
const settingsVisible = ref(false)
const loginUsername = ref('')
const loginPassword = ref('')
const loginCaptcha = ref('')
const captcha = ref(createCaptcha())
const showLoginPassword = ref(false)
const showForcePassword = ref(false)
const showForceConfirm = ref(false)
const forcePassword = ref('')
const forcePasswordConfirm = ref('')
const editingName = ref(false)
const nicknameDraft = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarUrl = ref('')
const avatarHover = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const profileDraft = ref({...profile.value})

const profileStats = computed(() => [
  {value: profile.value.modsInstalled, label: '已装 Mod'},
  {value: profile.value.blueprints, label: '蓝图'},
  {value: profile.value.saves, label: '存档'},
])

const refreshCaptcha = (): void => {
  captcha.value = createCaptcha()
  loginCaptcha.value = ''
}

const previewLogin = (): void => {
  view.value = loginPassword.value === 'force' ? 'force-change-password' : 'profile'
}

const enterProfile = (): void => {
  view.value = 'profile'
}

const openSettings = (): void => {
  profileDraft.value = {...profile.value}
  settingsVisible.value = true
}

const saveProfile = (): void => {
  profile.value = {...profileDraft.value}
  settingsVisible.value = false
}

const startNameEdit = (): void => {
  nicknameDraft.value = profile.value.nickname
  editingName.value = true
}

const saveNickname = (): void => {
  if (nicknameDraft.value.trim()) {
    profile.value.nickname = nicknameDraft.value.trim()
  }
  editingName.value = false
}

const selectAvatar = (): void => {
  avatarInput.value?.click()
}

const updateAvatar = (event: Event): void => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) {
    return
  }

  if (avatarUrl.value) {
    URL.revokeObjectURL(avatarUrl.value)
  }
  avatarUrl.value = URL.createObjectURL(file)
  input.value = ''
}

const logOutPreview = (): void => {
  settingsVisible.value = false
  view.value = 'login'
  loginUsername.value = ''
  loginPassword.value = ''
  loginCaptcha.value = ''
}
</script>

<template>
  <section class="profile-page">
    <div v-if="view === 'login'" class="login-view">
      <div class="login-panel">
        <div class="login-intro">
          <span class="login-icon"><User :size="21" :stroke-width="1.7" /></span>
          <h1>账号登录</h1>
          <p>登录后可管理个人资料</p>
        </div>
        <div class="form-fields">
          <label>
            <span>用户名</span>
            <n-input v-model:value="loginUsername" placeholder="请输入用户名" @keyup.enter="previewLogin" />
          </label>
          <label>
            <span>密码</span>
            <n-input v-model:value="loginPassword" :type="showLoginPassword ? 'text' : 'password'" placeholder="请输入密码" @keyup.enter="previewLogin">
              <template #suffix>
                <button class="field-icon-button" type="button" :title="showLoginPassword ? '隐藏密码' : '显示密码'" @click="showLoginPassword = !showLoginPassword">
                  <EyeOff v-if="showLoginPassword" :size="15" />
                  <Eye v-else :size="15" />
                </button>
              </template>
            </n-input>
          </label>
          <label>
            <span>验证码</span>
            <div class="captcha-row">
              <n-input v-model:value="loginCaptcha" placeholder="请输入验证码" maxlength="4" @keyup.enter="previewLogin" />
              <button class="captcha" type="button" title="刷新验证码" @click="refreshCaptcha">
                <span v-for="(character, index) in captcha.split('')" :key="`${character}-${index}`" :style="{transform: `rotate(${index % 2 === 0 ? -7 : 6}deg)`}">{{ character }}</span>
                <RefreshCw :size="12" :stroke-width="1.8" />
              </button>
            </div>
          </label>
          <n-button type="primary" block @click="previewLogin">登录</n-button>
        </div>
        <p class="login-hint">当前不支持短信、邮件验证及第三方登录</p>
      </div>
    </div>

    <div v-else-if="view === 'force-change-password'" class="login-view">
      <div class="login-panel">
        <div class="password-intro">
          <h1>修改初始密码</h1>
          <p>检测到您使用的是初始密码，请修改后继续访问</p>
        </div>
        <div class="form-fields">
          <label>
            <span>新密码</span>
            <n-input v-model:value="forcePassword" :type="showForcePassword ? 'text' : 'password'" placeholder="至少 8 位字符">
              <template #suffix>
                <button class="field-icon-button" type="button" :title="showForcePassword ? '隐藏密码' : '显示密码'" @click="showForcePassword = !showForcePassword">
                  <EyeOff v-if="showForcePassword" :size="15" />
                  <Eye v-else :size="15" />
                </button>
              </template>
            </n-input>
          </label>
          <label>
            <span>确认新密码</span>
            <n-input v-model:value="forcePasswordConfirm" :type="showForceConfirm ? 'text' : 'password'" placeholder="再次输入新密码" @keyup.enter="enterProfile">
              <template #suffix>
                <button class="field-icon-button" type="button" :title="showForceConfirm ? '隐藏密码' : '显示密码'" @click="showForceConfirm = !showForceConfirm">
                  <EyeOff v-if="showForceConfirm" :size="15" />
                  <Eye v-else :size="15" />
                </button>
              </template>
            </n-input>
          </label>
          <n-button type="primary" block @click="enterProfile">确认修改</n-button>
        </div>
      </div>
    </div>

    <div v-else class="profile-view">
      <section class="profile-hero">
        <div class="avatar-wrap" @mouseenter="avatarHover = true" @mouseleave="avatarHover = false" @click="selectAvatar">
          <img v-if="avatarUrl" :src="avatarUrl" alt="个人头像" />
          <User v-else :size="34" :stroke-width="1.35" />
          <span v-show="avatarHover" class="avatar-cover"><Camera :size="19" /></span>
          <input ref="avatarInput" class="hidden" type="file" accept="image/*" @change="updateAvatar" />
        </div>
        <div class="profile-summary">
          <div class="profile-name-row">
            <template v-if="editingName">
              <n-input v-model:value="nicknameDraft" size="small" class="nickname-input" @keyup.enter="saveNickname" @keyup.esc="editingName = false" />
              <button class="inline-icon-button" type="button" title="确认昵称" @click="saveNickname"><Check :size="15" /></button>
              <button class="inline-icon-button" type="button" title="取消编辑" @click="editingName = false"><X :size="15" /></button>
            </template>
            <template v-else>
              <h1>{{ profile.nickname }}</h1>
              <button class="inline-icon-button" type="button" title="编辑昵称" @click="startNameEdit"><Pencil :size="14" /></button>
            </template>
          </div>
          <div class="badge-row">
            <span>Lv.{{ profile.level }}</span>
            <span>{{ profile.role }}</span>
          </div>
          <div class="stats-row">
            <span v-for="item in profileStats" :key="item.label"><strong>{{ item.value }}</strong>{{ item.label }}</span>
          </div>
          <div class="profile-meta">
            <span>简介：<b>{{ profile.bio }}</b></span>
            <span>地区：<b>{{ profile.region }}</b></span>
            <span>邮箱：<b>{{ profile.email }}</b></span>
          </div>
        </div>
        <button class="settings-button" type="button" @click="openSettings"><Settings :size="15" />设置</button>
      </section>
    </div>

    <Transition name="settings-drawer">
      <div v-if="settingsVisible" class="settings-mask" @mousedown.self="settingsVisible = false">
        <aside class="settings-drawer">
          <header>
            <h2>账号设置</h2>
            <button class="drawer-close" type="button" title="关闭设置" @click="settingsVisible = false"><X :size="17" /></button>
          </header>
          <div class="settings-body">
            <section class="settings-section">
              <h3>个人资料</h3>
              <label><span>昵称</span><n-input v-model:value="profileDraft.nickname" size="small" /></label>
              <label><span>个人简介</span><n-input v-model:value="profileDraft.bio" size="small" placeholder="介绍一下自己" /></label>
              <label><span>性别</span></label>
              <n-radio-group v-model:value="profileDraft.gender" size="small" class="gender-group">
                <n-radio-button value="male">男</n-radio-button>
                <n-radio-button value="female">女</n-radio-button>
                <n-radio-button value="unknown">保密</n-radio-button>
              </n-radio-group>
              <label><span>地区</span><n-input v-model:value="profileDraft.region" size="small" placeholder="填写所在地区" /></label>
            </section>
            <section class="settings-section">
              <h3>联系方式</h3>
              <label><span><Mail :size="13" />邮箱</span><n-input v-model:value="profileDraft.email" size="small" /></label>
              <label><span><Phone :size="13" />手机号</span><n-input v-model:value="profileDraft.phone" size="small" /></label>
            </section>
            <n-button type="primary" size="small" @click="saveProfile">保存资料</n-button>
            <section class="settings-section">
              <h3><Shield :size="13" />账号安全</h3>
              <label><span>当前密码</span><n-input v-model:value="currentPassword" :type="showCurrentPassword ? 'text' : 'password'" size="small"><template #suffix><button class="field-icon-button" type="button" @click="showCurrentPassword = !showCurrentPassword"><EyeOff v-if="showCurrentPassword" :size="14" /><Eye v-else :size="14" /></button></template></n-input></label>
              <label><span>新密码</span><n-input v-model:value="newPassword" :type="showNewPassword ? 'text' : 'password'" size="small" placeholder="至少 8 位字符"><template #suffix><button class="field-icon-button" type="button" @click="showNewPassword = !showNewPassword"><EyeOff v-if="showNewPassword" :size="14" /><Eye v-else :size="14" /></button></template></n-input></label>
              <label><span>确认新密码</span><n-input v-model:value="confirmPassword" :type="showConfirmPassword ? 'text' : 'password'" size="small"><template #suffix><button class="field-icon-button" type="button" @click="showConfirmPassword = !showConfirmPassword"><EyeOff v-if="showConfirmPassword" :size="14" /><Eye v-else :size="14" /></button></template></n-input></label>
              <n-button size="small">确认修改</n-button>
            </section>
            <section class="settings-section danger-zone">
              <h3>危险操作</h3>
              <n-button type="error" ghost size="small" @click="logOutPreview"><template #icon><LogOut :size="14" /></template>注销登录</n-button>
            </section>
          </div>
        </aside>
      </div>
    </Transition>
  </section>
</template>

<style scoped lang="scss">
.profile-page { min-height: 100%; color: var(--ds-text-primary); }
.login-view { display: grid; min-height: calc(100vh - 4rem); place-items: center; padding: 32px; }
.login-panel { width: min(288px, 100%); }
.login-intro { margin-bottom: 24px; text-align: center; }
.login-icon { display: grid; width: 42px; height: 42px; margin: 0 auto 12px; place-items: center; border: 1px solid var(--ds-border-default); border-radius: 50%; color: var(--ds-text-muted); }
h1, h2, h3, p { margin: 0; }
.login-intro h1, .password-intro h1 { font-size: 15px; font-weight: 600; }
.login-intro p, .password-intro p, .login-hint { margin-top: 4px; color: var(--ds-text-muted); font-size: 12px; line-height: 1.6; }
.password-intro { margin-bottom: 24px; }
.form-fields { display: grid; gap: 12px; }
.form-fields label, .settings-section label { display: grid; gap: 5px; color: var(--ds-text-muted); font-size: 12px; }
.captcha-row { display: flex; gap: 8px; }
.captcha-row :deep(.n-input) { min-width: 0; flex: 1; }
.captcha { display: flex; width: 96px; height: 34px; align-items: center; justify-content: center; gap: 3px; overflow: hidden; border: 1px solid var(--ds-border-default); border-radius: 4px; background: #f6f6f6; color: #2563eb; font-family: monospace; font-size: 17px; font-weight: 700; letter-spacing: 2px; }
.captcha svg { margin-left: 2px; color: #71717a; }
.login-hint { text-align: center; }
.field-icon-button, .inline-icon-button, .drawer-close { display: grid; padding: 0; place-items: center; border: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.field-icon-button:hover, .inline-icon-button:hover, .drawer-close:hover { color: var(--ds-text-primary); }
.profile-view { padding: 0 32px; }
.profile-hero { display: flex; min-height: 148px; align-items: flex-start; gap: 20px; padding: 24px 0; border-bottom: 1px solid var(--ds-border-subtle); }
.avatar-wrap { position: relative; display: grid; width: 72px; height: 72px; flex: none; place-items: center; overflow: hidden; border: 1px solid var(--ds-border-default); border-radius: 50%; background: var(--ds-hover-bg); color: var(--ds-text-muted); cursor: pointer; }
.avatar-wrap img { width: 100%; height: 100%; object-fit: cover; }
.avatar-cover { position: absolute; inset: 0; display: grid; place-items: center; background: rgb(0 0 0 / 0.48); color: #fff; }
.profile-summary { min-width: 0; flex: 1; padding-top: 2px; }
.profile-name-row { display: flex; height: 24px; align-items: center; gap: 8px; }
.profile-name-row h1 { font-size: 15px; font-weight: 600; }
.nickname-input { width: 148px; }
.badge-row, .stats-row, .profile-meta { display: flex; flex-wrap: wrap; align-items: center; }
.badge-row { gap: 6px; margin-top: 7px; }
.badge-row span { border: 1px solid var(--ds-border-default); border-radius: 3px; padding: 2px 6px; color: var(--ds-text-muted); font-size: 10px; line-height: 1.1; }
.stats-row { gap: 18px; margin-top: 11px; font-size: 12px; }
.stats-row span { color: var(--ds-text-muted); }
.stats-row strong { margin-right: 4px; color: var(--ds-text-primary); font-weight: 600; }
.profile-meta { gap: 4px 18px; margin-top: 9px; color: var(--ds-text-muted); font-size: 12px; }
.profile-meta b { color: var(--ds-text-primary); font-weight: 400; }
.settings-button { display: inline-flex; align-items: center; gap: 6px; margin-left: auto; border: 1px solid var(--ds-border-default); border-radius: 4px; padding: 6px 10px; background: var(--ds-bg-panel); color: var(--ds-text-muted); font-size: 12px; cursor: pointer; }
.settings-button:hover { border-color: var(--ds-border-strong); color: var(--ds-text-primary); }
.settings-mask { position: fixed; inset: 0; z-index: 30; display: flex; justify-content: flex-end; background: rgb(0 0 0 / 0.12); }
.settings-drawer-enter-active, .settings-drawer-leave-active { transition: background-color 180ms ease; }
.settings-drawer-enter-active .settings-drawer, .settings-drawer-leave-active .settings-drawer { transition: transform 220ms cubic-bezier(.22, .61, .36, 1); }
.settings-drawer-enter-from, .settings-drawer-leave-to { background: rgb(0 0 0 / 0); }
.settings-drawer-enter-from .settings-drawer, .settings-drawer-leave-to .settings-drawer { transform: translateX(100%); }
.settings-drawer { display: flex; width: min(320px, 100vw); height: 100%; flex-direction: column; border-left: 1px solid var(--ds-border-default); background: var(--ds-bg-panel-elevated); box-shadow: -10px 0 28px rgb(0 0 0 / 0.08); }
.settings-drawer header { display: flex; height: 52px; align-items: center; justify-content: space-between; padding: 0 16px; border-bottom: 1px solid var(--ds-border-subtle); }
.settings-drawer h2 { font-size: 14px; font-weight: 600; }
.settings-body { display: grid; gap: 20px; overflow-y: auto; padding: 16px; }
.settings-section { display: grid; gap: 11px; border-top: 1px solid var(--ds-border-subtle); padding-top: 16px; }
.settings-section:first-child { border-top: 0; padding-top: 0; }
.settings-section h3 { display: flex; align-items: center; gap: 5px; color: var(--ds-text-muted); font-size: 10px; font-weight: 600; letter-spacing: .08em; text-transform: uppercase; }
.settings-section label > span { display: inline-flex; align-items: center; gap: 5px; }
.gender-group { display: flex; gap: 6px; }
.danger-zone { padding-bottom: 8px; }
@media (max-width: 640px) { .profile-view { padding: 0 20px; } .profile-hero { gap: 14px; } .settings-button { padding: 6px; font-size: 0; } .profile-meta { display: grid; } }
</style>
