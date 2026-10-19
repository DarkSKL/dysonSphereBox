import {computed, onActivated, onBeforeUnmount, onDeactivated, ref, watch, type Ref} from 'vue'
import type {modDependence} from '../../../wailsjs/go/models'
import {verifiedAccountSession} from '../../services/account'
import {createDescriptionTranslationQueue} from '../../services/descriptionTranslationQueue'
import {translateDescriptions, translationContextKey} from '../../services/modTranslation'
import type {DescriptionTranslation, ModDescriptionTranslationState, TranslationStatus} from '../../types/translation'

type RemotePack = modDependence.RemoteModPackageDTO

// 说明：卡片与详情使用同一标题回退顺序，翻译键与展示原文保持一致。
function packTitle(pack: RemotePack): string {
  return pack.displayName || pack.name || ''
}

// =============== 市场标题与描述翻译投影 ===============
// 说明：开关仅存在当前页面会话，视口回调只报告可见键；详情打开时只加入其当前标题与描述。
// 边界：停用缓存页停止新请求，账号上下文变化清空译文；原始包数据与安装信息始终保持来源值。
export function useModDescriptionTranslation(options: {
  packages: Ref<RemotePack[]>
  selected: Ref<RemotePack | undefined>
  dialogVisible: Ref<boolean>
  available: Ref<boolean>
}): ModDescriptionTranslationState {
  const showChinese = ref(false)
  const revision = ref(0)
  const visibleKeys = ref(new Set<string>())
  const queue = createDescriptionTranslationQueue(translateDescriptions, () => { revision.value++ })
  const visibleTexts = computed(() => {
    if (!options.available.value) return []
    const selected = options.dialogVisible.value ? options.selected.value : undefined
    const packs = selected ? [selected] : options.packages.value.filter((pack) => visibleKeys.value.has(pack.key))
    return packs.filter((pack) => pack.source === 'thunderstore').flatMap((pack) => [packTitle(pack), pack.description || ''])
  })

  watch(visibleTexts, (texts) => queue.setVisibleTexts(texts), {immediate: true, flush: 'sync'})
  watch(showChinese, (enabled) => queue.setEnabled(enabled), {flush: 'sync'})
  watch(verifiedAccountSession, () => queue.setContext(translationContextKey()), {immediate: true, flush: 'sync'})
  onActivated(() => queue.setActive(true))
  onDeactivated(() => queue.setActive(false))
  onBeforeUnmount(() => queue.dispose())

  function translationState(text: string): DescriptionTranslation | undefined {
    void revision.value
    return showChinese.value ? queue.get(text) : undefined
  }

  return {
    showChinese,
    titleText(pack: RemotePack | undefined): string {
      if (!pack) return ''
      const title = packTitle(pack)
      return translationState(title)?.data || title || '未命名模组'
    },
    descriptionText(pack: RemotePack | undefined): string {
      const text = pack?.description || ''
      return translationState(text)?.data || text || '暂无模组说明。'
    },
    // 说明：标题与描述任一在途即为加载中，任一失败即提示重试；均未请求时返回 undefined。
    translationStatus(pack: RemotePack | undefined): TranslationStatus | undefined {
      if (!pack) return undefined
      const states = [translationState(packTitle(pack)), translationState(pack.description || '')]
      if (states.some((state) => state?.status === 'loading')) return 'loading'
      if (states.some((state) => state?.status === 'error')) return 'error'
      return states.some((state) => state?.status === 'ready') ? 'ready' : undefined
    },
    retryTranslation(pack: RemotePack | undefined): void {
      if (!pack) return
      queue.retry(packTitle(pack))
      queue.retry(pack.description || '')
    },
    setDescriptionVisible(key: string, visible: boolean): void {
      const keys = new Set(visibleKeys.value)
      if (visible) keys.add(key)
      else keys.delete(key)
      visibleKeys.value = keys
    },
  }
}
