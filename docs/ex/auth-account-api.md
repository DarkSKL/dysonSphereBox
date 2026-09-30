# 账号与认证接口接入说明

面向接入方(前端 / 客户端 / 另一个 AI agent)，覆盖：验证码、注册、登录、获取当前用户、修改资料、修改密码、注销。字段与行为以 `server/` 实现为准，Swagger 见 `/swagger/index.html`。

## 1. 通用约定

### 地址

- 后端直连：`http://127.0.0.1:8888`（`system.router-prefix` 为空，路径即下表所列）
- 走本仓库前端开发代理：`/api` + 路径（Vite 会去掉 `/api` 前缀转发到 8888）

### 响应结构

所有接口返回 `{ code, data, msg }`：

| 场景 | HTTP 状态 | code | 说明 |
|---|---|---|---|
| 成功 | 200 | `0` | |
| 业务失败(参数、验证码、密码错误、查重等) | 200 | `7` | 看 `msg` |
| Casbin 无权限 | 200 | `7` | `msg = "权限不足"`，说明角色未分配该 API |
| 未登录 / token 无效 / 过期 / 已注销 | **401** | `7` | `data = null`，应清空本地 token 回到登录页 |
| 需要先改密码 | **409** | `10001` | `data = { needChangePassword: true }`，只能调用改密 / 获取信息 / 注销 |
| 限流(安全配置开启 IP 限流时) | 200 | `7` | 仅作用于 `/base/*` |

判断成功只看 `code === 0`，不要只看 HTTP 状态。

### 鉴权

- 登录成功后把 `data.token` 存起来，之后所有非 `/base/*` 请求带请求头 `x-token: <token>`
- 后端同时会写一个 `x-token` 的 HttpOnly Cookie；跨域客户端以请求头为准
- **静默续期**：token 剩余有效期小于 buffer(默认 1 天，总有效期默认 7 天)时，任意受保护接口的响应头会带 `new-token` 和 `new-expires-at`，收到后替换本地 token
- 时间单位不一致：登录返回的 `expiresAt` 是**毫秒**时间戳，响应头 `new-expires-at` 是**秒**时间戳

### 字段命名注意

- 登录 / 注册请求里用户名字段是 `username`（全小写）
- 返回的用户对象 `SysUser` 里是 `userName`（驼峰），主键是大写 `ID`

## 2. 验证码 `POST /base/captcha`

公开接口，无请求体。

响应 `data`：

```json
{
  "captchaId": "xxxx",
  "picPath": "data:image/png;base64,....",
  "captchaLength": 6,
  "openCaptcha": true
}
```

- `picPath` 可直接作为 `<img src>`
- `captchaLength` 是验证码位数(纯数字)，可用来限制输入框长度
- `openCaptcha`：对**登录**有意义，`true` 表示登录必须带验证码(默认配置下恒为 `true`)。**注册无论该值如何都必须带验证码**
- 验证码**一次性**：只要被提交校验过一次(不论后续成功失败)就作废，有效期约 180 秒。任何一次登录 / 注册失败后都要重新获取验证码

## 3. 注册 `POST /base/register`

公开接口，不需要 token。注册成功**不会自动登录**。

请求体：

```json
{
  "username": "zhangsan",
  "password": "Blue7Horse",
  "captcha": "123456",
  "captchaId": "xxxx",
  "nickName": "张三"
}
```

| 字段 | 必填 | 规则 |
|---|---|---|
| `username` | 是 | 字母开头，4-32 位字母 / 数字 / 下划线；前后空格会被去掉；**忽略大小写查重**；`admin`、`root`、`system`、`test` 等保留名不可用 |
| `password` | 是 | 见下方密码规则 |
| `captcha` / `captchaId` | 是 | 来自 `/base/captcha` |
| `nickName` | 否 | 缺省等于用户名，超过 32 字符截断 |

密码规则(全部满足)：

- 满足「系统设置 → 安全配置」中的密码策略(默认最少 8 位；是否强制大写 / 小写 / 数字 / 特殊字符由管理员配置)
- 不超过 72 字节
- 字母、数字、符号中至少包含两类
- 不含空格等空白字符
- 不包含用户名(忽略大小写)
- 不是常见弱口令(如 `password123`、`12345678`、`admin123`)

成功响应：`data.user` 为新建的 `SysUser`，`authorityId = 1001`，`enable = 1`。

失败 `msg`(code 7)：

- `验证码错误`
- `Username值不能为空` / `Password值不能为空`
- `用户名需以字母开头, 由 4-32 位字母、数字或下划线组成`
- `该用户名为系统保留, 请更换`
- `密码长度不能少于 N 位` / `密码必须包含…` / `密码长度不能超过 72 字节` / `密码不能包含空白或控制字符` / `密码至少需包含字母、数字、符号中的两类` / `密码不能包含用户名` / `密码过于常见, 请更换`
- `用户名已注册`
- `默认注册角色不存在, 请联系管理员`(后台没建 1001 角色)
- `注册失败`(其他服务端错误)

校验顺序是先验证码、再账号密码，所以**任何失败都已消耗验证码**。建议客户端先在本地做一遍上述格式校验，再提交。

## 4. 登录 `POST /base/login`

公开接口。

请求体：

```json
{
  "username": "zhangsan",
  "password": "Blue7Horse",
  "captcha": "123456",
  "captchaId": "xxxx"
}
```

成功响应 `data`：

```json
{
  "user": { "ID": 12, "uuid": "…", "userName": "zhangsan", "nickName": "张三", "authorityId": 1001, "…": "…" },
  "token": "eyJhbGciOi…",
  "expiresAt": 1759800000000,
  "needChangePassword": false
}
```

- `needChangePassword = true`：管理员开启了「新用户强制改密」或密码已过期。此时 token 只能调用 `/user/changePassword`、`/user/getUserInfo`、`/jwt/jsonInBlacklist`，其他接口返回 409 / 10001，应直接跳到改密页

失败 `msg`(code 7)：

- `验证码错误`
- `用户名不存在或者密码错误`
- `用户被禁止登录`
- `账号已锁定，请 N 分钟后再试`(安全配置开启账号锁定且连续失败达到阈值)

## 5. 获取当前用户 `GET /user/getUserInfo`

需要 `x-token`。无参数。

响应 `data.userInfo` 为 `SysUser`，常用字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `ID` | number | 用户主键 |
| `uuid` | string | 用户 UUID |
| `userName` | string | 登录名 |
| `nickName` | string | 昵称 |
| `headerImg` | string | 头像 URL |
| `phone` / `email` | string | 手机 / 邮箱 |
| `authorityId` | number | 当前角色 ID(注册用户为 1001) |
| `authority` | object | 当前角色，含 `authorityName`、`defaultRouter` |
| `authorities` | array | 拥有的全部角色 |
| `enable` | number | 1 正常，2 冻结 |
| `originSetting` | object \| null | 个人界面配置(见 7) |
| `passwordUpdatedAt` | string \| null | 密码最后修改时间 |
| `CreatedAt` / `UpdatedAt` | string | 时间 |

`password` 不会返回。

## 6. 修改个人资料 `PUT /user/setSelfInfo`

需要 `x-token`。只能改自己(用户 ID 取自 token)。

请求体(按需传)：

```json
{
  "nickName": "新昵称",
  "headerImg": "https://…/avatar.png",
  "phone": "13800000000",
  "email": "user@example.com"
}
```

- 成功 `msg = "设置成功"`，不返回数据；需要最新数据时重新调 `getUserInfo`
- **空字符串会被忽略**，因此无法通过此接口把字段清空
- 不要传 `enable`、`authorityIds` 等字段
- Swagger 里写的是 `/user/SetSelfInfo`，实际路由是小写开头的 `/user/setSelfInfo`

## 7. 保存个人界面配置 `PUT /user/setSelfSetting`

需要 `x-token`。请求体是任意 JSON 对象，**整体覆盖** `originSetting`(不是合并)，客户端应先读取、合并后再提交。成功 `msg = "设置成功"`。

## 8. 修改密码 `POST /user/changePassword`

需要 `x-token`。

```json
{
  "password": "旧密码",
  "newPassword": "新密码"
}
```

- 新密码只校验安全配置中的密码策略(比注册宽松，客户端可复用注册规则做本地校验以保持一致)
- 失败 `msg`：`Password值不能为空` / `NewPassword值不能为空` / 策略类提示 / `修改失败，原密码与当前账户不符`
- 成功后旧 token 仍有效但带着改密前的状态，**应立即调用注销并引导重新登录**

## 9. 注销 `POST /jwt/jsonInBlacklist`

需要 `x-token`，无请求体。当前 token 进入黑名单并清除 Cookie，之后再用该 token 会得到 401。成功 `msg = "jwt作废成功"`。无论接口成功与否，客户端都应清空本地 token。

## 10. (可选)获取菜单 `POST /menu/getMenu`

需要 `x-token`，请求体 `{}`。返回 `data.menus` 为当前角色的菜单树。只有需要复用本系统动态路由时才接。

## 11. 推荐接入流程

1. 进入注册 / 登录页：调 `/base/captcha` 展示图片
2. 注册：本地校验 → `/base/register`；失败则刷新验证码；成功后提示并切到登录(再取一次验证码)
3. 登录：`/base/login`；失败刷新验证码；成功保存 `token`
4. `needChangePassword = true` → 改密页 → `/user/changePassword` → `/jwt/jsonInBlacklist` → 回登录页
5. 否则 `/user/getUserInfo` 拉取用户信息进入应用
6. 请求拦截器：统一加 `x-token`；响应头有 `new-token` 就替换；HTTP 401 清 token 回登录；HTTP 409 且 `code = 10001` 跳改密页
7. 退出：`/jwt/jsonInBlacklist` 后清本地状态

## 12. 角色 1001 需要的 API 权限

注册用户使用上述接口，需要在「角色管理 → 1001 → API 权限」中勾选：

- `GET /user/getUserInfo`
- `PUT /user/setSelfInfo`
- `PUT /user/setSelfSetting`
- `POST /user/changePassword`
- `POST /jwt/jsonInBlacklist`
- `POST /menu/getMenu`(使用第 10 节时)

`/base/captcha`、`/base/register`、`/base/login` 为公开接口，不需要分配。缺权限时接口返回 `code 7 / 权限不足`。

## 相关记录

- 业务记忆：`aiDoc/memory/business/active/user-public-register.md`
