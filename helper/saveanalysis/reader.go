package saveanalysis

// 文件职责：提供受边界约束的只读二进制读取。
// 使用：区段解码器消费已读取的稳定快照；边界：任何短读、非法长度或未知版本均终止，不返回部分结果。

import (
	"fmt"
	"math"
	"unicode/utf8"
)

// decodeError 职责：保存解析位置和原因；使用：内部中断并由入口转换为错误；边界：不捕获其它程序错误。
type decodeError struct{ message string }

// floatValue 职责：恢复单精度位模式；使用：太阳帆有效性分支；边界：不把浮点字段当整数比较。
func floatValue(bits int64) float32 { return math.Float32frombits(uint32(bits)) }

// Error 职责：提供错误文本；使用：调用方诊断；边界：不包含玩家身份字段。
func (e decodeError) Error() string { return e.message }

// reader 职责：维护快照游标和必要查询区段；使用：单次解析独占；边界：不写文件、不跨解析复用。
type reader struct {
	// capsuleRemoved 表示存档已经记录回收或跳过初始太空舱。
	capsuleRemoved bool
	// 厂区解码期间借用货运缓冲，仅保留分拣器投放点已证实的结果。
	pathBuffers  map[int64][]byte
	beltPivots   map[int64][2]int64
	currentTech  int64
	technology   *PlanningTechnology
	recipes      map[int]bool
	pendingTechs map[int64]bool
	line         *factoryLine
	lines        map[int64]*factoryLine
	surfaces     map[int64]*planetSurface
	scanned      map[int64]*scannedPlanet
	surface      *planetSurface
	data         []byte
	offset       int64
	section      string
	production   *layoutProductionStatistics
	planets      []int64
	// astroNames、worldPlanetIDs 职责：保留存档命名覆盖和天体清单；使用：完整星系关联核对；边界：不以厂区清单替代。
	astroNames     map[int64]string
	worldPlanetIDs []int64
	stations       []*layoutStationComponent
	// virtualRelayZones 保留已落地但尚未实体化的地面基地规划坐标。
	virtualRelayZones map[int64][]RelayZone
	// groundBaseZones 和 pendingGroundBases 分别保留确认基地和当前厂区待关联的主基地实体。
	groundBaseZones    map[int64][]RelayZone
	pendingGroundBases map[int64]*layoutEnemyData
	// astroRoutes 和 astroBans 保留原版按物品和天体的生效规则键，供规划诊断。
	astroRoutes map[uint64]bool
	astroBans   map[uint64]bool
	// storages 职责：按厂区顺序保留地面储物设施；使用：分类库存索引；边界：不收集玩家背包。
	storages []*layoutFactoryStorage
	// world 职责：保留星系生成参数与球体结构；使用：关联查询；边界：不是完整生成后的星系。
	world *layoutGameData
}

// fail 职责：中断无效区段；使用：读取原语；边界：仅在解析入口恢复为返回错误。
func (r *reader) fail(reason string) {
	panic(decodeError{fmt.Sprintf("存档解析失败 [%s @ %d]: %s", r.section, r.offset, reason)})
}

// skip 职责：推进已验证的字节范围；使用：跳过无关数据；边界：不允许负数或越过文件末尾。
func (r *reader) skip(n int64) {
	if n < 0 || n > int64(len(r.data))-r.offset {
		r.fail("存档截断或区段长度无效")
	}
	r.offset += n
}

// length 职责：限制元素数量；使用：数组分配或循环前；边界：每个元素至少占一字节。
func (r *reader) length(n int64) int64 {
	if n < 0 || n > int64(len(r.data))-r.offset {
		r.fail("数组长度无效或存档截断")
	}
	return n
}

// number 职责：读取小端整数；使用：区段字段读取；边界：严格验证完整字节并按位宽扩展符号。
func (r *reader) number(size int64, signed bool) int64 {
	start := r.offset
	r.skip(size)
	var value uint64
	for i := int64(0); i < size; i++ {
		value |= uint64(r.data[start+i]) << (8 * i)
	}
	if signed && size < 8 && value&(1<<(size*8-1)) != 0 {
		value |= ^uint64(0) << (size * 8)
	}
	return int64(value)
}

// string 职责：读取七位编码长度的 UTF-8 字符串；使用：跳过或读取文本字段；边界：拒绝溢出、短读和无效 UTF-8。
func (r *reader) string() string {
	var n int64
	for shift := 0; shift < 35; shift += 7 {
		b := r.number(1, false)
		if shift == 28 && b > 7 {
			r.fail("字符串长度溢出")
		}
		n |= (b & 127) << shift
		if b < 128 {
			start := r.offset
			r.skip(n)
			data := r.data[start:r.offset]
			if !utf8.Valid(data) {
				r.fail("无效 UTF-8")
			}
			return string(data)
		}
	}
	r.fail("字符串长度无效")
	return ""
}

// flexible 职责：读取变长有符号计数；使用：紧凑计数字段；边界：长度前缀与普通七位编码不同。
func (r *reader) flexible() int64 {
	n := r.number(1, false)
	if n == 0 || n > 4 {
		return n
	}
	return r.number(n, n == 4)
}

// expect 职责：校验已支持的区段版本或固定值；使用：布局入口；边界：未知版本明确失败。
func (r *reader) expect(value, expected int64, field string) {
	if value != expected {
		r.section = field
		r.fail(fmt.Sprintf("不支持的版本或固定值 %d（支持 %d）", value, expected))
	}
}

// gameVersion 职责：限定已核对的游戏版本；使用：读取主档头之后；边界：新版本需核对布局后显式支持。
func (r *reader) gameVersion(v *layoutGameSave) {
	if v.majorGameVersion != 0 || v.minorGameVersion != 10 || v.releaseGameVersion != 35 || v.buildGameVersion != 29151 {
		r.fail(fmt.Sprintf("不支持的游戏版本 %d.%d.%d.%d", v.majorGameVersion, v.minorGameVersion, v.releaseGameVersion, v.buildGameVersion))
	}
	if v.fileLength != int64(len(r.data)) {
		r.fail("档头长度与文件长度不一致，存档可能截断")
	}
}
