import type {Ref} from 'vue'
import type {modDependence, modTranslation} from '../../wailsjs/go/models'

// =============== 描述翻译契约 ===============
// 说明：复用官方生成的纯文本批次，前端状态独立于账号凭据和安装 DTO。
export type TranslationBatch = modTranslation.BatchData

export type TranslationStatus = 'loading' | 'ready' | 'error'

export interface DescriptionTranslation {
  status: TranslationStatus
  reason: string
  data: string
}

export interface DescriptionTranslationQueue {
  setEnabled(value: boolean): void
  setActive(value: boolean): void
  setContext(value: string): void
  setVisibleTexts(texts: string[]): void
  get(text: string): DescriptionTranslation | undefined
  retry(text: string): void
  dispose(): void
}

export interface ModDescriptionTranslationState {
  showChinese: Ref<boolean>
  titleText(pack: modDependence.RemoteModPackageDTO | undefined): string
  descriptionText(pack: modDependence.RemoteModPackageDTO | undefined): string
  translationStatus(pack: modDependence.RemoteModPackageDTO | undefined): TranslationStatus | undefined
  retryTranslation(pack: modDependence.RemoteModPackageDTO | undefined): void
  setDescriptionVisible(key: string, visible: boolean): void
}
