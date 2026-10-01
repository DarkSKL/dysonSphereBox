package operationguard

// 文件职责：保护同一游戏目录的安装和卸载写入。
// 使用：V2 安装、冲突继续和卸载入口获取后在退出时释放。
// 边界：仅当前进程有效；忙碌时返回错误，不排队或替换正在运行的取消句柄。

import (
	"fmt"
	"sync"
)

// mutation 职责：串行保护共享目录及安装清单；使用：Acquire 获取；边界：取消接口不获取该锁。
var mutation sync.Mutex

// Acquire 获取安装文件操作权限。
// 职责：阻止安装与卸载交叉修改共享组件；使用：工作流入口 defer 返回的释放函数；边界：不支持嵌套获取。
func Acquire() (func(), error) {
	if !mutation.TryLock() {
		return nil, fmt.Errorf("另一个模组安装或卸载操作正在进行，请稍后重试")
	}
	return mutation.Unlock, nil
}
