import * as THREE from 'three';
import { applyPremultBlending, damp, RENDER_ORDER } from '../../internal/utils';
import type { Palette } from '../../internal/palette';
import { isAchievementUnlocked } from '../../achievement';
import type { AchievementTier, DysonAchievement } from '../../types';
import { ParticleTrail } from '../trail';
import { BODY_FRAG, BODY_VERT, HALO_EXTENT, HALO_FRAG, HALO_VERT } from './shaders';

// 粒子拖尾弧长（弧度）：已解锁更长；轨道暂停时收缩到很短
const TRAIL_LEN = { unlocked: 0.8, locked: 0.35, paused: 0.06 };

interface OrbitDef {
  radius: number;
  /** 轨道倾角 */
  incl: number;
  /** 升交点经度 */
  node: number;
  /** 角速度（弧度 / 秒） */
  speed: number;
  /** 星球半径 */
  size: number;
  /** 地貌类型：0 熔岩岩石 / 1 冰晶 / 2 气态巨行星 */
  kind: number;
}

// 三条成就轨道：按等级分层，铜在内、金在外
const ORBITS: Record<AchievementTier, OrbitDef> = {
  bronze: { radius: 3.55, incl: 0.34, node: 0.4, speed: 0.07, size: 0.11, kind: 0 },
  silver: { radius: 4.1, incl: -0.22, node: 2.3, speed: 0.052, size: 0.125, kind: 1 },
  gold: { radius: 4.65, incl: 0.16, node: 4.1, speed: 0.04, size: 0.145, kind: 2 },
};
const TIERS: AchievementTier[] = ['bronze', 'silver', 'gold'];
const UNLOCK_DURATION = 1.8;

/** 最外层成就轨道的外缘半径，相机取景与最近缩放距离依赖它 */
export const ACHIEVEMENT_OUTER_RADIUS = ORBITS.gold.radius + ORBITS.gold.size * HALO_EXTENT;

interface OrbitState {
  angle: number;
  /** 0~1，选中节点所在轨道平滑减速到 1（停止） */
  pause: number;
  /** 0~1，悬停 / 选中时轨道上的节点整体增亮 */
  highlight: number;
  matrix: THREE.Matrix3;
}

type Uniforms = Record<string, THREE.IUniform>;

interface Item {
  data: DysonAchievement;
  tier: AchievementTier;
  phase: number;
  holder: THREE.Group;
  body: THREE.Mesh<THREE.SphereGeometry, THREE.ShaderMaterial>;
  halo: THREE.Mesh<THREE.PlaneGeometry, THREE.ShaderMaterial>;
  hit: THREE.Mesh;
  trail: ParticleTrail;
  trailLen: number;
  bodyUniforms: Uniforms;
  haloUniforms: Uniforms;
  hover: number;
  selected: number;
  /** 解锁动画进度 0~1；-1 表示未在播放 */
  unlockT: number;
  startProgress: number;
  spinAxis: THREE.Vector3;
}

/**
 * 成就节点图层：按等级分布在三条倾斜轨道上的小星球，
 * 每个星球带光晕与进度环；不画轨道线，星球身后拖一串向后飘散的粒子。
 * 支持拾取、悬停、选中（暂停所在轨道）与解锁动画。
 * 节点以数组下标标识，与传入 setAchievements 的列表顺序一致。
 */
export class AchievementLayer {
  readonly group = new THREE.Group();

  private items: Item[] = [];
  private hovered = -1;
  private selected = -1;
  private readonly orbitState = {} as Record<AchievementTier, OrbitState>;
  private pointScale = 400;
  private palette: Palette | null = null;

  private readonly bodyGeometry = new THREE.SphereGeometry(1, 48, 32);
  private readonly haloGeometry = new THREE.PlaneGeometry(2, 2);
  private readonly hitGeometry = new THREE.SphereGeometry(1, 8, 6);
  private readonly hitMaterial = new THREE.MeshBasicMaterial({ visible: false });

  private readonly _v = new THREE.Vector3();
  private readonly _v2 = new THREE.Vector3();

  constructor() {
    const m4 = new THREE.Matrix4();
    for (const tier of TIERS) {
      const o = ORBITS[tier];
      const euler = new THREE.Euler(o.incl, o.node, 0, 'YXZ');
      const matrix = new THREE.Matrix3().setFromMatrix4(m4.makeRotationFromEuler(euler));
      this.orbitState[tier] = { angle: 0, pause: 0, highlight: 0, matrix };
    }
  }

  /** 按成就列表重建节点；列表对象被直接引用，调用方需传入场景持有的副本 */
  setAchievements(list: DysonAchievement[]): void {
    this.clearItems();
    const byTier: Record<AchievementTier, number[]> = { bronze: [], silver: [], gold: [] };
    list.forEach((a, i) => byTier[a.tier].push(i));

    list.forEach((data, index) => {
      const tier = data.tier;
      const o = ORBITS[tier];
      const slot = byTier[tier].indexOf(index);
      const total = byTier[tier].length;
      const phase = (slot / total) * Math.PI * 2 + TIERS.indexOf(tier) * 0.9;
      const unlocked = isAchievementUnlocked(data);

      const bodyUniforms: Uniforms = {
        uTier: { value: new THREE.Color() },
        uLocked: { value: new THREE.Color() },
        uLightDir: { value: new THREE.Vector3(0, 0, 1) },
        uUnlock: { value: unlocked ? 1 : 0 },
        uHover: { value: 0 },
        uFlash: { value: 0 },
        uKind: { value: o.kind },
        uSeed: { value: (index * 0.618) % 1 },
        uTime: { value: 0 },
      };
      const body = new THREE.Mesh(
        this.bodyGeometry,
        new THREE.ShaderMaterial({ uniforms: bodyUniforms, vertexShader: BODY_VERT, fragmentShader: BODY_FRAG }),
      );
      body.scale.setScalar(o.size);

      const haloUniforms: Uniforms = {
        uTier: bodyUniforms.uTier,
        uRing: { value: new THREE.Color() },
        uTrack: { value: new THREE.Color() },
        uProgress: { value: Math.min(data.current / data.threshold, 1) },
        uUnlock: bodyUniforms.uUnlock,
        uHover: bodyUniforms.uHover,
        uSelected: { value: 0 },
        uFlash: bodyUniforms.uFlash,
        uWave: { value: 0 },
        uGlow: { value: 1 },
        uOver: { value: 0 },
        uTime: { value: 0 },
      };
      const halo = new THREE.Mesh(
        this.haloGeometry,
        applyPremultBlending(
          new THREE.ShaderMaterial({ uniforms: haloUniforms, vertexShader: HALO_VERT, fragmentShader: HALO_FRAG }),
        ),
      );
      halo.renderOrder = RENDER_ORDER.achNear;
      halo.scale.setScalar(o.size * HALO_EXTENT);

      // 拾取用的不可见球体，比天体本身大，方便鼠标命中
      const hit = new THREE.Mesh(this.hitGeometry, this.hitMaterial);
      hit.scale.setScalar(o.size * 2.6);
      hit.userData.index = index;

      const holder = new THREE.Group();
      holder.add(body, halo, hit);
      this.group.add(holder);

      const trail = new ParticleTrail({
        matrix: this.orbitState[tier].matrix,
        radius: o.radius,
        bodyRadius: o.size,
        seed: (index * 0.377 + 0.13) % 1,
        unlock: bodyUniforms.uUnlock as { value: number },
        orderFront: RENDER_ORDER.achTrailNear,
        orderBack: RENDER_ORDER.achTrailFar,
      });
      trail.setPointScale(this.pointScale);
      this.group.add(...trail.meshes);

      this.items.push({
        data,
        tier,
        phase,
        holder,
        body,
        halo,
        hit,
        trail,
        trailLen: unlocked ? TRAIL_LEN.unlocked : TRAIL_LEN.locked,
        bodyUniforms,
        haloUniforms,
        hover: 0,
        selected: 0,
        unlockT: -1,
        startProgress: 0,
        spinAxis: new THREE.Vector3(Math.sin(index * 1.7) * 0.3, 1, Math.cos(index * 2.3) * 0.3).normalize(),
      });
    });
    this.hovered = -1;
    this.selected = -1;
    if (this.palette) this.applyPalette(this.palette);
  }

  /** 射线拾取，返回成就下标或 -1 */
  pick(raycaster: THREE.Raycaster): number {
    if (this.items.length === 0) return -1;
    const hits = raycaster.intersectObjects(
      this.items.map((it) => it.hit),
      false,
    );
    return hits.length ? (hits[0].object.userData.index as number) : -1;
  }

  setHovered(index: number): void {
    this.hovered = index;
  }

  /** 选中节点：所在轨道暂停；-1 取消选中 */
  setSelected(index: number): void {
    this.selected = index;
  }

  /** 播放解锁动画（数据已由调用方更新） */
  playUnlock(index: number): void {
    const it = this.items[index];
    if (!it) return;
    it.unlockT = 0;
    it.startProgress = it.haloUniforms.uProgress.value;
  }

  getWorldPosition(index: number, out: THREE.Vector3): THREE.Vector3 | null {
    const it = this.items[index];
    return it ? it.holder.getWorldPosition(out) : null;
  }

  applyPalette(p: Palette): void {
    this.palette = p;
    const tierColor: Record<AchievementTier, THREE.Color> = { bronze: p.tierBronze, silver: p.tierSilver, gold: p.tierGold };
    for (const it of this.items) {
      it.bodyUniforms.uTier.value.copy(tierColor[it.tier]);
      it.bodyUniforms.uLocked.value.copy(p.locked);
      it.haloUniforms.uRing.value.copy(p.ring);
      it.haloUniforms.uTrack.value.copy(p.ringTrack);
      it.haloUniforms.uOver.value = p.over;
      it.haloUniforms.uGlow.value = 1 - p.over * 0.55;
      it.trail.setStyle({
        color: tierColor[it.tier],
        locked: p.locked,
        tail: p.trailTail,
        core: p.trailCore,
        alpha: p.trailAlpha,
        over: p.over,
      });
    }
  }

  /** pointScale：世界尺寸点精灵的像素换算系数 */
  setViewport(pointScale: number): void {
    this.pointScale = pointScale;
    for (const it of this.items) it.trail.setPointScale(pointScale);
  }

  update(dt: number, time: number, camera: THREE.Camera, spinSpeed: number): void {
    const selectedTier = this.selected >= 0 ? this.items[this.selected]?.tier : undefined;
    const hoverTier = this.hovered >= 0 ? this.items[this.hovered]?.tier : undefined;
    for (const tier of TIERS) {
      const st = this.orbitState[tier];
      // 选中节点所在轨道平滑减速到停止
      st.pause = damp(st.pause, tier === selectedTier ? 1 : 0, 6, dt);
      st.angle += dt * ORBITS[tier].speed * spinSpeed * (1 - st.pause);
      st.highlight = damp(st.highlight, tier === selectedTier || tier === hoverTier ? 1 : 0, 8, dt);
    }

    camera.updateMatrixWorld();
    const starView = this._v2.setFromMatrixPosition(this.group.matrixWorld).applyMatrix4(camera.matrixWorldInverse);

    this.items.forEach((it, i) => {
      const o = ORBITS[it.tier];
      const st = this.orbitState[it.tier];
      const a = it.phase + st.angle;
      it.holder.position.set(Math.cos(a) * o.radius, 0, Math.sin(a) * o.radius).applyMatrix3(st.matrix);
      it.holder.updateMatrixWorld();

      it.hover = damp(it.hover, i === this.hovered || i === this.selected ? 1 : 0, 10, dt);
      it.selected = damp(it.selected, i === this.selected ? 1 : 0, 10, dt);
      const s = 1 + it.hover * 0.35;
      it.body.scale.setScalar(o.size * s);
      it.halo.scale.setScalar(o.size * HALO_EXTENT * s);
      it.body.rotateOnAxis(it.spinAxis, dt * 0.35);
      it.halo.quaternion.copy(camera.quaternion);

      // 位于恒星后方的节点，光晕要先于壳层绘制，才能被前方壳板遮挡
      const near = it.holder.position.dot(camera.position) >= 0;
      it.halo.renderOrder = near ? RENDER_ORDER.achNear : RENDER_ORDER.achFar;

      // 光照方向：天体指向恒星（视空间）
      const nodeView = this._v.setFromMatrixPosition(it.holder.matrixWorld).applyMatrix4(camera.matrixWorldInverse);
      it.bodyUniforms.uLightDir.value.subVectors(starView, nodeView).normalize();

      it.bodyUniforms.uHover.value = it.hover;
      it.bodyUniforms.uTime.value = time;
      it.haloUniforms.uSelected.value = it.selected;
      it.haloUniforms.uTime.value = time;

      // 解锁动画：进度环补满 → 闪光 → 冲击波扩散，天体由暗转亮
      if (it.unlockT >= 0) {
        it.unlockT += dt / UNLOCK_DURATION;
        const t = Math.min(it.unlockT, 1);
        const fill = Math.min(t / 0.35, 1);
        it.haloUniforms.uProgress.value = THREE.MathUtils.lerp(it.startProgress, 1, 1 - Math.pow(1 - fill, 3));
        const k = Math.max((t - 0.3) / 0.7, 0);
        it.bodyUniforms.uUnlock.value = Math.min(k * 2.5, 1);
        it.bodyUniforms.uFlash.value = k > 0 ? Math.exp(-k * 5) * 1.4 : 0;
        it.haloUniforms.uWave.value = k;
        if (it.unlockT >= 1) {
          it.unlockT = -1;
          it.haloUniforms.uWave.value = 0;
          it.bodyUniforms.uFlash.value = 0;
        }
      }

      // 拖尾：长度随解锁状态变化，轨道暂停时收缩；悬停 / 选中时增亮
      const unlock = it.bodyUniforms.uUnlock.value as number;
      const moving = THREE.MathUtils.lerp(TRAIL_LEN.locked, TRAIL_LEN.unlocked, unlock);
      const targetLen = THREE.MathUtils.lerp(moving, TRAIL_LEN.paused, st.pause);
      it.trailLen = damp(it.trailLen, targetLen, 4, dt);
      it.trail.set(a, it.trailLen, 1 + it.hover * 0.6 + st.highlight * 0.25, time, o.size * s);
    });
  }

  private clearItems(): void {
    for (const it of this.items) {
      this.group.remove(it.holder, ...it.trail.meshes);
      it.body.material.dispose();
      it.halo.material.dispose();
      it.trail.dispose();
    }
    this.items = [];
  }

  dispose(): void {
    this.clearItems();
    this.bodyGeometry.dispose();
    this.haloGeometry.dispose();
    this.hitGeometry.dispose();
    this.hitMaterial.dispose();
  }
}
