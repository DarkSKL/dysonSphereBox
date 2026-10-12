# 2026-10-12 Client 账号接口与版本审查

本记录对应 Client HEAD `7c73c80` 的当前未提交版本；Client 与 Server agent 已直接确认契约。主契约为 Server `aiDoc/frontend-backend/auth-account-api.md`，Client 镜像为 [账号接口](ex/auth-account-api.md)。本轮不 stage、commit 或 push，最终提交由协调者审核。

## 双端契约矩阵

| 能力 | 方法与路径 | 请求 / 身份 | 成功与会话 |
|---|---|---|---|
| 获取账号 | GET /user/getUserInfo | x-token；JWT 当前用户 | code0、data.userInfo；不含 password/token |
| 修改资料 | PUT /user/setSelfInfo | nickName/headerImg/phone/email；省略或 null 保持，空串清除后三项；昵称必填；只取 JWT 非零 ID | HTTP200/code0/data={}/msg设置成功；随后 GET 刷新；不主动重新签发 JWT |
| 修改密码 | POST /user/changePassword | password/newPassword；JWT 当前用户 | 成功后注销并重新登录 |
| 保存界面设置 | PUT /user/setSelfSetting | 独立设置对象，整体覆盖 | code0；不混入资料请求 |
| 注销 | POST /jwt/jsonInBlacklist | x-token | 请求失败也清本地会话 |
| 业务校验失败 | 同上 | 全字段先校验再写入 | HTTP200/code7/msg；不部分更新 |
| 未登录/失效 | 受保护接口 | 无效 token | HTTP401/code7；Client 清会话 |
| 强制改密 | 受保护接口 | 服务端 guard | HTTP409/code10001；Client 切改密；GET/改密/注销放行 |
| 静默续期 | 受保护响应 | new-token 响应头 | 仅 Go 保存，不进入前端 |

字段长度、URL、手机号和邮箱精确规则见主契约与 Client 镜像。ID/uuid/userName/enable/authority/department/password 等额外字段忽略且不写入；用户身份来自 JWT，权限不由前端判断。手机号和邮箱当前是资料字段，没有验证码绑定、联系方式登录或单独的验证状态，不能把填写联系方式解释为已验证绑定。

## 本轮修复与验证

- Client 资料 DTO 改 `*string + omitempty`，显式空串不再丢失；只发送变化字段；UI 允许清除头像和联系方式。Wails 重新生成，没有手改生成文件。
- 保存后 GET 刷新失败明确提示并在重开时重试；刷新 401/409 保留正确登录/改密视图；恢复强制改密和会话失效会清理密码草稿；初始 open=true 调用改为函数声明，避免 TDZ。
- 相对头像按配置服务器地址解析，禁止危险 URL scheme、userinfo 和协议相对外部 host，不携带 token；无法加载时回退图标。
- 外部临时 Go 夹具使用内存 SQLite 与本地 httptest：PUT 方法/基础路径前缀、空串/null、未知 ID/权限/password 字段过滤、GET 用户 DTO 无 token/password、new-token、HTTP200/code7、409/10001、401、错误响应和失败注销均 PASS。
- 外部临时 SFC 逻辑夹具使用编译后的真实账号弹窗 setup：初始 open=true 无缓存用户及未登录、变化字段清除、保存后刷新失败重试、刷新409/401、强制改密恢复、密码草稿清理及相对头像解析均 PASS。夹具没有挂载 WebView，Vue 生命周期警告属直接 setup 验证边界，不构成桌面视觉验收。
- Server agent 报告真实 JWT + Gin + 内存库六组矩阵全部 PASS，含九类危险字段忽略、19例非法值原子失败、GET不泄密、JWT ID0/失效、改密guard/身份、续期、DB故障/context。这是 Server agent 的验证证据，不冒充 Client 独立运行。

本轮 `wails generate module`、`go build ./...`、账号与多人目标包 `go vet`、`npm run build` 均通过。前端只有 Sass legacy API 和大 chunk 警告；全量 `go vet ./...` 仍在未修改历史文件 `model/system/enter.go` 重复 JSON tag 与 `helper/modDependence/bepinex/winver/version.go:67` unsafe.Pointer 失败。生成的 `models.ts` 空白行使 `git diff --check` 报4处尾空白，按生成文件政策保留，不手改。

## 全版本审查与边界

已逐组审查全部 pending 源码：账号弹窗/服务/契约、导航/Game Icons/Tabler/依赖与署名、多人大页面拆分、搜索/分页/预览失效/重复意图/缓存页生命周期、原生 relay 预约/续期/撤回/准入、DCUtR事件/升级等待、转发两侧 fallback 限制、协议v2及 Wails投影。最新 logging 三个提交 `7c73c80/8d3ae62/1167e3b` 与多人基础 `ee790a1`、账号接入 `1ff3533` 已在历史中，不重复提交也不重写。

原生 relay/DCUtR 当前能力缺失默认关闭，v2 必须两端同步发布；fallback=false 时游戏流禁止受限电路，升级超时只报失败。共享 Client DCUtR 进度记录曾记载本地真实联合 TCP/UDP/踢人/仅协商窗口、quota 恢复 PASS；临时测试已删除，本轮仅复核源码与构建，不把历史结果称为本轮复跑。

仍需实际 Wails 日夜/窄窗口/键盘焦点与真实公网 NAT、长时间 quota/断网、真实 Nebula 游戏验证。头像直连公开图片资源，不支持需要 x-token 的私有图片；JWT 昵称快照和已在线多人会话昵称不会随资料保存自动更新，GET 账号显示以数据库新值为准。未测 race、真实账号或生产部署，编译不等同于行为测试。

确认四个临时多人测试 `helper/multiplayer/p2p/relay_test.go`、`helper/multiplayer/forward/relay_test.go`、`service/multiplayer/relay_joint_test.go`、`service/multiplayer/relay_members_joint_test.go` 均不在当前工作区或 git 已跟踪清单。历史验收记录保留并明确移除状态；既有已跟踪 `helper/environment/save_bundle_test.go` 与 `frontend/src/packages/dyson-sphere/test/consumer-ts4.ts` 保留。账号外部临时夹具验证后清理；runtime截图和用户数据不删除、不提交。
