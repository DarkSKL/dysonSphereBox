package blueprint

// 文件职责：严格读取信封、四支建筑记录与地基；使用：factory 解码/写回自检；边界：只处理内存，不修改展示解析器。
// 按协议独立实现；参考行为为 dsp-calc utils/parser.js，来源和差异见 NOTICE.md。

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxTextBytes 职责：限制单次串大小；使用：桥接输入检查；边界：不是解压上限。
const MaxTextBytes = 8 << 20

// MaxBuildingCount 职责：统一生成和编解码的建筑安全上限；使用：大规模产线预算与负载检查；边界：不保证游戏放置、预览性能，文本及解压限制仍独立生效。
const MaxBuildingCount = 200000

// maxPayloadBytes 职责：限制解压内存；使用：拒绝压缩炸弹；边界：不执行文件导入。
const maxPayloadBytes = 64 << 20

// Error 职责：携带稳定错误码；使用：服务映射中文信息；边界：不暴露完整蓝图内容。
type Error struct {
	// Code 职责：错误分类；使用：前端分支；边界：不匹配文案。
	Code string `json:"code"`
	// Message 职责：中文诊断；使用：直接展示；边界：不包含源蓝图。
	Message string `json:"message"`
}

// Error 职责：实现标准错误；使用：错误链传递；边界：只返回分类信息。
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// fail 职责：构造格式错误；使用：内部验证；边界：不记录文件内容。
func fail(code, message string) error { return &Error{code, message} }

// reader 职责：收敛字节边界检查；使用：二进制解码；边界：首错后不再前进。
type reader struct {
	// data 职责：负载；使用：按偏移读取；边界：只读。
	data []byte
	// pos 职责：读游标；使用：定位截断；边界：不越界。
	pos int
	// err 职责：首个错误；使用：逐段检查；边界：不覆盖首错。
	err error
}

// take 职责：读取固定字节；使用：所有字段读取；边界：截断时置错并返回空值。
func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data)-r.pos {
		r.err = fail("BP_E_TRUNCATED", fmt.Sprintf("负载在偏移 %d 截断，需要 %d 字节", r.pos, n))
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// read 职责：读取小端字段；使用：结构分段读取；边界：错误不继续读取。
func (r *reader) read(values ...interface{}) {
	for _, v := range values {
		if r.err != nil {
			return
		}
		size := binary.Size(v)
		b := r.take(size)
		if r.err != nil {
			return
		}
		if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, v); err != nil {
			r.err = fail("BP_E_RECORD", err.Error())
			return
		}
	}
}

// content 职责：读取双长度 UTF-8 内容；使用：最新记录；边界：校验声明长度并拒绝错帧。
func (r *reader) content() string {
	var declared int32
	r.read(&declared)
	if r.err != nil || declared == 0 {
		return ""
	}
	if declared < 0 || declared > maxPayloadBytes {
		r.err = fail("BP_E_RECORD", "content 长度无效")
		return ""
	}
	var n uint32
	for shift := uint(0); shift < 35; shift += 7 {
		var b uint8
		r.read(&b)
		if r.err != nil {
			return ""
		}
		if shift == 28 && b > 7 {
			r.err = fail("BP_E_RECORD", "content 前缀溢出")
			return ""
		}
		n |= uint32(b&127) << shift
		if b&128 == 0 {
			if n != uint32(declared) {
				r.err = fail("BP_E_RECORD", "content 两个字节长度不一致")
				return ""
			}
			data := r.take(int(n))
			if !utf8.Valid(data) {
				r.err = fail("BP_E_RECORD", "content 不是合法 UTF-8")
			}
			return string(data)
		}
	}
	r.err = fail("BP_E_RECORD", "content 前缀未结束")
	return ""
}

// building 职责：解析一条按判别字分支的记录；使用：负载循环；边界：不截断 float32、不解释未知参数。
func (r *reader) building() Building {
	var b Building
	r.read(&b.Num)
	switch {
	case b.Num <= -101:
		b.RecordLayout = "v1"
		if b.Num <= -102 {
			b.RecordLayout = "v2"
		}
		r.read(&b.Index, &b.ItemID, &b.ModelIndex, &b.AreaIndex, &b.LocalOffset, &b.Yaw)
		b.LocalOffset2 = b.LocalOffset
		b.Yaw2 = b.Yaw
		if b.ItemID > 2000 && b.ItemID < 2010 {
			r.read(&b.Tilt)
			b.Tilt2 = b.Tilt
		} else if b.ItemID > 2010 && b.ItemID < 2020 {
			r.read(&b.Tilt, &b.Pitch, &b.LocalOffset2, &b.Yaw2, &b.Tilt2, &b.Pitch2)
		}
	case b.Num <= -100:
		b.RecordLayout = "v1-old"
		r.read(&b.Index, &b.AreaIndex, &b.LocalOffset, &b.LocalOffset2, &b.Yaw, &b.Yaw2, &b.Tilt, &b.ItemID, &b.ModelIndex)
	default:
		b.RecordLayout = "legacy"
		b.Index = b.Num
		r.read(&b.AreaIndex, &b.LocalOffset, &b.LocalOffset2, &b.Yaw, &b.Yaw2, &b.ItemID, &b.ModelIndex)
	}
	r.read(&b.OutputObjIdx, &b.InputObjIdx, &b.OutputToSlot, &b.InputFromSlot, &b.OutputFromSlot, &b.InputToSlot, &b.OutputOffset, &b.InputOffset, &b.RecipeID, &b.FilterID)
	var count int16
	r.read(&count)
	if count < 0 {
		r.err = fail("BP_E_RECORD", "参数数量为负")
		return b
	}
	if r.err == nil && int(count) <= (len(r.data)-r.pos)/4 {
		b.Parameters = make([]int32, int(count))
		for i := range b.Parameters {
			r.read(&b.Parameters[i])
		}
	} else if r.err == nil {
		r.err = fail("BP_E_TRUNCATED", "参数数量超过剩余负载")
	}
	if b.Num <= -102 {
		b.Content = r.content()
	}
	return b
}

// decodeBinary 职责：读取完整二进制；使用：解码和编码自检；边界：限制数量且保留未解释尾部。
func decodeBinary(data []byte) (Document, error) {
	d := Document{}
	r := reader{data: data}
	r.read(&d.BinaryVersion, &d.CursorOffset, &d.CursorTargetArea, &d.DragBoxSize, &d.PrimaryAreaIndex)
	if r.err != nil {
		return d, r.err
	}
	if d.BinaryVersion < 0 || d.BinaryVersion > 2 {
		return d, fail("BP_E_PAYLOAD", "不支持的二进制版本")
	}
	var areas uint8
	r.read(&areas)
	if areas == 0 {
		return d, fail("BP_E_HEADER", "没有区域")
	}
	d.Areas = make([]Area, int(areas))
	for i := range d.Areas {
		r.read(&d.Areas[i])
	}
	var count uint32
	r.read(&count)
	if r.err != nil {
		return d, r.err
	}
	if count > MaxBuildingCount || uint64(count)*49 > uint64(len(data)-r.pos) {
		return d, fail("BP_E_TRUNCATED", "建筑数量超出负载或安全上限")
	}
	d.Buildings = make([]Building, 0, int(count))
	seen := map[int32]bool{}
	for i := uint32(0); i < count; i++ {
		b := r.building()
		if r.err != nil {
			return d, fmt.Errorf("建筑 %d: %w", i, r.err)
		}
		if seen[b.Index] {
			return d, fail("BP_E_RECORD", "建筑记录 index 重复")
		}
		seen[b.Index] = true
		if err := validateBuilding(b); err != nil {
			return d, err
		}
		d.Buildings = append(d.Buildings, b)
	}
	if d.BinaryVersion >= 2 {
		r.read(&d.Patch)
		var has uint8
		r.read(&has)
		if has > 1 {
			return d, fail("BP_E_RECORD", "地基存在标记必须为 0 或 1")
		}
		if has == 1 {
			reform := &ReformData{}
			var n int32
			r.read(&reform.Reserved, &n)
			if r.err != nil {
				return d, r.err
			}
			if n < 0 || int64(n)*9 > int64(len(data)-r.pos) {
				return d, fail("BP_E_TRUNCATED", "地基矩形数量无效")
			}
			reform.Rects = make([]ReformRect, int(n))
			for i := range reform.Rects {
				r.read(&reform.Rects[i])
			}
			r.read(&reform.CustomReformColorMask, &n)
			if r.err != nil {
				return d, r.err
			}
			if n < 0 || int64(n)*4 > int64(len(data)-r.pos) {
				return d, fail("BP_E_TRUNCATED", "地基颜色数量无效")
			}
			reform.CustomReformColors = make([]uint32, int(n))
			for i := range reform.CustomReformColors {
				r.read(&reform.CustomReformColors[i])
			}
			d.Reform = reform
		}
	}
	if r.err != nil {
		return d, r.err
	}
	d.RawTail = append([]byte(nil), data[r.pos:]...)
	d.TrailingBytes = len(d.RawTail)
	d.Summary = Summary{BuildingCount: len(d.Buildings), AreaCount: len(d.Areas), ItemCounts: map[int16]int{}}
	for _, b := range d.Buildings {
		d.Summary.ItemCounts[b.ItemID]++
	}
	return d, nil
}

// unzip 职责：限量解压；使用：信封输入；边界：拒绝损坏 gzip 与超过上限的内容。
func unzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fail("BP_E_PAYLOAD", "gzip 头无效")
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, maxPayloadBytes+1))
	if err != nil {
		return nil, fail("BP_E_PAYLOAD", "gzip 负载损坏")
	}
	if len(raw) > maxPayloadBytes {
		return nil, fail("BP_E_TOO_LARGE", "解压负载超过 64 MiB")
	}
	return raw, nil
}

// decodeText 职责：解码百分号字段；使用：信封元数据；边界：保留加号，不作为查询字符串。
func decodeText(s string) (string, error) {
	v, err := url.PathUnescape(s)
	if err != nil {
		return "", fail("BP_E_ENVELOPE", "信封百分号编码无效")
	}
	return v, nil
}

// ticksTime 职责：转换 .NET ticks；使用：时间展示；边界：原始字段仍保留以避免精度丢失。
func ticksTime(ticks string) string {
	n, err := strconv.ParseInt(ticks, 10, 64)
	if err != nil {
		return ""
	}
	n -= 621355968000000000
	return time.Unix(n/10000000, (n%10000000)*100).UTC().Format(time.RFC3339Nano)
}

// Decode 职责：严格解析内存蓝图串；使用：导入、检查、编码自检；边界：strict 仅控制 MD5F，结构错误始终失败。
func Decode(text string, strict bool) (Document, error) {
	d := Document{}
	text = strings.TrimSpace(text)
	if len(text) > MaxTextBytes {
		return d, fail("BP_E_TOO_LARGE", "蓝图文本超过 8 MiB")
	}
	if !strings.HasPrefix(text, "BLUEPRINT:") {
		return d, fail("BP_E_FORMAT", "缺少 BLUEPRINT: 前缀")
	}
	p1 := strings.IndexByte(text, '"')
	p2 := len(text) - 33
	if p1 < 10 || p2 <= p1 || text[p2] != '"' {
		return d, fail("BP_E_FORMAT", "负载引号或校验尾部不完整")
	}
	fields := strings.Split(text[10:p1], ",")
	if len(fields) < 12 {
		return d, fail("BP_E_ENVELOPE", "信封至少需要 12 个字段")
	}
	version, err := strconv.Atoi(fields[0])
	if err != nil || version < 0 || version > 1 || version == 1 && len(fields) < 15 {
		return d, fail("BP_E_ENVELOPE", "信封版本或字段数无效")
	}
	checksum := text[p2+1:]
	if _, err := strconv.ParseUint(checksum[:16], 16, 64); err != nil {
		return d, fail("BP_E_FORMAT", "校验值不是十六进制")
	}
	if _, err := strconv.ParseUint(checksum[16:], 16, 64); err != nil {
		return d, fail("BP_E_FORMAT", "校验值不是十六进制")
	}
	valid := strings.EqualFold(MD5F([]byte(text[:p2])), checksum)
	if strict && !valid {
		return d, fail("BP_E_HASH", "MD5F 不匹配")
	}
	packed, err := base64.StdEncoding.DecodeString(text[p1+1 : p2])
	if err != nil {
		return d, fail("BP_E_BASE64", "负载 base64 无效")
	}
	raw, err := unzip(packed)
	if err != nil {
		return d, err
	}
	d, err = decodeBinary(raw)
	if err != nil {
		return d, err
	}
	d.EnvelopeVersion = version
	d.EnvelopeFields = fields
	d.OriginalGzip = packed
	d.Checksum = checksum
	d.ChecksumValid = valid
	d.GameVersion = fields[9]
	d.CreatedAt = ticksTime(fields[8])
	d.Name, err = decodeText(fields[10])
	if err != nil {
		return d, err
	}
	desc := 11
	if version == 1 {
		d.Author, err = decodeText(fields[11])
		if err != nil {
			return d, err
		}
		desc = 14
	}
	d.Description, err = decodeText(strings.Join(fields[desc:], ","))
	return d, err
}

// validateBuilding 职责：拒绝不可写回的数值；使用：读写前共同检查；边界：不改变坐标、不推断几何合法性。
func validateBuilding(b Building) error {
	if len(b.Parameters) > 32767 || len(b.Content) > maxPayloadBytes {
		return fail("BP_E_RECORD", "建筑参数或内容超限")
	}
	values := []float32{b.LocalOffset.X, b.LocalOffset.Y, b.LocalOffset.Z, b.LocalOffset2.X, b.LocalOffset2.Y, b.LocalOffset2.Z, b.Yaw, b.Tilt, b.Pitch, b.Yaw2, b.Tilt2, b.Pitch2}
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fail("BP_E_RECORD", "建筑含非有限坐标")
		}
	}
	return nil
}
