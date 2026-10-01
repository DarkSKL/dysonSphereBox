import {useMessage} from 'naive-ui'
import {ref} from 'vue'

export interface EnvTextDialogState {
  visible: boolean
  title: string
  label: string
  value: string
  placeholder: string
  positiveText: string
  isSubmitting: boolean
  submit: ((value: string) => Promise<void>) | null
}

export interface EnvConfirmDialogState {
  visible: boolean
  title: string
  content: string
  positiveText: string
  isDanger: boolean
  isSubmitting: boolean
  submit: (() => Promise<void>) | null
}

export interface EnvQuickImportDialogState {
  visible: boolean
  name: string
  code: string
  isSubmitting: boolean
  submit: ((name: string, code: string) => Promise<void>) | null
}

export type EnvTextDialogConfig = Omit<EnvTextDialogState, 'visible' | 'isSubmitting'>
export type EnvConfirmDialogConfig = Omit<EnvConfirmDialogState, 'visible' | 'isSubmitting'>

// =============== 资源页弹窗状态 ===============
// 说明：集中维护单字段输入、危险操作确认和蓝图快速导入三类弹窗。
// 边界：弹窗只收集用户意图并调用 submit；提交失败时保持弹窗打开，错误提示由业务动作负责。
export const useEnvironmentDialogs = () => {
  const message = useMessage()

  const textDialog = ref<EnvTextDialogState>({
    visible: false,
    title: '',
    label: '',
    value: '',
    placeholder: '',
    positiveText: '保存',
    isSubmitting: false,
    submit: null,
  })

  const confirmDialog = ref<EnvConfirmDialogState>({
    visible: false,
    title: '',
    content: '',
    positiveText: '确认',
    isDanger: false,
    isSubmitting: false,
    submit: null,
  })

  const quickImportDialog = ref<EnvQuickImportDialogState>({
    visible: false,
    name: '',
    code: '',
    isSubmitting: false,
    submit: null,
  })

  const openTextDialog = (config: EnvTextDialogConfig): void => {
    textDialog.value = {...config, visible: true, isSubmitting: false}
  }

  const openConfirmDialog = (config: EnvConfirmDialogConfig): void => {
    confirmDialog.value = {...config, visible: true, isSubmitting: false}
  }

  const openQuickImportDialog = (submit: (name: string, code: string) => Promise<void>): void => {
    quickImportDialog.value = {visible: true, name: '', code: '', isSubmitting: false, submit}
  }

  // 说明：返回值供 Naive UI positive-click 使用，false 表示保持弹窗打开。
  const submitTextDialog = async (): Promise<boolean> => {
    const value = textDialog.value.value.trim()
    if (!value) {
      message.warning('名称不能为空。')
      return false
    }
    const submit = textDialog.value.submit
    if (!submit) {
      textDialog.value.visible = false
      return true
    }
    textDialog.value.isSubmitting = true
    try {
      await submit(value)
      textDialog.value.visible = false
      return true
    } catch {
      return false
    } finally {
      textDialog.value.isSubmitting = false
    }
  }

  const submitConfirmDialog = async (): Promise<boolean> => {
    const submit = confirmDialog.value.submit
    if (!submit) {
      confirmDialog.value.visible = false
      return true
    }
    confirmDialog.value.isSubmitting = true
    try {
      await submit()
      confirmDialog.value.visible = false
      return true
    } catch {
      return false
    } finally {
      confirmDialog.value.isSubmitting = false
    }
  }

  const submitQuickImportDialog = async (): Promise<boolean> => {
    const name = quickImportDialog.value.name.trim()
    const code = quickImportDialog.value.code.trim()
    if (!name || !code) {
      message.warning('蓝图名称和编码都不能为空。')
      return false
    }
    const submit = quickImportDialog.value.submit
    if (!submit) {
      quickImportDialog.value.visible = false
      return true
    }
    quickImportDialog.value.isSubmitting = true
    try {
      await submit(name, code)
      quickImportDialog.value.visible = false
      return true
    } catch {
      return false
    } finally {
      quickImportDialog.value.isSubmitting = false
    }
  }

  return {
    textDialog,
    confirmDialog,
    quickImportDialog,
    openTextDialog,
    openConfirmDialog,
    openQuickImportDialog,
    submitTextDialog,
    submitConfirmDialog,
    submitQuickImportDialog,
  }
}
