export type TutorialPageKey = 'global'

export interface TutorialStep {
  readonly selector?: string
  readonly title: string
  readonly description: string
  readonly side?: 'top' | 'right' | 'bottom' | 'left'
  readonly align?: 'start' | 'center' | 'end'
}

export interface TutorialDefinition {
  readonly pageKey: TutorialPageKey
  readonly version: number
  readonly autoStart: boolean
  readonly steps: readonly TutorialStep[]
}
