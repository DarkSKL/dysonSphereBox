package fileops

// 文件职责：封装文件复制过程中的上下文取消感知读取器。
// 使用：CopyFileWithContext 用它包装源文件 reader，让大型文件复制可以响应用户取消。
// 边界：只检查 context 状态并转换取消错误，不关闭底层 reader、不发送进度、不清理半写入文件。

import (
	"context"
	"io"
)

// contextReader 在读取源文件时检查安装上下文。
// 职责：包装 io.Reader 并在读取前后检查取消状态。
// 使用：CopyFileWithContext 内部包装源文件 reader。
// 边界：不发送进度事件，只转换取消错误。
type contextReader struct {
	// 职责：保存复制流程的取消上下文；使用：Read 每次读取时检查；边界：为空时由构造器回退为 context.Background。
	ctx context.Context
	// 职责：保存真实底层读取器；使用：Read 在取消检查通过后委托读取；边界：不负责关闭底层资源。
	reader io.Reader
}

// newContextReader 创建取消感知读取器。
// 职责：为普通 reader 补齐上下文取消检查能力。
// 使用：文件复制入口统一通过该构造器补齐空上下文。
// 边界：ctx 为空时回退到 context.Background，不会主动取消。
func newContextReader(ctx context.Context, reader io.Reader) io.Reader {
	if ctx == nil {
		ctx = context.Background()
	}
	return contextReader{ctx: ctx, reader: reader}
}

// Read 在复制文件时检查安装上下文是否已取消。
// 职责：在每次读取前后检查取消状态并返回统一取消错误。
// 使用：由 io.Copy 间接调用；边界：不吞掉普通读取错误。
func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, ErrInstallCancelled
	}
	count, err := reader.reader.Read(buffer)
	if err != nil && reader.ctx.Err() != nil {
		return count, ErrInstallCancelled
	}
	return count, err
}
