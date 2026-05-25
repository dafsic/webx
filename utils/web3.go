package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func Web3VerifySignature(address, message, signature string) (bool, error) {
	if IsEthereumAddress(address) {
		return ethVerifySignature(address, message, signature)
	}
	return false, errors.New("unsupported address type")
}

func ethVerifySignature(address, message, signature string) (bool, error) {
	// 解码签名
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return false, err
	}
	if len(sig) != 65 {
		return false, errors.New("invalid signature length")
	}

	// 以太坊签名中，V值需要调整
	if sig[64] != 27 && sig[64] != 28 {
		return false, errors.New("invalid signature V value")
	}
	sig[64] -= 27

	// 计算消息的哈希
	msgHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)))
	// 从签名中恢复公钥
	pubKey, err := crypto.SigToPub(msgHash.Bytes(), sig)
	if err != nil {
		return false, err
	}

	// 从公钥中恢复地址
	recoveredAddr := crypto.PubkeyToAddress(*pubKey).Hex()

	// 比较恢复的地址和传入的地址
	return strings.EqualFold(recoveredAddr, address), nil
}
