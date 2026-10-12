<script setup lang="ts">
import {ArrowRight, Eye, EyeOff, KeyRound, Link2, LockKeyhole, LogOut, Mail, PencilLine, Phone, RefreshCw, UserRound, X} from '@lucide/vue'
import {computed, nextTick, onBeforeUnmount, reactive, ref, watch} from 'vue'
import {useMessage} from 'naive-ui'
import {
  accountIsAuthenticated,
  accountNeedsPasswordChange,
  accountUser,
  changeAccountPassword,
  fetchCaptcha,
  fetchCurrentUser,
  loginAccount,
  logoutAccount,
  registerAccount,
  updateSelfInfo,
  type CaptchaInfo,
} from '../../services/account'
import {loadServerAddress, serverAddressReady, serverEffectiveAddress} from '../../services/server'

interface AccountModalProps {
  readonly open: boolean
}

const props = defineProps<AccountModalProps>()
const emit = defineEmits<{close: []}>()
const message = useMessage()
const mode = ref<'login' | 'register' | 'changePassword' | 'editProfile'>('login')
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const newPassword = ref('')
const captchaValue = ref('')
const captchaInfo = ref<CaptchaInfo | null>(null)
const isSubmitting = ref(false)
const isRefreshingCaptcha = ref(false)
const showPassword = ref(false)
const firstField = ref<HTMLInputElement | null>(null)
const requiresCaptcha = computed(() => mode.value === 'register' || (mode.value === 'login' && (!captchaInfo.value || captchaInfo.value.openCaptcha)))
const isAuthenticated = accountIsAuthenticated
// 已登录时 login/register 均视为账号信息视图；改密与编辑资料是从账号信息进入的子视图
const isShowingProfile = computed(() => isAuthenticated.value && (mode.value === 'login' || mode.value === 'register'))
const user = accountUser
const formTransitionName = computed(() => mode.value === 'login' ? 'account-form-backward' : 'account-form-forward')
const formKicker = computed(() => isShowingProfile.value ? '04' : mode.value === 'login' ? '01' : mode.value === 'register' ? '02' : mode.value === 'changePassword' ? '03' : '05')
const titleId = computed(() => isShowingProfile.value ? 'account-profile-title' : mode.value === 'login' ? 'account-login-title' : mode.value === 'register' ? 'account-register-title' : mode.value === 'changePassword' ? 'account-password-title' : 'account-edit-title')
const displayName = computed(() => user.value?.nickName || user.value?.userName || '')
// 自愿改密可返回账号信息；登录后被要求强制改密时不提供返回入口
const canLeavePasswordChange = computed(() => isAuthenticated.value && !accountNeedsPasswordChange.value)
// 改密校验“不含用户名”时，恢复会话后登录输入框为空，改用当前用户的登录名
const passwordAccountName = computed(() => username.value.trim() || user.value?.userName || '')

// =============== 账号资料 ===============
// 说明：资料草稿只在进入编辑时从当前用户复制；头像加载失败回退为图标，头像地址变化后重新尝试。
const profileKeys = ['nickName', 'headerImg', 'email', 'phone'] as const
type ProfileKey = typeof profileKeys[number]
const profileDraft = reactive<Record<ProfileKey, string>>({nickName: '', headerImg: '', email: '', phone: ''})
const avatarFailed = ref(false)
const isLoadingUser = ref(false)
const needsUserRefresh = ref(false)
// 相对头像路径属于账号服务器，不能落到 Wails 本地资源根；地址加载前暂用图标，不发送 token。
const avatarSource = computed(() => {
  const raw = user.value?.headerImg?.trim() ?? ''
  if (!raw || !serverAddressReady.value || raw.startsWith('//')) return ''
  try {
    const url = new URL(raw, `${serverEffectiveAddress.value}/`)
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch {
    return ''
  }
})
watch(avatarSource, () => { avatarFailed.value = false })
// 会话在编辑资料时失效（401、切换服务器），回到登录表单
watch(isAuthenticated, (authenticated) => {
  if (!authenticated && (mode.value === 'editProfile' || mode.value === 'changePassword')) {
    mode.value = 'login'
    password.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    void refreshCaptcha()
  }
})
// 受保护接口返回 409/10001 时立即切到改密视图，恢复会话和资料编辑共用此入口。
watch(accountNeedsPasswordChange, (required) => {
  if (required && isAuthenticated.value) mode.value = 'changePassword'
})
const passwordUpdatedText = computed(() => {
  const raw = user.value?.passwordUpdatedAt
  if (!raw) return '—'
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? raw : date.toLocaleString('zh-CN', {hour12: false})
})
// 品牌区换位动画：切到注册时向左移出、从右侧移入；回到登录时向右移出、从左侧移入。
// 为空表示不播放，弹窗每次打开时重置，避免挂载时重放上一次的换位动画。
const artMotion = ref<'to-login' | 'to-register' | null>(null)
const circuitTraces = [
  {d: 'M -10 90 H 72 V 132 H 165', end: [165, 132], duration: 5.2, delay: 0},
  {d: 'M -10 210 H 52 V 178 H 143 V 154 H 210', end: [210, 154], duration: 6.1, delay: 1.2},
  {d: 'M -10 330 H 86 V 294 H 160 V 260 H 218', end: [218, 260], duration: 5.7, delay: 2.1},
  {d: 'M 10 470 H 72 V 428 H 142 V 396 H 206', end: [206, 396], duration: 6.4, delay: 0.8},
] as const

// =============== 弹窗生命周期 ===============
// 说明：管理 Escape 关闭、验证码加载、首个输入框聚焦和页面滚动锁定。
const handleKeydown = (event: KeyboardEvent): void => {
  if (props.open && event.key === 'Escape') {
    emit('close')
  }
}

watch(
  () => props.open,
  async (open) => {
    document.body.classList.toggle('account-modal-open', open)
    if (open) {
      void loadServerAddress().catch(() => { avatarFailed.value = true })
      artMotion.value = null
      if (accountNeedsPasswordChange.value) mode.value = 'changePassword'
      // 重新打开时从账号信息开始，强制改密状态除外
      if (canLeavePasswordChange.value && (mode.value === 'editProfile' || mode.value === 'changePassword')) {
        mode.value = 'login'
      }
      if (!isAuthenticated.value && mode.value !== 'changePassword') {
        void refreshCaptcha()
      }
      if (isAuthenticated.value && (!user.value || needsUserRefresh.value)) {
        void reloadUser()
      }
      await nextTick()
      firstField.value?.focus()
    }
  },
  {immediate: true},
)

// 品牌区只在注册模式位于右侧，其余模式（登录、改密、账号信息）位于左侧；仅跨越两侧时播放换位动画
watch(mode, (next, prev) => {
  const wasRight = prev === 'register'
  const isRight = next === 'register'
  if (wasRight !== isRight) {
    artMotion.value = isRight ? 'to-register' : 'to-login'
  }
})

window.addEventListener('keydown', handleKeydown)
onBeforeUnmount(() => {
  document.body.classList.remove('account-modal-open')
  window.removeEventListener('keydown', handleKeydown)
})

// =============== 模式切换与验证码 ===============
// 说明：登录和注册共用用户名、密码与一次性验证码状态；切换时重新获取验证码。
const toggleMode = (): void => {
  mode.value = mode.value === 'login' ? 'register' : 'login'
  showPassword.value = false
  password.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  captchaValue.value = ''
  void refreshCaptcha()
}

async function refreshCaptcha(): Promise<void> {
  isRefreshingCaptcha.value = true
  captchaInfo.value = null
  captchaValue.value = ''
  try {
    captchaInfo.value = await fetchCaptcha()
  } catch (error) {
    message.error(`验证码加载失败：${error instanceof Error ? error.message : String(error)}`)
  } finally {
    isRefreshingCaptcha.value = false
  }
}

// =============== 表单提交 ===============
// 说明：按接口规范执行注册、登录、强制改密和退出；已提交的验证码不会重复使用。
const submit = async (): Promise<void> => {
  if (isSubmitting.value) return
  const accountName = username.value.trim()
  if (mode.value !== 'changePassword' && !accountName) {
    message.warning('请输入用户名。')
    return
  }
  if (mode.value !== 'changePassword' && !password.value) {
    message.warning('请输入密码。')
    return
  }
  if (mode.value === 'changePassword' && !password.value) {
    message.warning('请输入当前密码。')
    return
  }
  if (mode.value === 'register' && !/^[A-Za-z][A-Za-z0-9_]{3,31}$/.test(accountName)) {
    message.warning('用户名需以字母开头，长度为 4 至 32 位字母、数字或下划线。')
    return
  }
  if (mode.value === 'register' && ['admin', 'root', 'system', 'test'].includes(accountName.toLowerCase())) {
    message.warning('该用户名为系统保留，请更换。')
    return
  }
  if (mode.value === 'register' && password.value !== confirmPassword.value) {
    message.warning('两次输入的密码不一致。')
    return
  }
  if (mode.value === 'register' && !isPasswordLocallyValid(password.value, accountName)) {
    message.warning('密码至少 8 位，且需包含字母、数字、符号中的两类，不含空白字符或用户名。')
    return
  }
  if (mode.value === 'changePassword' && newPassword.value !== confirmPassword.value) {
    message.warning('两次输入的新密码不一致。')
    return
  }
  if (mode.value === 'changePassword' && !isPasswordLocallyValid(newPassword.value, passwordAccountName.value)) {
    message.warning('新密码至少 8 位，且需包含字母、数字、符号中的两类，不含空白字符或用户名。')
    return
  }
  if (requiresCaptcha.value && (!captchaInfo.value || !captchaValue.value.trim())) {
    message.warning('请输入验证码。')
    return
  }

  const usedCaptcha = requiresCaptcha.value
  isSubmitting.value = true
  try {
    if (mode.value === 'register') {
      await registerAccount({
        username: accountName,
        password: password.value,
        captcha: captchaValue.value.trim(),
        captchaId: captchaInfo.value!.captchaId,
      })
      message.success('注册成功，请登录。')
      mode.value = 'login'
      password.value = ''
      confirmPassword.value = ''
      await refreshCaptcha()
      return
    }

    if (mode.value === 'changePassword') {
      await changeAccountPassword(password.value, newPassword.value)
      try {
        await logoutAccount()
      } catch {
        // 密码已修改且本地会话已由 logoutAccount 清除，继续要求重新登录。
      }
      mode.value = 'login'
      password.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
      message.success('密码已修改，请重新登录。')
      await refreshCaptcha()
      return
    }

    const result = await loginAccount({
      username: accountName,
      password: password.value,
      ...(usedCaptcha && captchaInfo.value
        ? {captcha: captchaValue.value.trim(), captchaId: captchaInfo.value.captchaId}
        : {}),
    })
    if (result.needChangePassword) {
      mode.value = 'changePassword'
      newPassword.value = ''
      confirmPassword.value = ''
      message.warning('此账号需要先修改密码。')
      return
    }
    await fetchCurrentUser()
    message.success('登录成功。')
    emit('close')
  } catch (error) {
    if (accountNeedsPasswordChange.value) {
      mode.value = 'changePassword'
    } else {
      message.error(error instanceof Error ? error.message : String(error))
    }
    if (usedCaptcha) await refreshCaptcha()
  } finally {
    isSubmitting.value = false
  }
}

const isPasswordLocallyValid = (value: string, accountName: string): boolean => {
  const classes = [/[A-Za-z]/.test(value), /\d/.test(value), /[^A-Za-z0-9\s]/.test(value)]
  const commonPasswords = new Set(['password123', '12345678', 'admin123'])
  return new TextEncoder().encode(value).length <= 72
    && value.length >= 8
    && classes.filter(Boolean).length >= 2
    && !/\s/.test(value)
    && !(accountName && value.toLowerCase().includes(accountName.toLowerCase()))
    && !commonPasswords.has(value.toLowerCase())
}

// =============== 账号信息子视图 ===============
// 说明：编辑资料与自愿改密都从账号信息进入，返回时清空密码输入，不保留敏感草稿。
async function reloadUser(): Promise<void> {
  if (isLoadingUser.value) return
  isLoadingUser.value = true
  try {
    await fetchCurrentUser()
    needsUserRefresh.value = false
  } catch (error) {
    message.error(`账号信息读取失败：${error instanceof Error ? error.message : String(error)}`)
  } finally {
    isLoadingUser.value = false
  }
}

const startEditProfile = (): void => {
  const current = user.value
  if (!current) return
  for (const key of profileKeys) profileDraft[key] = current[key] ?? ''
  mode.value = 'editProfile'
}

const startChangePassword = (): void => {
  showPassword.value = false
  password.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  mode.value = 'changePassword'
}

const backToProfile = (): void => {
  if (isSubmitting.value) return
  showPassword.value = false
  password.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  mode.value = 'login'
}

// saveProfile 只提交变化字段；显式空字符串用于清除头像和联系方式，昵称保持必填。
const saveProfile = async (): Promise<void> => {
  if (isSubmitting.value) return
  const current = user.value
  const next = Object.fromEntries(profileKeys.map((key) => [key, profileDraft[key].trim()])) as Record<ProfileKey, string>
  const nickLength = Array.from(next.nickName).length
  if (!nickLength || nickLength > 32) {
    message.warning('昵称需填写 1 至 32 个字符。')
    return
  }
  // URL 和邮箱格式由服务端统一校验，避免客户端拒绝兼容的相对头像路径或合法邮箱。
  if (new TextEncoder().encode(next.headerImg).length > 2048) {
    message.warning('头像地址不能超过 2048 字节。')
    return
  }
  if (new TextEncoder().encode(next.email).length > 254) {
    message.warning('邮箱地址不能超过 254 字节。')
    return
  }
  if (next.phone && (next.phone.length > 32 || !/^[0-9+ ()-]+$/.test(next.phone) || !/[0-9]/.test(next.phone))) {
    message.warning('请输入有效的手机号。')
    return
  }
  const changes: Partial<Record<ProfileKey, string>> = {}
  for (const key of profileKeys) {
    if (next[key] !== (current?.[key] ?? '')) changes[key] = next[key]
  }
  if (!Object.keys(changes).length) {
    message.info('资料没有变化。')
    mode.value = 'login'
    return
  }

  isSubmitting.value = true
  try {
    await updateSelfInfo(changes)
    needsUserRefresh.value = true
    try {
      await fetchCurrentUser()
      needsUserRefresh.value = false
    } catch {
      // 资料已保存；读取失败时保留重试标记，避免展示旧资料却永久不再同步。
    }
    if (needsUserRefresh.value) message.warning('资料已保存，最新信息读取失败，请重新打开账号窗口重试。')
    else message.success('资料已更新。')
    mode.value = isAuthenticated.value && accountNeedsPasswordChange.value ? 'changePassword' : 'login'
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  } finally {
    isSubmitting.value = false
  }
}

const signOut = async (): Promise<void> => {
  try {
    await logoutAccount()
    message.success('已退出登录。')
    mode.value = 'login'
    await refreshCaptcha()
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}
</script>

<template>
  <!-- ~ 账号登录注册弹窗 -->
  <!-- * 说明：品牌区与表单区左右分栏；切换模式时品牌区从一侧移出卡片、再从另一侧移入，表单区平移到对侧并淡出淡入内容 -->
  <!-- * 交互：点击遮罩或关闭按钮退出，点击切换入口改变模式，Enter 提交当前表单，Escape 关闭弹窗 -->
  <!-- * 依赖：依赖 open、表单状态与 useMessage -->
  <!-- * 边界：只经 Wails 账号服务调用服务器，token 不进入前端；会话失效时返回登录表单 -->
  <Transition name="account-modal">
    <div v-if="open" class="account-modal-backdrop" @pointerdown.self="emit('close')">
      <section
        class="account-modal-card"
        :class="{'is-register': mode === 'register'}"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @pointerdown.stop
      >
        <button type="button" class="account-close" aria-label="关闭" title="关闭" @click="emit('close')">
          <X :size="18" :stroke-width="1.8" />
        </button>

        <!-- ~ 品牌区：渐变底色 + 电路脉冲线条 -->
        <div class="account-art" :class="artMotion ? `is-moving-${artMotion}` : ''" aria-hidden="true">
          <svg class="account-circuit" viewBox="0 0 320 580" preserveAspectRatio="xMidYMid slice">
            <g v-for="(trace, index) in circuitTraces" :key="index">
              <path class="account-circuit-trace" :d="trace.d" />
              <path
                class="account-circuit-pulse"
                :d="trace.d"
                pathLength="100"
                :style="{animationDuration: `${trace.duration}s`, animationDelay: `${trace.delay}s`}"
              />
              <circle
                class="account-circuit-node"
                :cx="trace.end[0]"
                :cy="trace.end[1]"
                r="2.5"
                :style="{animationDuration: `${trace.duration}s`, animationDelay: `${trace.delay}s`}"
              />
            </g>
          </svg>
          <div class="account-art-copy">
            <span class="account-art-kicker">DYSON SPHERE BOX</span>
            <strong>让每一次建造，<br />都有自己的轨道。</strong>
            <span class="account-art-caption">同步你的工程进度与本地资源</span>
          </div>
        </div>

        <!-- ~ 账号表单区 -->
        <div class="account-form-panel">
          <Transition :name="formTransitionName">
            <div :key="isShowingProfile ? 'profile' : mode" class="account-form-inner">
              <div class="account-form-heading">
                <span class="account-form-kicker">ACCOUNT / {{ formKicker }}</span>
                <h2 v-if="isShowingProfile" id="account-profile-title">账号信息</h2>
                <h2 v-else-if="mode === 'login'" id="account-login-title">欢迎回来</h2>
                <h2 v-else-if="mode === 'register'" id="account-register-title">创建账号</h2>
                <h2 v-else-if="mode === 'changePassword'" id="account-password-title">修改密码</h2>
                <h2 v-else id="account-edit-title">编辑资料</h2>
                <p v-if="isShowingProfile">管理你的工程师档案与登录安全。</p>
                <p v-else-if="mode === 'login'">使用用户名登录后继续管理你的戴森球工程。</p>
                <p v-else-if="mode === 'register'">建立你的工程师档案，保存每一次跃迁。</p>
                <p v-else-if="mode === 'changePassword'">设置新密码后需要重新登录。</p>
                <p v-else>昵称会在联机大厅与房间中显示。</p>
              </div>

              <!-- ~ 账号信息：头像、昵称与资料概览；编辑资料、修改密码与退出登录 -->
              <div v-if="isShowingProfile" class="account-profile">
                <div class="account-profile-card">
                  <span class="account-avatar" aria-hidden="true">
                    <img v-if="avatarSource && !avatarFailed" :src="avatarSource" alt="" @error="avatarFailed = true" />
                    <UserRound v-else :size="22" :stroke-width="1.8" />
                  </span>
                  <div class="account-profile-identity">
                    <strong :title="displayName">{{ displayName || (isLoadingUser ? '正在读取…' : '账号信息暂不可用') }}</strong>
                    <span v-if="user?.userName">@{{ user.userName }}</span>
                  </div>
                  <button v-if="!user" type="button" class="account-text-button" :disabled="isLoadingUser" @click="reloadUser">
                    <RefreshCw :size="15" :class="{'is-spinning': isLoadingUser}" />重试
                  </button>
                </div>

                <dl v-if="user" class="account-profile-list">
                  <div><dt><Mail :size="14" />邮箱</dt><dd>{{ user.email || '未填写' }}</dd></div>
                  <div><dt><Phone :size="14" />手机</dt><dd>{{ user.phone || '未填写' }}</dd></div>
                  <div><dt><KeyRound :size="14" />密码更新</dt><dd>{{ passwordUpdatedText }}</dd></div>
                </dl>

                <div class="account-profile-actions">
                  <button type="button" class="account-secondary" :disabled="!user" @click="startEditProfile">
                    <PencilLine :size="15" :stroke-width="1.8" /><span>编辑资料</span>
                  </button>
                  <button type="button" class="account-secondary" @click="startChangePassword">
                    <LockKeyhole :size="15" :stroke-width="1.8" /><span>修改密码</span>
                  </button>
                </div>
                <button type="button" class="account-submit account-signout" @click="signOut">
                  <span>退出登录</span>
                  <LogOut :size="16" :stroke-width="1.8" />
                </button>
              </div>

              <!-- ~ 编辑资料：只提交变化字段；留空可清除头像和联系方式，昵称必填 -->
              <form v-else-if="mode === 'editProfile'" class="account-form" @submit.prevent="saveProfile">
                <label class="account-field">
                  <span>昵称</span>
                  <span class="account-input-wrap">
                    <UserRound :size="16" :stroke-width="1.8" />
                    <input v-model="profileDraft.nickName" type="text" autocomplete="nickname" maxlength="32" placeholder="1 至 32 个字符" />
                  </span>
                </label>
                <label class="account-field">
                  <span>头像地址</span>
                  <span class="account-input-wrap">
                    <Link2 :size="16" :stroke-width="1.8" />
                    <input v-model="profileDraft.headerImg" type="text" autocomplete="photo" placeholder="https://… 或相对路径（可选）" />
                  </span>
                </label>
                <label class="account-field">
                  <span>邮箱</span>
                  <span class="account-input-wrap">
                    <Mail :size="16" :stroke-width="1.8" />
                    <input v-model="profileDraft.email" type="email" autocomplete="email" placeholder="可选" />
                  </span>
                </label>
                <label class="account-field">
                  <span>手机</span>
                  <span class="account-input-wrap">
                    <Phone :size="16" :stroke-width="1.8" />
                    <input v-model="profileDraft.phone" type="tel" autocomplete="tel" placeholder="可选" />
                  </span>
                </label>
                <div class="account-form-meta"><span>头像、邮箱和手机留空可清除，昵称必填</span></div>
                <div class="account-form-actions">
                  <button type="button" class="account-secondary" :disabled="isSubmitting" @click="backToProfile">返回</button>
                  <button type="submit" class="account-submit" :disabled="isSubmitting">
                    <span>保存资料</span>
                    <ArrowRight :size="17" :stroke-width="1.8" />
                  </button>
                </div>
              </form>

              <form v-else class="account-form" @submit.prevent="submit">
                <label v-if="mode !== 'changePassword'" class="account-field">
                  <span>用户名</span>
                  <span class="account-input-wrap">
                    <UserRound :size="16" :stroke-width="1.8" />
                    <input ref="firstField" v-model="username" type="text" autocomplete="username" placeholder="4 至 32 位字母、数字或下划线" />
                  </span>
                </label>

                <label class="account-field">
                  <span>{{ mode === 'changePassword' ? '当前密码' : '密码' }}</span>
                  <span class="account-input-wrap">
                    <LockKeyhole :size="16" :stroke-width="1.8" />
                    <input v-model="password" :type="showPassword ? 'text' : 'password'" :autocomplete="mode === 'register' ? 'new-password' : 'current-password'" :placeholder="mode === 'changePassword' ? '请输入当前密码' : '请输入密码'" />
                    <button type="button" class="account-input-action" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
                      <EyeOff v-if="showPassword" :size="16" :stroke-width="1.8" />
                      <Eye v-else :size="16" :stroke-width="1.8" />
                    </button>
                  </span>
                </label>

                <label v-if="mode === 'changePassword'" class="account-field">
                  <span>新密码</span>
                  <span class="account-input-wrap">
                    <LockKeyhole :size="16" :stroke-width="1.8" />
                    <input v-model="newPassword" type="password" autocomplete="new-password" placeholder="请输入新密码" />
                  </span>
                </label>

                <label v-if="mode === 'register' || mode === 'changePassword'" class="account-field">
                  <span>{{ mode === 'register' ? '确认密码' : '确认新密码' }}</span>
                  <span class="account-input-wrap">
                    <LockKeyhole :size="16" :stroke-width="1.8" />
                    <input v-model="confirmPassword" type="password" autocomplete="new-password" placeholder="再次输入密码" />
                  </span>
                </label>

                <div v-if="requiresCaptcha" class="account-captcha-row">
                  <label class="account-field account-captcha-field">
                    <span>验证码</span>
                    <span class="account-input-wrap">
                      <input v-model="captchaValue" type="text" inputmode="numeric" autocomplete="off" :maxlength="captchaInfo?.captchaLength || 6" placeholder="输入验证码" />
                    </span>
                  </label>
                  <button type="button" class="account-captcha-image" :disabled="isRefreshingCaptcha" title="点击刷新验证码" @click="refreshCaptcha">
                    <img v-if="captchaInfo?.picPath" :src="captchaInfo.picPath" alt="验证码，点击刷新" />
                    <RefreshCw v-else :size="18" :class="{'is-spinning': isRefreshingCaptcha}" />
                  </button>
                </div>

                <div class="account-form-meta">
                  <span v-if="mode === 'register'">注册验证码始终必填</span>
                  <span v-else-if="mode === 'login' && captchaInfo && !captchaInfo.openCaptcha">登录暂不要求验证码</span>
                  <span v-else>验证码仅可使用一次</span>
                </div>

                <div :class="{'account-form-actions': mode === 'changePassword' && canLeavePasswordChange}">
                  <button v-if="mode === 'changePassword' && canLeavePasswordChange" type="button" class="account-secondary" :disabled="isSubmitting" @click="backToProfile">返回</button>
                  <button type="submit" class="account-submit" :disabled="isSubmitting || isRefreshingCaptcha">
                    <span>{{ mode === 'login' ? '登录工程台' : mode === 'register' ? '创建工程师账号' : '修改密码并继续' }}</span>
                    <ArrowRight :size="17" :stroke-width="1.8" />
                  </button>
                </div>
              </form>

              <div v-if="!isShowingProfile && (mode === 'login' || mode === 'register')" class="account-switch">
                <span>{{ mode === 'login' ? '还没有账号？' : '已经有账号？' }}</span>
                <button type="button" class="account-switch-button" @click="toggleMode">
                  {{ mode === 'login' ? '立即注册' : '返回登录' }}
                </button>
              </div>
            </div>
          </Transition>
        </div>
      </section>
    </div>
  </Transition>
</template>

<style scoped lang="scss">
:global(body.account-modal-open) {
  overflow: hidden;
}

.account-modal-backdrop {
  position: fixed;
  z-index: 100;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(1 4 12 / 0.62);
  backdrop-filter: blur(10px);
}

// =============== 弹窗卡片与品牌区配色 ===============
// 说明：品牌区配色随主题切换；日间沿用石墨黑无彩色基调，夜间使用深空蓝底与炫蓝脉冲。
// 布局：卡片宽高在两种模式下完全一致（高度按注册表单确定），品牌区占 42cqw、表单区占 58cqw，
//       两者只通过 transform 对向平移换位，尺寸与形状不参与过渡，避免切换时拉伸变形。
.account-modal-card {
  --account-motion: 620ms cubic-bezier(0.65, 0, 0.35, 1);
  // 品牌区斜边的水平跨度
  --account-slant: 56px;
  --account-art-bg:
    radial-gradient(ellipse 80% 55% at 18% 12%, rgb(255 255 255 / 0.07), transparent 70%),
    linear-gradient(160deg, #2c2c2f 0%, #1c1c1e 55%, #111113 100%);
  --account-art-line: rgb(255 255 255 / 0.09);
  --account-art-pulse: rgb(245 245 245 / 0.75);
  --account-art-pulse-glow: rgb(255 255 255 / 0.35);
  --account-art-kicker: rgb(245 245 245 / 0.62);
  --account-art-text: #f5f5f5;
  --account-art-muted: rgb(245 245 245 / 0.6);

  position: relative;
  width: min(760px, calc(100vw - 32px));
  // 固定高度按最长的注册表单（用户名 + 两次密码 + 验证码）确定，所有模式共用，切换时卡片不缩放；
  // 窗口最小高度 768px，减去遮罩内边距后仍能完整容纳
  height: 540px;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 8px;
  background: var(--ds-bg-panel);
  box-shadow: 0 28px 80px rgb(0 0 0 / 0.36);
  isolation: isolate;
  // 作为尺寸容器，子元素用 cqw 按卡片宽度定位
  container-type: inline-size;
}

:root[data-theme='dark'] .account-modal-card {
  --account-art-bg:
    radial-gradient(ellipse 80% 55% at 18% 12%, rgb(76 194 255 / 0.14), transparent 70%),
    radial-gradient(ellipse 70% 50% at 90% 95%, rgb(122 108 255 / 0.12), transparent 70%),
    linear-gradient(160deg, #0b1a33 0%, #071225 55%, #050b17 100%);
  --account-art-line: rgb(125 212 255 / 0.13);
  --account-art-pulse: rgb(125 212 255 / 0.9);
  --account-art-pulse-glow: rgb(76 194 255 / 0.6);
  --account-art-kicker: var(--ds-brand);
  --account-art-text: #f3f6ff;
  --account-art-muted: rgb(217 227 255 / 0.66);
}

.account-close {
  position: absolute;
  z-index: 6;
  top: 14px;
  right: 14px;
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-primary);
  cursor: pointer;
  transition: color var(--account-motion), border-color var(--account-motion), background-color 160ms ease, transform 160ms ease;
}

.account-close:hover {
  border-color: var(--ds-border-strong);
  background: var(--ds-hover-bg);
  transform: rotate(4deg);
}

// 注册模式下关闭按钮位于品牌区之上，改用品牌区文字色
.is-register .account-close {
  border-color: rgb(255 255 255 / 0.16);
  color: var(--account-art-text);
}

.is-register .account-close:hover {
  border-color: rgb(255 255 255 / 0.36);
  background: rgb(255 255 255 / 0.08);
}

.account-art,
.account-form-panel {
  position: absolute;
  top: 0;
  bottom: 0;
}

.account-form-panel {
  transition: transform var(--account-motion);
}

// =============== 品牌区 ===============
// 说明：品牌区是固定形状的平行四边形（两条斜边同向），比可见槽位宽一个斜边跨度；
//       登录时左斜边伸出卡片左侧，注册时右斜边伸出卡片右侧，被卡片 overflow 裁掉。
//       切换时只做整体平移，clip-path 不变，靠近表单的斜边始终保持同一方向。
// 边界：斜边只裁掉品牌区自身，露出的三角区域为卡片底色，不侵入表单区。
.account-art {
  z-index: 2;
  left: calc(var(--account-slant) * -1);
  width: calc(42cqw + var(--account-slant));
  overflow: hidden;
  background: var(--account-art-bg);
  clip-path: polygon(var(--account-slant) 0, 100% 0, calc(100% - var(--account-slant)) 100%, 0 100%);
}

.is-register .account-art {
  transform: translateX(calc(58cqw + var(--account-slant)));
}

// 换位动画：先沿切换方向移出卡片，再从卡片另一侧移入到新槽位，不从表单区上方横穿
// 位置：-42cqw 为完全移出左侧，100cqw + 斜边跨度为完全移出右侧
.account-art.is-moving-to-register {
  animation: account-art-to-register 760ms both;
}

.account-art.is-moving-to-login {
  animation: account-art-to-login 760ms both;
}

.account-circuit {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  fill: none;
  mask-image: linear-gradient(180deg, #000 0%, #000 45%, transparent 78%);
}

.account-circuit-trace {
  stroke: var(--account-art-line);
  stroke-width: 1;
}

// 脉冲：pathLength 归一化为 100，短划从起点外 (-12) 走到终点外 (100)，剩余时间静止，形成间歇脉冲
.account-circuit-pulse {
  stroke: var(--account-art-pulse);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-dasharray: 12 100;
  stroke-dashoffset: 12;
  filter: drop-shadow(0 0 3px var(--account-art-pulse-glow));
  animation: account-circuit-pulse linear infinite both;
}

.account-circuit-node {
  fill: var(--account-art-pulse);
  opacity: 0.28;
  // 与对应走线的脉冲同周期，在脉冲抵达终点（55%）时点亮
  animation: account-circuit-node ease-in-out infinite both;
}

// 文案始终对齐卡片内的可见槽位：登录时槽位在品牌区内偏移一个斜边跨度，注册时偏移为 0
.account-art-copy {
  position: absolute;
  z-index: 1;
  bottom: 44px;
  left: 48px;
  display: grid;
  // 右侧额外让出一个斜边跨度，文案底部不会碰到斜边
  width: calc(42cqw - 96px - var(--account-slant));
  gap: 10px;
  color: var(--account-art-text);
  transform: translateX(var(--account-slant));
  // 在品牌区完全移出卡片的瞬间（换位动画 46% ≈ 350ms）切换偏移，移入时文案已在新位置
  transition: transform 0s 350ms;
}

.is-register .account-art-copy {
  transform: translateX(0);
}

.account-art-kicker {
  color: var(--account-art-kicker);
  font-family: var(--ds-font-display);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.2em;
}

.account-art-copy strong {
  font-size: 21px;
  font-weight: 650;
  line-height: 1.4;
  letter-spacing: 0.02em;
}

.account-art-caption {
  color: var(--account-art-muted);
  font-size: 12px;
}

// =============== 表单区 ===============
// 说明：新旧表单内容叠放在同一网格单元内交叉过渡，离场内容不占位，避免切换时跳动。
// 边界：卡片高度已按最长的注册表单确定，表单区不滚动，新增字段时需同步检查卡片高度
.account-form-panel {
  z-index: 1;
  display: grid;
  width: 58cqw;
  align-items: center;
  justify-items: center;
  padding: 26px 48px 22px;
  overflow: hidden;
  transform: translateX(42cqw);
}

.is-register .account-form-panel {
  transform: translateX(0);
}

.account-form-inner {
  grid-area: 1 / 1;
  width: min(320px, 100%);
}

.account-form-kicker {
  display: block;
  color: var(--ds-brand);
  font-size: 10px;
  font-family: var(--ds-font-display);
  font-weight: 700;
  letter-spacing: 0.2em;
  line-height: 1.2;
}

.account-form-heading h2 {
  margin: 8px 0 4px;
  color: var(--ds-text-primary);
  font-size: 25px;
  font-weight: 700;
  letter-spacing: 0.01em;
  line-height: 1.15;
}

.account-form-heading p {
  margin: 0;
  color: var(--ds-text-secondary);
  font-size: 12.5px;
  line-height: 1.5;
}

.account-form {
  display: grid;
  gap: 10px;
  margin-top: 18px;
}

.account-field {
  display: grid;
  gap: 5px;
  color: var(--ds-text-secondary);
  font-size: 11px;
  font-weight: 650;
  line-height: 1.4;
}

.account-input-wrap {
  display: flex;
  align-items: center;
  gap: 9px;
  height: 36px;
  padding: 0 11px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-muted);
  transition: border-color 160ms ease, box-shadow 160ms ease;
}

.account-input-wrap:focus-within {
  border-color: var(--ds-brand);
  box-shadow: 0 0 0 3px var(--ds-brand-glow);
}

.account-input-wrap input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--ds-text-primary);
  font: inherit;
  font-size: 13px;
}

.account-input-wrap > svg {
  flex: 0 0 16px;
}

.account-input-wrap input::placeholder {
  color: var(--ds-text-muted);
  opacity: 0.8;
}

.account-input-action,
.account-text-button,
.account-switch-button {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--ds-text-muted);
  cursor: pointer;
}

.account-input-action:hover,
.account-text-button:hover,
.account-switch-button:hover {
  color: var(--ds-brand);
}

.account-captcha-row {
  display: flex;
  align-items: end;
  gap: 10px;
}

.account-captcha-field {
  flex: 1;
}

.account-captcha-image {
  display: grid;
  width: 112px;
  height: 36px;
  flex: 0 0 112px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-muted);
  cursor: pointer;
}

.account-captcha-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.account-captcha-image:disabled,
.account-submit:disabled {
  cursor: wait;
  opacity: 0.65;
}

// =============== 账号信息与资料编辑 ===============
// 说明：资料概览使用扁平分隔线，次要操作并排、退出登录单独一行；高度需容纳在固定卡片内。
.account-profile {
  display: grid;
  gap: 14px;
  margin-top: 22px;
}

.account-profile-card {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.account-avatar {
  display: grid;
  width: 46px;
  height: 46px;
  flex: 0 0 46px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-muted);
}

.account-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.account-profile-identity {
  display: grid;
  min-width: 0;
  flex: 1;
  gap: 2px;
}

.account-profile-identity strong {
  overflow: hidden;
  color: var(--ds-text-primary);
  font-size: 15px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-profile-identity span {
  overflow: hidden;
  color: var(--ds-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-text-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font: inherit;
  font-size: 12px;
}

.account-profile-list {
  margin: 0;
  border-top: 1px solid var(--ds-border-subtle);
}

.account-profile-list > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
  border-bottom: 1px solid var(--ds-border-subtle);
  font-size: 12px;
}

.account-profile-list dt {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  gap: 7px;
  color: var(--ds-text-muted);
}

.account-profile-list dd {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--ds-text-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-profile-actions,
.account-form-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.account-form-actions {
  grid-template-columns: auto 1fr;
}

.account-secondary {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 40px;
  margin-top: 2px;
  padding: 0 16px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 650;
  transition: background-color 160ms ease, border-color 160ms ease;
}

.account-secondary:hover:not(:disabled) {
  border-color: var(--ds-border-strong);
  background: var(--ds-hover-bg);
}

.account-secondary:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.account-signout {
  width: 100%;
}

.is-spinning {
  animation: account-spin 900ms linear infinite;
}

@keyframes account-spin {
  to { transform: rotate(360deg); }
}

.account-form-meta,
.account-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--ds-text-muted);
  font-size: 11px;
  line-height: 1.45;
}

.account-check {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  cursor: pointer;
}

.account-check input {
  accent-color: var(--ds-brand);
}

.account-terms {
  max-width: 280px;
  line-height: 1.45;
}

.account-submit {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  min-height: 40px;
  margin-top: 2px;
  border: 1px solid var(--ds-brand);
  border-radius: 4px;
  background: var(--ds-brand);
  color: var(--ds-brand-contrast-text);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  transition: background-color 160ms ease, border-color 160ms ease, transform 160ms ease;
}

.account-submit:hover {
  border-color: var(--ds-brand-hover);
  background: var(--ds-brand-hover);
  transform: translateY(-1px);
}

.account-switch {
  justify-content: center;
  margin-top: 14px;
}

.account-switch-button {
  color: var(--ds-brand);
  font-weight: 700;
}

// =============== 过渡动画 ===============
.account-modal-enter-active,
.account-modal-leave-active {
  transition: opacity 260ms ease;
}

.account-modal-enter-from,
.account-modal-leave-to {
  opacity: 0;
}

// 表单内容：旧内容先快速离场，新内容在面板平移过半后进场
.account-form-forward-leave-active,
.account-form-backward-leave-active {
  transition: opacity 220ms ease-in, transform 220ms ease-in;
}

.account-form-forward-enter-active,
.account-form-backward-enter-active {
  transition: opacity 340ms ease-out 240ms, transform 340ms cubic-bezier(0.22, 1, 0.36, 1) 240ms;
}

.account-form-forward-leave-to,
.account-form-backward-enter-from {
  opacity: 0;
  transform: translateX(-24px);
}

.account-form-forward-enter-from,
.account-form-backward-leave-to {
  opacity: 0;
  transform: translateX(24px);
}

// 前半程加速移出，后半程在卡片另一侧外瞬移后减速移入
@keyframes account-art-to-register {
  0% {
    transform: translateX(0);
    animation-timing-function: cubic-bezier(0.55, 0, 0.9, 0.45);
  }
  46% { transform: translateX(-42cqw); }
  46.01% {
    transform: translateX(calc(100cqw + var(--account-slant)));
    animation-timing-function: cubic-bezier(0.1, 0.55, 0.3, 1);
  }
  100% { transform: translateX(calc(58cqw + var(--account-slant))); }
}

@keyframes account-art-to-login {
  0% {
    transform: translateX(calc(58cqw + var(--account-slant)));
    animation-timing-function: cubic-bezier(0.55, 0, 0.9, 0.45);
  }
  46% { transform: translateX(calc(100cqw + var(--account-slant))); }
  46.01% {
    transform: translateX(-42cqw);
    animation-timing-function: cubic-bezier(0.1, 0.55, 0.3, 1);
  }
  100% { transform: translateX(0); }
}

@keyframes account-circuit-pulse {
  0% { stroke-dashoffset: 12; }
  55%,
  100% { stroke-dashoffset: -100; }
}

@keyframes account-circuit-node {
  0%,
  45%,
  100% { opacity: 0.28; }
  55% { opacity: 0.8; }
}

@media (max-width: 700px) {
  .account-modal-backdrop {
    padding: 12px;
  }

  .account-modal-card {
    height: min(760px, calc(100vh - 24px));
  }

  .account-art,
  .is-register .account-art,
  .account-art.is-moving-to-register,
  .account-art.is-moving-to-login {
    right: 0;
    left: 0;
    animation: none;
    bottom: auto;
    width: 100%;
    height: 170px;
    transform: none;
    clip-path: polygon(0 0, 100% 0, 100% 78%, 0 100%);
  }

  .account-form-panel,
  .is-register .account-form-panel {
    top: 170px;
    right: 0;
    width: 100%;
    align-items: start;
    padding: 24px;
    transform: none;
  }

  .account-art-copy,
  .is-register .account-art-copy {
    right: 28px;
    bottom: 40px;
    left: 28px;
    width: auto;
    transform: none;
  }

  .account-art-copy strong {
    font-size: 18px;
  }

  .account-close,
  .is-register .account-close {
    border-color: rgb(255 255 255 / 0.16);
    color: var(--account-art-text);
  }
}

@media (prefers-reduced-motion: reduce) {
  .account-art,
  .account-art-copy,
  .account-form-panel,
  .account-close,
  .account-submit {
    transition: none;
  }

  .account-art.is-moving-to-register,
  .account-art.is-moving-to-login {
    animation: none;
  }

  .account-circuit-pulse {
    display: none;
    animation: none;
  }

  .account-circuit-node,
  .is-spinning {
    animation: none;
  }

  .account-form-forward-enter-active,
  .account-form-forward-leave-active,
  .account-form-backward-enter-active,
  .account-form-backward-leave-active {
    transition: opacity 160ms ease;
  }

  .account-form-forward-leave-to,
  .account-form-forward-enter-from,
  .account-form-backward-leave-to,
  .account-form-backward-enter-from {
    transform: none;
  }
}
</style>
