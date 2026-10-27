package factory

// 文件职责：统一纯引擎错误到桥接状态；使用：factory 服务；边界：不记录完整蓝图或系统路径。

import (
	"DysonSphereBox/helper/blueprint"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"errors"
)

// Status 职责：转换稳定错误码；使用：各绑定方法；边界：未知内部错误不伪称成功。
func Status(err error) contract.Status {
	if err == nil {
		return contract.Status{Code: "OK", Message: "完成"}
	}
	var q *quant.Error
	if errors.As(err, &q) {
		return contract.Status{Code: q.Code, Message: q.Message}
	}
	var b *blueprint.Error
	if errors.As(err, &b) {
		return contract.Status{Code: b.Code, Message: b.Message}
	}
	return contract.Status{Code: "Q_E_INTERNAL", Message: "引擎执行失败，请查看日志"}
}
