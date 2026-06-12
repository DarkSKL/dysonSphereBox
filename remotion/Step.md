# Remotion 产品介绍动画接续记录

## 当前目标

- 使用 `remotion/public/images/` 中的真实产品截图制作 Remotion 产品介绍动画。
- 构图保持 `1920x1080`、`60fps`。
- 动画使用 Remotion + GSAP，同步方式为暂停 GSAP 时间线并按 Remotion 当前帧 seek。
- 用户最新纠偏：
  - 截图动画不要放大，聚焦框需要适配新版本真实 UI。
  - 底部时间线必须铺满整个底部一整行。
  - 时间线不要拆到焦点级别，按场景分类即可。
  - 时间线需要有相关功能描述。

## 已完成

1. 已读取 Remotion、GSAP core、GSAP React、TypeScript 与项目级约束。
2. 已确认当前 Remotion 构图来自 `src/Root.tsx`，参数为 `1920x1080`、`60fps`、`3600` 帧。
3. 已确认截图展示组件中不再使用 `zoom`、`drift` 或截图舞台 `scale` 推拉动画。
4. 中间尝试过把时间线拆成 40 多个焦点级小段，但用户反馈“太细”；该方案已回退。
5. 已把 `timelineSegments` 调整为 9 个场景分类：
   - 产品定位
   - 模组市场
   - 整合包详情
   - 来源切换
   - 下载安装
   - Nebula 开服
   - 系统日志
   - 设置与更新
   - 能力收束
6. 已重写 `src/components/TimelineProgress.tsx`：
   - 底部仍然是全宽推进 UI。
   - 每个场景段显示场景名与一句短功能描述。
   - 保留全局进度填充、当前进度头、场景段填充、节点和底部 beat。
7. 已重写 `src/style.css` 的时间线样式：
   - 时间线固定在 `bottom: 0`，`left: 0`，`right: 0`。
   - 底部时间线高度为 `118px`，内部轨道 `100px`，铺满整行。
   - 每个场景段按帧长 flex 比例占宽，使用不同 tone 色彩。
   - 文字为单行省略，避免场景描述挤爆布局。
8. 已运行 `npm run typecheck`，当前通过。

## 验证记录

- `rg -n "zoom|drift|scale:|scale\(|transform: \`scale|transform: scale" remotion\src`：无结果，表示当前源码内没有截图缩放相关残留。
- `rg -n "timeline-meta|timeline-label|timeline-short-label|PRODUCT TOUR|戴森球盒子功能推进|shortLabel" remotion\src`：无旧版时间线标题/短标签残留。
- `npm run typecheck`：通过。
- 已使用 `node scripts/with-edge-browser.cjs still src/index.ts MyComp ... --frame=...` 渲染并查看关键帧：
  - `out/timeline-0360.png`：模组市场场景，底部时间线全宽，当前段和功能描述可读。
  - `out/timeline-0900.png`：整合包详情场景，聚焦框对齐详情弹窗标题区域，底栏未遮挡主体。
  - `out/timeline-1500.png`：来源切换双截图场景，底部按场景分类推进，文字无明显溢出。
  - `out/timeline-2100.png`：Nebula 开服场景，已成功渲染待复查。
  - `out/timeline-2520.png`：系统日志场景，底栏未遮挡日志主体和说明文案。
  - `out/timeline-2940.png`：设置场景，聚焦框对齐连接地址区域，底栏不遮挡主要内容。
  - `out/timeline-3420.png`：片尾场景，底栏仍全宽显示，当前段高亮正常。

## 下一步

1. 继续复查 `out/timeline-2100.png` 的开服场景是否与其他关键帧一致。
2. 运行最终静态检查和类型检查。
3. 如时间允许，执行完整视频 render；若完整渲染耗时过长，至少保留当前多关键帧验证结果和命令记录。
