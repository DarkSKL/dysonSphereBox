<script setup lang="ts">
import {Camera, Eye, EyeOff, LogOut, Mail, Pencil, Phone, RefreshCw, Settings, Shield, User, X} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref} from 'vue'
import {storeToRefs} from 'pinia'
import {account} from '../../wailsjs/go/models'
import {ChangePassword, GetCaptcha, UpdateSelfInfo, UploadAvatar} from '../../wailsjs/go/account/AccountService'
import {useAccountStore} from '../stores/account'

definePage({meta: {layout: 'main', title: '个人'}})

type ProfileView = 'login' | 'force-change-password' | 'profile'

const message = useMessage()
const accountStore = useAccountStore()
const {authenticated, needChangePassword, profile} = storeToRefs(accountStore)
const view = ref<ProfileView>(authenticated.value ? (needChangePassword.value ? 'force-change-password' : 'profile') : 'login')
const captcha = ref<account.CaptchaData>({captchaId: '', picPath: '', captchaLength: 0, openCaptcha: false})
const loginUsername = ref('')
const loginPassword = ref('')
const loginCaptcha = ref('')
const forcePassword = ref('')
const forcePasswordConfirm = ref('')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const profileDraft = ref({nickName: '', phone: '', email: '', headerImg: ''})
const settingsVisible = ref(false)
const editingName = ref(false)
const nicknameDraft = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)
const showLoginPassword = ref(false)
const showForcePassword = ref(false)
const showForceConfirm = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const isLoading = ref(false)
const isSavingProfile = ref(false)
const isChangingPassword = ref(false)

const displayName = computed(() => profile.value.nickName || profile.value.userName || '未命名用户')
const avatarSource = computed(() => profile.value.headerImg)
const accountState = computed(() => profile.value.enable === 1 ? '账号正常' : '账号状态未知')

// =============== 账号会话初始化 ===============
// 说明：Pinia 已在应用挂载前从后端 SQLite 恢复资料，本页只决定视图并后台刷新。
// 边界：刷新失败时保留已缓存资料，只有失效会话才回到登录页。
const initializeSession = async (): Promise<void> => {
  if (!authenticated.value) return
  if (needChangePassword.value) { view.value = 'force-change-password'; return }
  view.value = 'profile'
  try { await refreshUserInfo() } catch (error) { message.warning(`资料刷新失败，正在使用本地缓存：${formatError(error)}`) }
}

// =============== 验证码与登录 ===============
// 说明：验证码完全由服务器生成；登录只提交服务器给出的 captchaId。
// 边界：验证码未开启时不显示输入框，登录失败后刷新服务器验证码，避免使用本地模拟验证码。
const refreshCaptcha = async (): Promise<void> => {
  try { captcha.value = await GetCaptcha(); loginCaptcha.value = '' } catch (error) { message.error(`获取验证码失败：${formatError(error)}`) }
}

const submitLogin = async (): Promise<void> => {
  if (isLoading.value) return
  if (!loginUsername.value.trim() || !loginPassword.value) { message.warning('请输入用户名和密码'); return }
  if (captcha.value.openCaptcha && !loginCaptcha.value.trim()) { message.warning('请输入验证码'); return }
  isLoading.value = true
  try {
    await accountStore.login({username: loginUsername.value.trim(), password: loginPassword.value, captchaId: captcha.value.captchaId, captcha: loginCaptcha.value.trim()})
    loginCaptcha.value = ''
    if (needChangePassword.value) { view.value = 'force-change-password'; return }
    loginPassword.value = ''
    if (!await refreshUserInfo()) return
    view.value = 'profile'
    message.success('登录成功')
  } catch (error) { message.error(formatError(error)); await refreshCaptcha() } finally { isLoading.value = false }
}

const refreshUserInfo = async (): Promise<boolean> => {
  // 说明：401 表示后端已使本地 JWT 失效，此时回到登录页属于正常状态迁移。
  // 边界：除 401 和强制改密外的网络、服务端错误仍交给调用方提示，避免吞掉真实故障。
  try { await accountStore.refreshProfile() } catch (error) {
    const text = formatError(error)
    if (text.includes('HTTP 401')) {
      resetToLogin()
      return false
    }
    if (text.includes('10001') || text.includes('409')) view.value = 'force-change-password'
    throw error
  }
  return true
}

// =============== 资料保存与头像上传 ===============
// 说明：头像先走上传接口获取 URL，再随个人资料一次提交；其余字段直接保存。
// 边界：上传或保存失败时保留当前编辑值和旧头像，用户可修正后重试。
const openSettings = (): void => { profileDraft.value = {nickName: profile.value.nickName, phone: profile.value.phone, email: profile.value.email, headerImg: profile.value.headerImg}; settingsVisible.value = true }
const saveProfile = async (): Promise<void> => {
  if (isSavingProfile.value) return
  isSavingProfile.value = true
  try { await UpdateSelfInfo(profileDraft.value); await refreshUserInfo(); settingsVisible.value = false; message.success('个人资料已保存') } catch (error) { message.error(formatError(error)) } finally { isSavingProfile.value = false }
}
const saveNickname = async (): Promise<void> => { profileDraft.value = {nickName: nicknameDraft.value.trim(), phone: profile.value.phone, email: profile.value.email, headerImg: profile.value.headerImg}; if (!profileDraft.value.nickName) { message.warning('昵称不能为空'); return }; await saveProfile(); editingName.value = false }
const selectAvatar = (): void => avatarInput.value?.click()
const updateAvatar = async (event: Event): Promise<void> => {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  isSavingProfile.value = true
  try {
    const contentBase64 = await readFileBase64(file)
    const headerImg = await UploadAvatar(file.name, file.type, contentBase64)
    await UpdateSelfInfo({nickName: profile.value.nickName, phone: profile.value.phone, email: profile.value.email, headerImg})
    await refreshUserInfo()
    message.success('头像已更新')
  } catch (error) { message.error(formatError(error)) } finally { isSavingProfile.value = false; (event.target as HTMLInputElement).value = '' }
}

// =============== 密码与注销 ===============
// 说明：改密成功后服务端会清理本地会话，页面统一回到登录态。
// 边界：确认密码只在前端比较，绝不作为 API 字段提交。
const changePassword = async (forced = false): Promise<void> => {
  const oldPassword = forced ? loginPassword.value : currentPassword.value
  const nextPassword = forced ? forcePassword.value : newPassword.value
  const confirm = forced ? forcePasswordConfirm.value : confirmPassword.value
  if (!oldPassword || !nextPassword) { message.warning(forced ? '请输入当前密码和新密码' : '请输入当前密码和新密码'); return }
  if (nextPassword !== confirm) { message.warning('两次输入的新密码不一致'); return }
  isChangingPassword.value = true
  try { await ChangePassword({password: oldPassword, newPassword: nextPassword}); resetToLogin(); message.success('密码修改成功，请重新登录') } catch (error) { message.error(formatError(error)) } finally { isChangingPassword.value = false }
}
const logOut = async (): Promise<void> => { try { await accountStore.logout() } catch (error) { message.warning(formatError(error)) } finally { resetToLogin() } }
const resetToLogin = (): void => { accountStore.reset(); settingsVisible.value = false; view.value = 'login'; loginPassword.value = ''; loginCaptcha.value = ''; forcePassword.value = ''; forcePasswordConfirm.value = ''; currentPassword.value = ''; newPassword.value = ''; confirmPassword.value = ''; void refreshCaptcha() }
const readFileBase64 = (file: File): Promise<string> => new Promise((resolve, reject) => { const reader = new FileReader(); reader.onload = () => { const value = typeof reader.result === 'string' ? reader.result.split(',')[1] : ''; value ? resolve(value) : reject(new Error('无法读取头像文件')) }; reader.onerror = () => reject(reader.error ?? new Error('读取头像文件失败')); reader.readAsDataURL(file) })
const formatError = (error: unknown): string => error instanceof Error ? error.message : String(error || '未知错误')

onMounted(() => { void initializeSession(); void refreshCaptcha() })
</script>

<template>
  <section class="profile-page">
    <!-- ~ 登录模块 --><div v-if="view === 'login'" class="login-view"><div class="login-panel"><div class="login-intro"><User :size="24" /><h1>账号登录</h1><p>使用国内服务器账号登录</p></div><div class="form-fields"><label><span>用户名</span><n-input v-model:value="loginUsername" @keyup.enter="submitLogin" /></label><label><span>密码</span><n-input v-model:value="loginPassword" :type="showLoginPassword ? 'text' : 'password'" @keyup.enter="submitLogin"><template #suffix><button type="button" class="field-icon-button" @click="showLoginPassword = !showLoginPassword"><EyeOff v-if="showLoginPassword" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><label v-if="captcha.openCaptcha"><span>验证码</span><div class="captcha-row"><n-input v-model:value="loginCaptcha" :maxlength="captcha.captchaLength" @keyup.enter="submitLogin" /><button class="captcha" type="button" title="刷新验证码" @click="refreshCaptcha"><img :src="captcha.picPath" alt="验证码" /><RefreshCw :size="14" /></button></div></label><n-button type="primary" block :loading="isLoading" @click="submitLogin">登录</n-button></div></div></div>
    <!-- ~ 强制改密模块 --><div v-else-if="view === 'force-change-password'" class="login-view"><div class="login-panel"><div class="login-intro"><Shield :size="24" /><h1>修改密码</h1><p>请先修改密码后继续使用账号</p></div><div class="form-fields"><label><span>当前密码</span><n-input v-model:value="loginPassword" type="password" /></label><label><span>新密码</span><n-input v-model:value="forcePassword" :type="showForcePassword ? 'text' : 'password'"><template #suffix><button type="button" class="field-icon-button" @click="showForcePassword = !showForcePassword"><EyeOff v-if="showForcePassword" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><label><span>确认新密码</span><n-input v-model:value="forcePasswordConfirm" :type="showForceConfirm ? 'text' : 'password'" @keyup.enter="() => changePassword(true)"><template #suffix><button type="button" class="field-icon-button" @click="showForceConfirm = !showForceConfirm"><EyeOff v-if="showForceConfirm" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><n-button type="primary" block :loading="isChangingPassword" @click="() => changePassword(true)">确认修改</n-button></div></div></div>
    <!-- ~ 个人资料模块 --><div v-else class="profile-view"><section class="profile-hero"><div class="avatar-wrap" :class="{saving: isSavingProfile}" @click="selectAvatar"><img v-if="avatarSource" :src="avatarSource" alt="个人头像" /><User v-else :size="32" /><span class="avatar-cover"><Camera :size="16" /></span><input ref="avatarInput" class="hidden" type="file" accept="image/*" @change="updateAvatar" /></div><div class="profile-summary"><div class="profile-name-row"><template v-if="editingName"><n-input v-model:value="nicknameDraft" size="small" @keyup.enter="saveNickname" /><n-button size="small" type="primary" @click="saveNickname">保存</n-button><button class="icon-button" type="button" @click="editingName = false"><X :size="15" /></button></template><template v-else><h1>{{ displayName }}</h1><button class="icon-button" type="button" @click="nicknameDraft = profile.nickName; editingName = true"><Pencil :size="15" /></button></template></div><div class="badge-row"><span>{{ profile.userName }}</span><span>{{ accountState }}</span></div><div class="profile-meta"><span>邮箱：<b>{{ profile.email || '未填写' }}</b></span><span>手机号：<b>{{ profile.phone || '未填写' }}</b></span></div></div><div class="profile-actions"><n-button secondary @click="openSettings"><template #icon><Settings :size="15" /></template>设置</n-button><n-button secondary type="error" @click="logOut"><template #icon><LogOut :size="15" /></template>退出</n-button></div></section></div>
    <!-- ~ 设置抽屉 --><n-drawer v-model:show="settingsVisible" :width="400" placement="right"><n-drawer-content title="账号设置" closable><div class="settings-body"><section><h3>个人资料</h3><label><span>昵称</span><n-input v-model:value="profileDraft.nickName" /></label><label><span><Mail :size="14" />邮箱</span><n-input v-model:value="profileDraft.email" /></label><label><span><Phone :size="14" />手机号</span><n-input v-model:value="profileDraft.phone" /></label><n-button type="primary" :loading="isSavingProfile" @click="saveProfile">保存资料</n-button></section><section><h3><Shield :size="14" />账号安全</h3><label><span>当前密码</span><n-input v-model:value="currentPassword" :type="showCurrentPassword ? 'text' : 'password'"><template #suffix><button type="button" class="field-icon-button" @click="showCurrentPassword = !showCurrentPassword"><EyeOff v-if="showCurrentPassword" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><label><span>新密码</span><n-input v-model:value="newPassword" :type="showNewPassword ? 'text' : 'password'"><template #suffix><button type="button" class="field-icon-button" @click="showNewPassword = !showNewPassword"><EyeOff v-if="showNewPassword" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><label><span>确认新密码</span><n-input v-model:value="confirmPassword" :type="showConfirmPassword ? 'text' : 'password'" @keyup.enter="() => changePassword(false)"><template #suffix><button type="button" class="field-icon-button" @click="showConfirmPassword = !showConfirmPassword"><EyeOff v-if="showConfirmPassword" :size="15" /><Eye v-else :size="15" /></button></template></n-input></label><n-button secondary type="warning" :loading="isChangingPassword" @click="() => changePassword(false)">修改密码</n-button></section></div></n-drawer-content></n-drawer>
  </section>
</template>

<style scoped lang="scss">
.profile-page { min-height: 100%; padding: 20px 32px 32px; color: var(--ds-text-primary); }
.login-view { display: grid; min-height: calc(100vh - 128px); place-items: center; }
.login-panel { width: min(100%, 390px); border: 1px solid var(--ds-border-default); background: var(--ds-bg-panel); padding: 28px; }
.login-intro { display: grid; justify-items: center; gap: 8px; margin-bottom: 26px; text-align: center; color: var(--ds-brand); }
.login-intro h1 { margin: 0; color: var(--ds-text-primary); font-size: 20px; }.login-intro p { margin: 0; color: var(--ds-text-muted); font-size: 13px; }
.form-fields, .settings-body, .settings-body section { display: grid; gap: 16px; }.form-fields label, .settings-body label { display: grid; gap: 7px; color: var(--ds-text-secondary); font-size: 13px; }.settings-body h3 { display: flex; align-items: center; gap: 6px; margin: 6px 0 0; font-size: 14px; }
.captcha-row { display: grid; grid-template-columns: 1fr 124px; gap: 8px; }.captcha { display: flex; align-items: center; justify-content: space-around; border: 1px solid var(--ds-border-default); background: var(--ds-bg-panel); color: var(--ds-brand); cursor: pointer; }.captcha img { max-width: 88px; height: 30px; object-fit: contain; }
.profile-view { max-width: 880px; margin: 0 auto; }.profile-hero { display: flex; align-items: center; gap: 16px; border-bottom: 1px solid var(--ds-border-default); padding: 16px 0 20px; }.avatar-wrap { position: relative; display: grid; width: 72px; height: 72px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid var(--ds-border-default); border-radius: 50%; background: var(--ds-bg-panel); cursor: pointer; color: var(--ds-text-muted); }.avatar-wrap img { width: 100%; height: 100%; object-fit: cover; }.avatar-cover { position: absolute; inset: 0; display: grid; place-items: center; background: rgb(0 0 0 / .45); color: #fff; opacity: 0; transition: opacity .15s ease; }.avatar-wrap:hover .avatar-cover { opacity: 1; }.avatar-wrap.saving { pointer-events: none; opacity: .65; }.profile-summary { min-width: 0; flex: 1; }.profile-name-row, .badge-row, .profile-meta, .profile-actions { display: flex; align-items: center; gap: 8px; }.profile-name-row h1 { margin: 0; font-size: 20px; }.badge-row { margin-top: 7px; color: var(--ds-text-muted); font-size: 12px; }.badge-row span { border: 1px solid var(--ds-border-default); padding: 2px 6px; }.profile-meta { flex-wrap: wrap; margin-top: 12px; color: var(--ds-text-muted); font-size: 12px; }.profile-meta b { color: var(--ds-text-secondary); font-weight: 500; }.profile-actions { align-self: flex-start; }.profile-actions :deep(.n-button) { font-size: 12px; }.icon-button, .field-icon-button { display: grid; border: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; place-items: center; }.hidden { display: none; }
@media (max-width: 720px) { .profile-page { padding: 16px 20px 20px; }.profile-hero { align-items: flex-start; flex-wrap: wrap; }.avatar-wrap { width: 64px; height: 64px; }.profile-actions { width: 100%; }.profile-actions :deep(.n-button) { flex: 1; } }
</style>
