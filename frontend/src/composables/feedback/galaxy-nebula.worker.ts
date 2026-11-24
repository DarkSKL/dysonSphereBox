import {sampleNebula, type NebulaSource} from './galaxy-nebula'

// 在工作线程建立星云，主线程可以在此期间旋转、选星和阅读反馈。
self.onmessage = ({data}: MessageEvent<{revision: number; sources: NebulaSource[]; budget: number}>) => {
  const samples = sampleNebula(data.sources, data.budget)
  self.postMessage({revision: data.revision, samples}, {transfer: [samples.buffer]})
}
