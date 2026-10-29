package blueprint

// 文件职责：精确写出蓝图并严格自检；使用：编码及生成结果出口；边界：不写文件，无改动往返复用原 gzip 而不依赖压缩器版本。
// 依据冻结格式独立实现；参考行为与修复差异见 NOTICE.md。

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// writer 职责：维护小端输出；使用：二进制编码；边界：仅内存缓冲。
type writer struct{ bytes.Buffer }

// write 职责：按顺序写字段；使用：记录装配；边界：所有值均为固定宽度基本类型。
func (w *writer) write(values ...interface{}) {
	for _, v := range values {
		_ = binary.Write(&w.Buffer, binary.LittleEndian, v)
	}
}

// content 职责：写双长度 UTF-8；使用：最新建筑记录；边界：长度按字节而非字符计数。
func (w *writer) content(content string) {
	w.write(int32(len(content)))
	if content == "" {
		return
	}
	n := uint32(len(content))
	for n >= 128 {
		w.write(uint8(n) | 128)
		n >>= 7
	}
	w.write(uint8(n))
	w.WriteString(content)
}

// building 职责：写保留原布局的建筑；使用：编解码往返；边界：新建筑默认 -102，不归一化任何角度。
func (w *writer) building(b Building) error {
	if err := validateBuilding(b); err != nil {
		return err
	}
	if len(b.RawTail) > 0 {
		return fail("BP_E_RECORD", "未知建筑扩展尾部不能写回")
	}
	layout := b.RecordLayout
	if layout == "" {
		layout = "v2"
	}
	marker := b.Num
	switch layout {
	case "v2", "v1":
		if layout == "v2" {
			if marker > -102 {
				marker = -102
			}
		} else {
			marker = -101
		}
		w.write(marker, b.Index, b.ItemID, b.ModelIndex, b.AreaIndex, b.LocalOffset, b.Yaw)
		if b.ItemID > 2000 && b.ItemID < 2010 {
			w.write(b.Tilt)
		} else if b.ItemID > 2010 && b.ItemID < 2020 {
			w.write(b.Tilt, b.Pitch, b.LocalOffset2, b.Yaw2, b.Tilt2, b.Pitch2)
		}
	case "v1-old":
		w.write(int32(-100), b.Index, b.AreaIndex, b.LocalOffset, b.LocalOffset2, b.Yaw, b.Yaw2, b.Tilt, b.ItemID, b.ModelIndex)
	case "legacy":
		if b.Index <= -100 {
			return fail("BP_E_RECORD", "旧建筑索引与布局判别字冲突")
		}
		w.write(b.Index, b.AreaIndex, b.LocalOffset, b.LocalOffset2, b.Yaw, b.Yaw2, b.ItemID, b.ModelIndex)
	default:
		return fail("BP_E_RECORD", "未知 recordLayout: "+layout)
	}
	w.write(b.OutputObjIdx, b.InputObjIdx, b.OutputToSlot, b.InputFromSlot, b.OutputFromSlot, b.InputToSlot, b.OutputOffset, b.InputOffset, b.RecipeID, b.FilterID, int16(len(b.Parameters)))
	for _, p := range b.Parameters {
		w.write(p)
	}
	if layout == "v2" {
		w.content(b.Content)
	} else if b.Content != "" {
		return fail("BP_E_RECORD", "旧布局无法承载 content")
	}
	return nil
}

// encodeBinary 职责：写精确二进制；使用：自检和信封装配；边界：保留尾部及地基保留字节，不预留零填充。
func encodeBinary(d Document) ([]byte, error) {
	if len(d.Areas) == 0 || len(d.Buildings) == 0 {
		return nil, fail("BP_E_EMPTY", "蓝图必须包含区域与建筑")
	}
	if len(d.Areas) > 255 || len(d.Buildings) > MaxBuildingCount {
		return nil, fail("BP_E_TOO_LARGE", "区域或建筑数量超限")
	}
	if d.BinaryVersion < 0 || d.BinaryVersion > 2 {
		return nil, fail("BP_E_PAYLOAD", "不支持的二进制版本")
	}
	w := writer{}
	w.write(d.BinaryVersion, d.CursorOffset, d.CursorTargetArea, d.DragBoxSize, d.PrimaryAreaIndex, uint8(len(d.Areas)))
	for _, a := range d.Areas {
		w.write(a)
	}
	w.write(uint32(len(d.Buildings)))
	for i, b := range d.Buildings {
		if err := w.building(b); err != nil {
			return nil, fmt.Errorf("建筑 %d: %w", i, err)
		}
		if w.Len() > maxPayloadBytes {
			return nil, fail("BP_E_TOO_LARGE", "建筑负载超过 64 MiB")
		}
	}
	if d.BinaryVersion >= 2 {
		w.write(d.Patch)
		if d.Reform == nil {
			w.write(uint8(0))
		} else {
			r := d.Reform
			if len(r.Rects) > 2930400 || len(r.CustomReformColors) > 2930400 {
				return nil, fail("BP_E_TOO_LARGE", "地基数据超限")
			}
			w.write(uint8(1), r.Reserved, int32(len(r.Rects)))
			for _, rect := range r.Rects {
				w.write(rect)
			}
			w.write(r.CustomReformColorMask, int32(len(r.CustomReformColors)))
			for _, c := range r.CustomReformColors {
				w.write(c)
			}
		}
	} else if d.Reform != nil {
		return nil, fail("BP_E_RECORD", "旧版本不能携带地基")
	}
	w.Write(d.RawTail)
	if w.Len() > maxPayloadBytes {
		return nil, fail("BP_E_TOO_LARGE", "二进制负载超过 64 MiB")
	}
	return w.Bytes(), nil
}

// escapeText 职责：实现 encodeURIComponent 字节规则；使用：新写信封文字；边界：无改动字段保留原编码。
func escapeText(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.!~*'()", rune(c)) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}

// envelopeFields 职责：构建保留未知字段的信封；使用：编码；边界：不猜测图标语义，时间缺省为固定基准以保证幂等。
func envelopeFields(d Document, version int) ([]string, error) {
	if version < 0 || version > 1 {
		return nil, fail("BP_E_ENVELOPE", "信封版本必须是 0 或 1")
	}
	required := 12
	if version == 1 {
		required = 15
	}
	fields := make([]string, required)
	for i := range fields {
		fields[i] = "0"
	}
	if len(d.EnvelopeFields) >= required && d.EnvelopeVersion == version {
		fields = append([]string(nil), d.EnvelopeFields...)
	} else {
		fields[8] = "621355968000000000"
		fields[9] = d.GameVersion
		fields[10] = ""
		fields[11] = ""
		if version == 1 {
			fields[12] = ""
			fields[13] = ""
			fields[14] = ""
		}
	}
	fields[0] = strconv.Itoa(version)
	if fields[9] != d.GameVersion {
		fields[9] = d.GameVersion
	}
	if current, _ := decodeText(fields[10]); current != d.Name {
		fields[10] = escapeText(d.Name)
	}
	if d.CreatedAt != "" && ticksTime(fields[8]) != d.CreatedAt {
		t, err := time.Parse(time.RFC3339Nano, d.CreatedAt)
		if err != nil {
			return nil, fail("BP_E_ENVELOPE", "创建时间格式无效")
		}
		if t.Year() < 1 || t.Year() > 9999 {
			return nil, fail("BP_E_ENVELOPE", "创建时间超限")
		}
		ticks := t.Unix()*10000000 + int64(t.Nanosecond()/100) + 621355968000000000
		fields[8] = strconv.FormatInt(ticks, 10)
	}
	desc := 11
	if version == 1 {
		desc = 14
		if a, _ := decodeText(fields[11]); a != d.Author {
			fields[11] = escapeText(d.Author)
		}
	}
	if current, _ := decodeText(strings.Join(fields[desc:], ",")); current != d.Description {
		fields = append(fields[:desc], escapeText(d.Description))
	}
	return fields, nil
}

// Encode 职责：编码后自解码验证；使用：导出与生成；边界：只有 Verified 为真才交给调用方，失败不返回半成品。
// 逻辑流：1. 精确写二进制 => 2. 保留原压缩或重新 gzip => 3. 组装信封与 MD5F => 4. 严格解析比对二进制。
func Encode(d Document, envelope int) (EncodeResult, error) {
	result := EncodeResult{}
	raw, err := encodeBinary(d)
	if err != nil {
		return result, err
	}
	fields, err := envelopeFields(d, envelope)
	if err != nil {
		return result, err
	}
	var packed []byte
	if len(d.OriginalGzip) > 0 {
		original, err := unzip(d.OriginalGzip)
		if err == nil && bytes.Equal(original, raw) {
			packed = d.OriginalGzip
		}
	}
	if packed == nil {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		if _, err := gz.Write(raw); err != nil {
			return result, err
		}
		if err := gz.Close(); err != nil {
			return result, err
		}
		packed = b.Bytes()
	}
	text := "BLUEPRINT:" + strings.Join(fields, ",") + "\"" + base64.StdEncoding.EncodeToString(packed)
	checksum := MD5F([]byte(text))
	if strings.EqualFold(checksum, d.Checksum) {
		checksum = d.Checksum
	}
	text += "\"" + checksum
	if len(text) > MaxTextBytes {
		return result, fail("BP_E_TOO_LARGE", "编码结果超过 8 MiB")
	}
	decoded, err := Decode(text, true)
	if err != nil {
		return result, fmt.Errorf("%w: %v", fail("BP_E_SELFCHECK", "编码后不能严格解析"), err)
	}
	check, err := encodeBinary(decoded)
	if err != nil || !bytes.Equal(check, raw) {
		return result, fail("BP_E_SELFCHECK", "编码后字段或记录不一致")
	}
	return EncodeResult{Blueprint: text, Length: len(text), Checksum: checksum, Verified: true}, nil
}
