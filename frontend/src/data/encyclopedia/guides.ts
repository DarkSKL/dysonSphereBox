import type {EncyclopediaGuide} from '../../types/encyclopedia'

// =============== 离线阅读与工具入口 ===============
// 说明：以下为本模块编写的简明索引，不复制 Wiki 正文；数值与配方以数据集条目为准。
export const encyclopediaGuides: readonly EncyclopediaGuide[] = [
  {id: 'icarus', category: 'setting', title: '伊卡洛斯与星际工业', summary: '从一台机甲开始，建立通往恒星的工业体系。', paragraphs: ['你驾驶伊卡洛斯探索行星、采集资源并建造工厂。自动化生产将逐步替代手工制作，星际物流把分散的行星连接成一个系统。', '研究科技、制造太阳帆与戴森球组件，将恒星能量转化为工业发展的动力。'], related: ['technology:electromagnetism', 'item:dyson-sphere-component']},
  {id: 'universe', category: 'setting', title: '行星与资源', summary: '矿脉、海洋和气态行星构成不同的资源来源。', paragraphs: ['矿物并不均匀分布在每一颗行星上。规划生产前，先确认本地矿脉、海洋以及附近气态行星能够提供的原料。', '图鉴的资源分区列出采集配方和采集建筑；具体星区的储量与产量以你的存档为准。'], related: ['resource:iron-vein', 'resource:gas-giant', 'building:orbital-collector']},
  {id: 'automation', category: 'gameplay', title: '从采矿到自动化', summary: '采矿、电力、传送带与分拣器连接起第一条生产线。', paragraphs: ['先为采矿机供电，再用传送带把矿物送到冶炼设备附近。分拣器负责在建筑与传送带之间搬运物品。', '打开配方可以查看单次制作的输入、输出和基础耗时。机器生产速度不同，实际吞吐量也会不同。'], related: ['building:mining-machine', 'building:arc-smelter', 'recipe:iron-ingot', 'building:conveyor-belt-1']},
  {id: 'research', category: 'gameplay', title: '矩阵与科技解锁', summary: '研究材料推动科技进程，前置关系决定解锁顺序。', paragraphs: ['科技详情会显示研究材料、研究哈希、前置科技以及解锁的配方。点击关联条目可继续沿科技与生产关系查询。', '矩阵实验室既能生产矩阵，也能进行研究。图鉴提供基础数据，不模拟存档中的研究进度。'], related: ['building:matrix-lab', 'technology:electromagnetism', 'technology:automatic-metallurgy']},
  {id: 'combat', category: 'combat', title: '黑雾与行星防御', summary: '结合炮塔、弹药供应和电力保障布置防线。', paragraphs: ['战斗分类收录数据集中的黑雾相关建筑、物资与科技。防御建筑依赖不同的弹药和供电条件，建立稳定的补给链同样重要。', '图鉴不改变游戏难度，也不读取当前存档的敌情。具体战斗行为以游戏版本为准。'], related: ['building:df-gauss-turret', 'building:df-missile-turret', 'building:df-battlefield-analysis-base']},
  {id: 'achievements', category: 'achievement', title: '成就与挑战', summary: '把探索、生产与戴森球建造作为下一次挑战的方向。', paragraphs: ['成就覆盖探索、自动化生产、科技发展和特殊条件挑战。完整的触发条件与解锁状态请以游戏内成就列表为准。', '本图鉴不读取 Steam 成就或存档，也不会把“任务完成！”科技视作账号成就。你可以从相关科技与建筑条目继续规划自己的目标。'], related: ['technology:mission-completed', 'building:vertical-launching-silo', 'item:universe-matrix']},
  {id: 'recipe-reading', category: 'tutorial', title: '如何阅读配方', summary: '先看输入输出，再看耗时、生产设备与解锁条件。', paragraphs: ['配方数量表示单次制作的材料比例。输入物品位于左侧，输出物品位于右侧，基础耗时显示在配方流程中。', '同一个物品可能有多种生产配方。打开物品详情的“生产配方”即可比较不同来源，再点击材料继续查询上游。'], related: ['item:graphene', 'recipe:graphene', 'item:hydrogen']},
  {id: 'logistics', category: 'tutorial', title: '组织运输与仓储', summary: '从传送带到星际物流，将分散的生产过程连接起来。', paragraphs: ['传送带与分拣器适合近距离连续运输；仓储为生产波动留出缓冲。跨区域或跨行星运输时，可以查询物流运输站及其相关材料。', '规划时同时考虑上游供给、下游消耗以及设备的运输能力。图鉴中的运输速度为基础值，不含存档中的升级或额外配置。'], related: ['building:conveyor-belt-1', 'building:planetary-logistics-station', 'building:interstellar-logistics-station']},
  {id: 'blueprints', category: 'tool', title: '蓝图工具', summary: '打开蓝图编辑与预览工具，整理你的建造方案。', paragraphs: ['蓝图工具属于客户端已有功能。通过下方入口打开后，可继续使用该页面支持的编辑与预览能力。'], related: [], to: '/dysonSphere/blueprint-generator', action: '打开蓝图工具'},
  {id: 'library', category: 'tool', title: '本地资源库', summary: '管理本机模组、蓝图和存档资源。', paragraphs: ['本地资源库负责资源整理；图鉴只提供只读参考。前往资源库后的导入、启用或删除操作仍由原页面处理。'], related: [], to: '/dysonSphere/environment', action: '打开资源库'},
]
