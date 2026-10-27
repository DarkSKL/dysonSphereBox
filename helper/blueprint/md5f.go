package blueprint

// 文件职责：实现游戏蓝图 MD5F；使用：信封校验与写出；边界：不是密码学认证，不使用标准 MD5 替代。
// 按蓝图格式事实独立实现，常量核对 dsp-calc utils/md5.js（MulanPSL-2.0），来源见 NOTICE.md。

import (
	"encoding/binary"
	"encoding/hex"
	"math/bits"
	"strings"
)

// MD5F 职责：计算游戏变体摘要；使用：传入从 BLUEPRINT: 至结束引号之前的字节；边界：不包含结束引号。
func MD5F(data []byte) string {
	k := [...]uint32{0xd76aa478, 0xe8d7b756, 0x242070db, 0xc1bdceee, 0xf57c0faf, 0x4787c62a, 0xa8304623, 0xfd469501, 0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be, 0x6b9f1122, 0xfd987193, 0xa679438e, 0x39b40821, 0xf61e2562, 0xc040b340, 0x265e5a51, 0xc9b6c7aa, 0xd62f105d, 0x02443453, 0xd8a1e681, 0xe7d3fbc8, 0x21f1cde6, 0xc33707d6, 0xf4d50d87, 0x475a14ed, 0xa9e3e905, 0xfcefa3f8, 0x676f02d9, 0x8d2a4c8a, 0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c, 0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70, 0x289b7ec6, 0xeaa127fa, 0xd4ef3085, 0x04881d05, 0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665, 0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039, 0x655b59c3, 0x8f0ccc92, 0xffeff47d, 0x85845dd1, 0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1, 0xf7537e82, 0xbd3af235, 0x2ad7d2bb, 0xeb86d391}
	s := [...]int{7, 12, 17, 22, 5, 9, 14, 20, 4, 11, 16, 23, 6, 10, 15, 21}
	state := [4]uint32{0x67452301, 0xefdcab89, 0x98badcfe, 0x10325746}
	padded := make([]byte, (len(data)+9+63)/64*64)
	copy(padded, data)
	padded[len(data)] = 0x80
	binary.LittleEndian.PutUint64(padded[len(padded)-8:], uint64(len(data))*8)
	for off := 0; off < len(padded); off += 64 {
		a, b, c, d := state[0], state[1], state[2], state[3]
		for i := 0; i < 64; i++ {
			var f uint32
			g := i
			switch {
			case i < 16:
				f = b&c | ^b&d
			case i < 32:
				f, g = d&b|^d&c, (5*i+1)%16
			case i < 48:
				f, g = b^c^d, (3*i+5)%16
			default:
				f, g = c^(b|^d), (7*i)%16
			}
			x := a + f + k[i] + binary.LittleEndian.Uint32(padded[off+g*4:])
			a, d, c, b = d, c, b, b+bits.RotateLeft32(x, s[(i/16)*4+i%4])
		}
		state[0] += a
		state[1] += b
		state[2] += c
		state[3] += d
	}
	var out [16]byte
	for i, value := range state {
		binary.LittleEndian.PutUint32(out[i*4:], value)
	}
	return strings.ToUpper(hex.EncodeToString(out[:]))
}
