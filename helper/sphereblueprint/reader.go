package sphereblueprint

// 文件职责：读取游戏小端字段；使用：轨道及对象池解码；边界：截断、非法浮点和越界计数始终保留首次错误。

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// reader 职责：跟踪剩余字节及对象预算；使用：每次 Parse 独占；边界：出错后后续读取返回零值。
type reader struct {
	data   []byte
	modern bool
	slots  int
	err    error
}

// fail 职责：记录具体格式错误；使用：内容校验失败时调用；边界：保留最初的失败原因。
func (r *reader) fail(message string) {
	if r.err == nil {
		r.err = errors.New(message)
	}
}

// take 职责：切出定长字段；使用：所有字节读取经此函数；边界：长度不足返回 nil。
func (r *reader) take(size int) []byte {
	if r.err != nil {
		return nil
	}
	if size < 0 || size > len(r.data) {
		r.fail("球壳蓝图载荷被截断")
		return nil
	}
	field := r.data[:size]
	r.data = r.data[size:]
	return field
}

// word 职责：读取 32 位小端位模式；使用：整数或浮点共用；边界：截断返回零。
func (r *reader) word() uint32 {
	if field := r.take(4); field != nil {
		return binary.LittleEndian.Uint32(field)
	}
	return 0
}

// integer 职责：还原有符号游戏整数；使用：读取 ID、版本和种子；边界：保留负值供调用者验证。
func (r *reader) integer() int { return int(int32(r.word())) }

// count 职责：约束数组长度；使用：在循环或分配前调用；边界：拒绝负数、限额及不够容纳元素的载荷。
func (r *reader) count(label string, maximum, minBytes int) int {
	n := r.integer()
	if n < 0 || n > maximum || n > len(r.data)/minBytes {
		r.fail(label + "数量无效或超限")
		return 0
	}
	return n
}

// version 职责：识别对象版本；使用：读取字段分支前；边界：未知版本停止解析。
func (r *reader) version(label string, latest int) int {
	v := r.integer()
	if v < 0 || v > latest {
		r.fail(fmt.Sprintf("不支持的%s版本 %d", label, v))
	}
	return v
}

// boolean 职责：读取 BinaryWriter 布尔值；使用：存在标记和属性；边界：仅接受 0、1。
func (r *reader) boolean() bool {
	field := r.take(1)
	if field == nil {
		return false
	}
	if field[0] > 1 {
		r.fail("球壳蓝图布尔标记无效")
	}
	return field[0] == 1
}

// scalar 职责：读取有限单精度数；使用：坐标、姿态和着色；边界：拒绝 NaN 及无穷大。
func (r *reader) scalar() float64 {
	value := float64(math.Float32frombits(r.word()))
	if math.IsInf(value, 0) || math.IsNaN(value) {
		r.fail("球壳蓝图包含非有限坐标或颜色")
	}
	return value
}

// vector 职责：读取定长向量；使用：位置、四元数或 HSVA；边界：维数由字段格式决定。
func (r *reader) vector(n int) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = r.scalar()
	}
	return v
}

// rgba 职责：保留游戏颜色字节顺序；使用：节点、框架及壳面 DTO；边界：默认零值保留透明通道。
func (r *reader) rgba() uint32 {
	if field := r.take(4); field != nil {
		return binary.BigEndian.Uint32(field)
	}
	return 0
}
