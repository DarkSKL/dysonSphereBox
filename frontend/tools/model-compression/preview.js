import '../../src/style.scss';
import {createAlgorithmViz} from '../../src/packages/algorithm-viz/src';
import basisJs from 'three/examples/jsm/libs/basis/basis_transcoder.js?url';
import basisWasm from 'three/examples/jsm/libs/basis/basis_transcoder.wasm?url';

// 职责：开发工具中并排展示真实渲染包加载的原版与两档压缩模型；不作为应用路由或发布入口。
const samples = {'assembler-1':2303, 'interstellar-logistic-station':2104, 'turret-plasma':3004, 'smelter-2':2315, 'mining-drill-mk2':2316};
const profiles = ['original', '1024', '512'];
const controllers = [];
const normalZoom = [];
const format = value => `${(value / 1048576).toFixed(2)} MiB`;
const byId = id => document.getElementById(id);
const status = value => { byId('status').textContent = value; };
let reports;
let generation = 0;
let enlarged = false;
let activeView = 0;
let ready = false;
window.comparison = {ready:false, errors:[]};
window.addEventListener('error', event => window.comparison.errors.push(event.message));

const maskFiles = import.meta.glob('../../src/data/models/masks/mask-*.png', {as:'url', eager:true});
const maskUrls = Object.fromEntries(Object.entries(maskFiles).map(([path,url]) => [path.slice(path.lastIndexOf('mask-') + 5, -4), url]));
const urlFor = (profile, name) => profile === 'original' ? `/src/data/models/${name}.glb` : `/.cache/models/${profile}/${name}.glb`;

function frameZoom() {
  controllers.forEach((viz, i) => {
    viz.rig.autoFrame = false;
    viz.rig.zoomGoal = normalZoom[i] * (enlarged ? 2 : 1);
  });
  byId('zoom').textContent = enlarged ? '常用视距' : '放大检查';
}

async function showModel(key) {
  const ticket = ++generation;
  ready = false; window.comparison.ready = false;
  status('正在加载三个模型…');
  controllers.forEach(viz => {
    viz.reset(); viz.resetView();
    viz.enqueue({coordinates:'layout',operations:[{type:'add',id:'sample',building:{itemId:samples[key],x:0,y:0,z:0,yaw:0}}]});
    viz.flush();
  });
  let activeWait = 0;
  let lastWait = performance.now();
  while (controllers.some(viz => viz.getProgress().pending || viz.getProgress().animating || viz.objects.get('sample')?.placeholder)) {
    if (ticket !== generation) return;
    // 比较页可能在后台创建；主动推进跳过队列和模型迁移，不依赖后台被限速的 rAF。
    for (const viz of controllers) {
      viz.frame(0, 0.05);
      if (viz.models.get(key).status === 'missing') throw new Error(`模型读取失败：${key}`);
    }
    // 隐藏标签页暂停场景队列，等待时长只统计可见且可渲染的时间。
    const now = performance.now();
    if (!document.hidden && controllers.some(viz => viz.inView)) activeWait += now - lastWait;
    lastWait = now;
    if (activeWait > 45000) throw new Error('模型加载超时，请检查控制台或缓存');
    await new Promise(resolve => setTimeout(resolve, 40));
  }
  if (ticket !== generation) return;
  await new Promise(resolve => setTimeout(resolve, 250));
  if (ticket !== generation) return;
  controllers.forEach((viz, i) => {
    const obj = viz.objects.get('sample');
    const projectedHeight = obj.height * 0.8165 + (obj.width + obj.depth) * 0.4083;
    const projectedWidth = (obj.width + obj.depth) * 0.7071;
    const host = byId(`view-${profiles[i]}`);
    const fit = Math.max(projectedHeight, projectedWidth / (host.clientWidth / host.clientHeight)) * 1.12;
    const entry = viz.models.get(key);
    const center = entry.center.clone().applyAxisAngle(entry.center.clone().set(0,1,0), obj.pose.yaw);
    center.x += obj.pose.x; center.y += obj.pose.y; center.z += obj.pose.z;
    viz.rig.target.copy(center); viz.rig.targetGoal.copy(center);
    viz.rig.viewSize = viz.rig.viewSizeGoal = Math.max(16, fit);
    normalZoom[i] = Math.min(6, viz.rig.viewSizeGoal / Math.max(fit, 1) * 0.85);
    // 自动取景的最小范围为 16 格，这里只同步开发页的相机状态，便于公平比较小建筑。
    const metadata = reports[i === 0 ? '512' : profiles[i]].models.find(model => model.name === `${key}.glb`);
    byId(`size-${profiles[i]}`).textContent = format(i === 0 ? metadata.beforeBytes : metadata.afterBytes);
    byId(`resolution-${profiles[i]}`).textContent = i === 0 ? '保留原始贴图' : `减少 ${(100 * (1 - metadata.afterBytes / metadata.beforeBytes)).toFixed(1)}%`;
  });
  frameZoom();
  ready = true; window.comparison.ready = true; window.comparison.model = key;
  status('已就绪 · 视角同步');
}

async function main() {
  const entries = await Promise.all(['1024','512'].map(async profile => {
    const response = await fetch(`/.cache/models/${profile}/report.json`);
    if (!response.ok) throw new Error('缺少压缩报告：请先执行 npm run models:compare');
    return [profile, await response.json()];
  }));
  reports = Object.fromEntries(entries);
  byId('total-original').textContent = format(reports['512'].beforeBytes);
  for (const profile of ['1024','512']) {
    byId(`total-${profile}`).textContent = format(reports[profile].afterBytes);
    byId(`saving-${profile}`).textContent = `−${reports[profile].reductionPercent}%`;
  }
  byId('note').textContent = `${reports['512'].modelCount} 个原始 GLB 均保留；网格与材质结构保持一致，压缩版仅降低贴图最大分辨率。`;
  for (const profile of profiles) {
    const host = byId(`view-${profile}`);
    const modelUrls = Object.fromEntries(Object.keys(samples).map(key => [key,urlFor(profile,key)]));
    const viz = createAlgorithmViz(host,{theme:'dark',quality:'high',modelUrls,maskUrls,transcoderUrls:{js:basisJs,wasm:basisWasm}});
    if (!viz.supported) throw new Error('浏览器未提供 WebGL');
    controllers.push(viz);
    for (const name of ['pointerdown','wheel','keydown']) host.addEventListener(name,()=>{ activeView=profiles.indexOf(profile); },{capture:true,passive:true});
  }
  // 比较工具同步内部相机目标；产品组件继续仅使用公开控制器接口。
  const sync = () => {
    if (ready) {
      const source = controllers[activeView].rig;
      for (let i=0;i<controllers.length;i++) if (i!==activeView) {
        const target = controllers[i].rig;
        target.autoFrame=false; target.azimuthGoal=source.dragging?source.azimuth:source.azimuthGoal;
        target.zoomGoal=source.zoomGoal; target.targetGoal.copy(source.targetGoal);
      }
    }
    requestAnimationFrame(sync);
  };
  sync();
  byId('model').addEventListener('change',event=>showModel(event.target.value).catch(fail));
  byId('zoom').addEventListener('click',()=>{enlarged=!enlarged;frameZoom();});
  byId('rotate').addEventListener('click',()=>controllers.forEach(viz=>viz.rotate(1)));
  byId('reset').addEventListener('click',()=>{enlarged=false;showModel(byId('model').value).catch(fail);});
  byId('theme').addEventListener('click',()=>{
    const theme=document.documentElement.dataset.theme==='dark'?'light':'dark';
    document.documentElement.dataset.theme=theme;
    controllers.forEach(viz=>viz.setTheme(theme));
    byId('theme').textContent=theme==='dark'?'切换日间':'切换夜间';
  });
  window.addEventListener('pagehide',()=>controllers.forEach(viz=>viz.dispose()),{once:true});
  window.comparison.controllers=controllers;
  await showModel('assembler-1');
}

function fail(error) { status(error.message); window.comparison.errors.push(error.message); console.error(error); }
main().catch(fail);
