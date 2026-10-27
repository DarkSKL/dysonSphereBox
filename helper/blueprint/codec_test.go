package blueprint

// 文件职责：验证 MD5F、布局、截断、UTF-8 和原始位保真；使用：go test；边界：不把自洽往返当游戏验收。

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// TestMD5FVectors 职责：验证独立参考向量；使用：摘要回归；边界：覆盖跨块与空输入。
func TestMD5FVectors(t *testing.T) {
	vectors := map[string]string{"": "84D1CE3BD68F49AB26EB0F96416617CF", "a": "F10BDDAECB62E5A92433757867EE06DB", "abcd": "FA27C78B6EC31559F0E760CE3F2B03F6", "Why are you doing this, Youthcat Studio?": "13424E12890A3F50A1F8567C464FFF8C"}
	for input, want := range vectors {
		if got := MD5F([]byte(input)); got != want {
			t.Fatalf("MD5F(%q)=%s want %s", input, got, want)
		}
	}
}

// testDocument 职责：构造边界覆盖文档；使用：布局测试；边界：不是游戏导出真值。
func testDocument(layout string) Document {
	return Document{EnvelopeVersion: 1, BinaryVersion: 2, Name: "中文,+测试", Description: "说明,包含逗号", Author: "测试", GameVersion: "0.10.34.28524", Areas: []Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: 20, Height: 20}}, Buildings: []Building{{Index: 42, RecordLayout: layout, ItemID: 2013, LocalOffset: Point{X: math.Float32frombits(0x3e15e9e2), Y: -2, Z: 1}, LocalOffset2: Point{X: 2.25, Y: -1, Z: 2}, Yaw: -90, Yaw2: 270, Tilt: 0.125, OutputObjIdx: -1, InputObjIdx: -1, Parameters: []int32{1, 2, -2147483648}}}}
}

// TestLayoutAndEnvelopeRoundtrip 职责：覆盖四布局与两信封；使用：写回保真；边界：比较完整字符串和原始二进制。
func TestLayoutAndEnvelopeRoundtrip(t *testing.T) {
	for _, layout := range []string{"v2", "v1", "v1-old", "legacy"} {
		for envelope := 0; envelope <= 1; envelope++ {
			t.Run(layout+string(rune('0'+envelope)), func(t *testing.T) {
				d := testDocument(layout)
				if layout == "v2" {
					d.Buildings[0].Content = "非 ASCII 内容 🏭"
				}
				d.RawTail = []byte{0, 0, 7}
				d.Reform = &ReformData{Reserved: 3, Rects: []ReformRect{{Reserved: 2, X: -9, Y: 8, W: 3, H: 4, Data: 127}}, CustomReformColorMask: 0x80000000, CustomReformColors: []uint32{0xffffffff}}
				result, err := Encode(d, envelope)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := Decode(result.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				if math.Float32bits(decoded.Buildings[0].LocalOffset.X) != math.Float32bits(d.Buildings[0].LocalOffset.X) {
					t.Fatal("float32 bits lost")
				}
				if !bytes.Equal(decoded.RawTail, d.RawTail) {
					t.Fatal("tail lost")
				}
				again, err := Encode(decoded, envelope)
				if err != nil {
					t.Fatal(err)
				}
				if again.Blueprint != result.Blueprint {
					t.Fatal("text roundtrip differs")
				}
			})
		}
	}
}

// TestRecordSizes 职责：核对协议偏移和朝向开区间；使用：独立定长断言；边界：没有参数或 content。
func TestRecordSizes(t *testing.T) {
	for _, tc := range []struct {
		layout string
		item   int16
		size   int
	}{{"v2", 2303, 53}, {"v2", 2001, 57}, {"v2", 2013, 85}, {"v2", 2010, 53}, {"v1", 2013, 81}, {"v1-old", 2303, 69}, {"legacy", 2303, 61}} {
		w := writer{}
		b := testDocument(tc.layout).Buildings[0]
		b.ItemID = tc.item
		b.Parameters = nil
		if err := w.building(b); err != nil {
			t.Fatal(err)
		}
		if w.Len() != tc.size {
			t.Fatalf("%+v size=%d", tc, w.Len())
		}
	}
}

// TestRejectCorruptInput 职责：防止截断与错误校验被接受；使用：输入边界回归；边界：宽松只跳过校验，不跳过结构。
func TestRejectCorruptInput(t *testing.T) {
	d := testDocument("v2")
	r, err := Encode(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	bad := r.Blueprint[:len(r.Blueprint)-1] + "0"
	if strings.HasSuffix(r.Blueprint, "0") {
		bad = r.Blueprint[:len(r.Blueprint)-1] + "1"
	}
	if _, err := Decode(bad, true); err == nil {
		t.Fatal("bad hash accepted")
	}
	if _, err := Decode(bad, false); err != nil {
		t.Fatal(err)
	}
	raw, err := encodeBinary(d)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < len(raw); n++ {
		if _, err := decodeBinary(raw[:n]); err == nil {
			t.Fatalf("truncated %d accepted", n)
		}
	}
	binary.LittleEndian.PutUint32(raw[43:47], 0xffffffff)
	if _, err := decodeBinary(raw); err == nil {
		t.Fatal("huge count accepted")
	}
}
