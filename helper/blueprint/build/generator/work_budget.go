package generator

// 文件职责：按可信生产规模限定完整生成预算；使用：创建任务根上下文；边界：所有子步骤服从父取消，总预算最高十分钟，不把增加预算计作性能收益。
import "time"

// generationWorkBudget 职责：为常规工厂保留四分钟，超大工厂按2048台一档增加完整几何与规则处理时间。
// 使用：一次生成开始前；边界：无效数量不延时，最多追加四档；传送带数不反向改变已经开始的任务截止。
func generationWorkBudget(machines int) time.Duration {
	if machines <= 2048 {
		return 4 * time.Minute
	}
	bands := min(4, (machines-2049)/2048+1)
	return 4*time.Minute + time.Duration(bands)*90*time.Second
}
