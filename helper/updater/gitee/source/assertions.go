package source

// 文件职责：集中声明 Gitee 更新源与 DTO 的 go-selfupdate 接口适配断言。
// 使用：维护者修改 Source、Release 或 Asset 方法签名时由编译器验证兼容性。
// 边界：只提供编译期检查，不产生运行时行为、不访问网络、不修改状态。

import selfupdate "github.com/creativeprojects/go-selfupdate"

// Source 接口断言确保更新源实现 go-selfupdate.Source。
// 职责：在编译期验证 Source 可被 go-selfupdate.Updater 使用。
// 使用：维护者修改 Source 方法签名时由编译器及时报错。
// 边界：不产生运行时行为。
var _ selfupdate.Source = (*Source)(nil)

// giteeRelease 接口断言确保 Release DTO 实现 SourceRelease。
// 职责：在编译期验证 giteeRelease 可被 go-selfupdate 识别。
// 使用：维护者修改 Release 适配方法时由编译器及时报错。
// 边界：不产生运行时行为。
var _ selfupdate.SourceRelease = (*giteeRelease)(nil)

// giteeAsset 接口断言确保附件 DTO 实现 SourceAsset。
// 职责：在编译期验证 giteeAsset 可被 go-selfupdate 选择和下载。
// 使用：维护者修改附件适配方法时由编译器及时报错。
// 边界：不产生运行时行为。
var _ selfupdate.SourceAsset = (*giteeAsset)(nil)
