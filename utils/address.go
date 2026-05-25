package utils

import (
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// IsEthereumAddress 验证是否是以太坊地址
func IsEthereumAddress(address string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(address)
}

// 将 address 格式化为ethereum标准格式（带大小写的地址格式）
func FormatEthereumAddress(address string) string {
	// 去除前导的 0x
	address = strings.TrimPrefix(address, "0x")

	// 验证地址长度
	if len(address) != 40 {
		return ""
	}

	// 使用 go-ethereum 的 common.HexToAddress 来格式化为 EIP-55 校验和格式
	return common.HexToAddress(address).Hex()
}
