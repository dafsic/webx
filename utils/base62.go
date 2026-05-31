package utils

import (
	"encoding/hex"
	"math/big"
	"strings"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// HexToShortID 将 hex 字符串（如 keccak256 输出 "0x57837b..."）转换为 base62 编码后取前 12 位的短 ID。
// 用于将链上 bytes32 类型的 questionId 转为数据库中更短的唯一标识。
func HexToShortID(hexStr string) string {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")

	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return hexStr // fallback: 返回原始字符串
	}

	n := new(big.Int).SetBytes(b)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var encoded []byte
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		encoded = append(encoded, base62Chars[mod.Int64()])
	}

	// 反转
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}

	s := string(encoded)
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
