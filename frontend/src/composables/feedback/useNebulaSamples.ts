import {onScopeDispose, shallowRef, watch} from 'vue'

import type {DisplayStar} from '../../types/feedback'
import {nebulaKey, particleBudget, publicMass, SAMPLE_STRIDE, sampleNebula, type NebulaSource} from './galaxy-nebula'
import NebulaWorker from './galaxy-nebula.worker?worker&inline'

interface SampleBank {
  sources: NebulaSource[]
  slots: Map<string, number>
  budget: number
  data: Float32Array
}

function makeBank(sources: NebulaSource[], budget: number, data: Float32Array): SampleBank {
  return {sources, budget, data, slots: new Map(sources.map((p, i) => [p.starId, i]))}
}

function matchingSlot(bank: SampleBank | undefined, p: NebulaSource): number {
  const slot = bank?.slots.get(p.starId)
  if (!bank || slot === undefined) return -1
  const old = bank.sources[slot]
  return old.x === p.x && old.y === p.y && old.z === p.z && old.count === p.count ? slot : -1
}

// =============== 视区切换与光尘所有权 ===============
// 每颗星的采样在缓冲中连续。换视区立即剔除失效星，保留重叠部分的原始光尘；新区域就绪后一次替换。
export function useNebulaSamples(points: () => readonly DisplayStar[]) {
  const empty = new Float32Array()
  const samples = shallowRef(empty)
  let sources: NebulaSource[] = []
  let drawn: SampleBank | undefined, wide: SampleBank | undefined
  let budget = particleBudget(0)
  let revision = 0
  let job: {revision: number; sources: NebulaSource[]; budget: number} | undefined
  let supported = typeof Worker !== 'undefined'
  let worker: Worker | undefined

  function stop(): void {
    if (worker) {
      worker.onmessage = null
      worker.onerror = null
      worker.terminate()
    }
    worker = undefined
    job = undefined
  }

  function display(bank: SampleBank): void {
    drawn = bank
    samples.value = bank.data
    // 多保留一份较广视区的确定性材质。复用前逐星核对新回执，旧数据不会自行变回可见星。
    if (!wide || bank.sources.length >= wide.sources.length) wide = bank
  }

  function fallback(): void {
    stop()
    supported = false
    display(makeBank(sources, budget, sampleNebula(sources, budget)))
  }

  function submit(): void {
    if (job || (drawn?.sources.length === sources.length && drawn.budget === budget)) return
    if (!supported) { fallback(); return }
    try {
      if (!worker) {
        worker = new NebulaWorker()
        worker.onerror = fallback
        worker.onmessage = ({data}: MessageEvent<{revision: number; samples: Float32Array}>) => {
          const finished = job
          job = undefined
          if (finished && data.revision === revision) {
            display(makeBank(finished.sources, finished.budget, data.samples))
          } else submit()
        }
      }
      job = {revision, sources, budget}
      worker.postMessage(job)
    } catch { fallback() }
  }

  function retain(next: NebulaSource[]): void {
    if (!drawn && !wide) return
    const kept: NebulaSource[] = []
    const blocks: Float32Array[] = []
    const span = budget * SAMPLE_STRIDE
    for (const p of next) {
      let bank = drawn, slot = matchingSlot(bank, p)
      if (slot < 0) { bank = wide; slot = matchingSlot(bank, p) }
      if (!bank || slot < 0) continue
      const start = slot * bank.budget * SAMPLE_STRIDE
      kept.push(p)
      blocks.push(bank.data.subarray(start, start + span))
    }
    if (drawn && drawn.budget === budget && kept.length === drawn.sources.length &&
      kept.every((p, i) => matchingSlot(drawn, p) === i)) return
    const buffer = new Float32Array(blocks.length * span)
    blocks.forEach((block, i) => buffer.set(block, i * span))
    display(makeBank(kept, budget, buffer))
  }

  watch(() => nebulaKey(points()), () => {
    sources = points().filter(p => !p.privateLabel).map(p => ({
      starId: p.starId, x: p.x, y: p.y, z: p.z ?? 0, count: p.count ?? 1,
    }))
    revision++
    if (publicMass(sources) < 2) {
      stop()
      drawn = wide = undefined
      samples.value = empty
      budget = particleBudget(0)
      return
    }
    // 一次浏览中只在触及总预算时降低每星采样。放大时，保留下来的星不会因数量变少而突然变亮。
    budget = Math.min(budget, particleBudget(sources.length))
    retain(sources)
    submit()
  }, {immediate: true})
  onScopeDispose(stop)
  return samples
}
