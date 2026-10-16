# Client 头像即时上传实现与验证

完成时间：2026-10-15；仅编辑 client，未编辑 server 或父目录，未 commit/push/部署。

## 已确认契约

服务端 worker ctx_ce27f275f5c9 与 coordinator 已确认：`POST /user/uploadAvatar`；Go 后端使用 `x-token`；`multipart/form-data` 字段 `file`；JPEG/PNG/WebP，最大 2097152 字节。服务器另校验宽高各不超过 4096、总像素不超过 16777216。成功 `{code:0,data:{user:完整当前用户},msg}` 中 `headerImg` 已持久化，可为 HTTP(S) 或服务器相对 URL；无需再调用 setSelfInfo。业务拒绝 HTTP 200/code 7，不更改头像。

## 实现路径

- `apiDefinition/system/account.go`：新增上传相对路径与 `AccountAvatarData` 响应契约。
- `helper/system/account/avatar.go`：接收纯 Base64，解码前后限制 2MiB、嗅探 JPEG/PNG/WebP，清理文件名，构造带实际 MIME 的 multipart `file`，复用统一账号请求；成功响应缺少用户 ID 或头像时返回错误。
- `helper/system/account/request.go`：抽出已编码正文的统一请求入口，复用 token、续期、401、业务响应与错误记录；认证请求发起前核对服务器，响应用会话代次检查。
- `helper/system/account/session.go`：锁内串行显式会话替换与响应副作用；登录/退出/切服改变 generation，同会话静默续期保持 generation，避免正常并发续期误拒已提交上传；旧会话响应不能恢复 token 或清除新会话，清理回调在锁外通知。
- `service/system/account.go`：Wails `UploadAvatar(fileName, contentBase64)` 返回服务器用户，成功记录头像更新日志。
- `frontend/src/services/account.ts`：文件读取与共享上传状态，成功响应直接替换共享 `accountUser`；失败不写头像，会话 revision 丢弃退出/切服后的响应，user revision 防止上传前发起的读取覆盖新头像。
- `frontend/src/components/layout/account-modal.vue`：现有头像按钮触发隐藏文件选择器，change 即上传，上传中禁重复及保存；原头像保留至成功，失败显示重试且可重选同文件；关闭弹窗后请求继续，共享头像在重开时保留。昵称/电话/邮箱草稿不含头像，仅显式“移除头像”才提交 `headerImg:''`；保留原有未提交的编辑页布局/过渡/样式。
- `frontend/wailsjs/go/system/AccountService.js`、`.d.ts`：由 `wails generate module` 生成，未手改。`frontend/wailsjs/go/models.ts` 生成后与任务开始备份 SHA256 相同（04286C9B2315EB5A958687B99A5816D32CCED5B90CD138FF870223AEABF1B01C），其联机差异属于既有改动。

## 验证结果

1. `go test ./helper/system/account ./service/system`：退出 0；新 `avatar_test.go` 验证 multipart 路径/基础地址前缀、file 字段、MIME、原始字节、文件名、服务器用户返回；业务拒绝、空 data、缺头像、无效 Base64、类型伪装和超限均返回错误而无用户。service/system 无测试文件。
2. `go build ./...`：退出 0（初次与最终复核均通过）。
3. `wails generate module`：退出 0，绑定已生成。
4. `frontend/npm run build`：最终退出 0，包含 vue-tsc 和 Vite 生产打包；Sass legacy-js-api 与大 chunk 提示仍存在。首次失败来自并行工作中的联机图表 markPoint 类型，已由其原 owner 修复，本任务未编辑联机文件。
5. Orca 嵌入浏览器 + **真实 Vue AccountModal / 模拟 Wails 桥接**：12 项断言全部通过，原始输出在 `.local/avatar-ui-validation-result.json`；重现脚本 `.local/avatar-ui-validation.js`、挂载 fixture `.local/avatar-validation.html`、图片 `.local/avatar.png`。fixture 不访问真实服务器，图片内容通过 mock bridge，不能替代真实服务端联调。为避免后台标签页 CSS 过渡阻塞测试，自动断言仅在 fixture 内将动画/过渡时长归零并用计时器驱动 rAF，生产源码未改动画。

浏览器断言覆盖：

- 头像按钮调用文件选择器；文件 change 自动产生唯一 UploadAvatar 请求。
- 上传中保留旧头像，重复调用拒绝，文件选择与保存禁用，程序触发 submit 也不会写资料。
- 关闭期间上传成功直接刷新共享用户，SetSelfInfo 调用数仍为 0；重开保持成功头像。
- 后续输入昵称、电话、邮箱并保存，实际 payload 为 `{nickName:'新昵称',email:'new@example.com',phone:'456'}`，不含 headerImg，头像保持成功值。
- 服务端模拟拒绝后头像不变，role=alert 显示重试；文件 input 已清空，可重选同一文件，重试成功清除错误。
- 上传前发起的用户读取迟到后不覆盖新头像；退出/切服 reset 后迟到上传响应不恢复旧用户。

独立 verifier ctx_c7ec8e7367b4 已通过 orchestration 报告真实 Client Go → Server Gin/JWTAuth → UserService → SQLite/Local OSS 联合验收全 PASS，包含上传持久化、失败保留、随后资料保存不回滚，以及跨服旧续期、退出旧续期、新会话旧401、同会话并发真实续期+上传正向。其权威报告由该 worker 写在 develop/.local/avatar-integration/report.md；本任务未自行修改或把浏览器 mock 当作真实联调。

## 重现浏览器验证

在 client 下将 `.local/avatar-validation.html` 复制为 `frontend/avatar-validation.html`，将 `.local/avatar.png` 分别复制为 `frontend/avatar-old.png` 和 `frontend/avatar-new.png`。启动 `frontend/npm run dev -- --host 127.0.0.1 --port 5177 --strictPort`，用 Orca 创建 http://127.0.0.1:5177/avatar-validation.html 标签页，再对该 page 用 `orca eval --expression (Get-Content .local/avatar-ui-validation.js -Raw) --json`。完成后移除这三个前端临时 fixture 文件；本次已清理，持久证据只保留 .local。

## 限制

未执行打包桌面 WebView2 的系统原生文件对话框点选；浏览器验证通过真实 input change 和原生 click 调用验证选择器接线，服务端真实性由独立 Go 联合 fixture 覆盖。构建属于编译验证，不是自动行为测试。未修改联机诊断，也未覆盖或回滚其他 worker 的工作。
`docs/ex/auth-account-api.md` 已补充 6.1 上传契约、限制、即时保存语义、资料保存避免回滚以及角色 API 权限；`go vet ./helper/system/account ./service/system` 和本任务文件 `git diff --check` 均通过。
