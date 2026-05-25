// Package eip712 implements EIP-712 typed-data hashing and signature
// recovery for Ethereum wallets.
//
// Login flow:
//  1. Server issues a random nonce via GetChallenge.
//  2. Client signs Domain{}.LoginDigest(nonce) with eth_signTypedData_v4.
//  3. Server calls RecoverAddress(digest, sig) and compares the result to the
//     claimed address to authenticate.
package eip712

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

// Domain holds the EIP-712 domain parameters baked into the domain separator.
type Domain struct {
	Name    string // e.g. "WebX People"
	Version string // e.g. "1"
	ChainID uint64
}

// pre-computed type hashes
var (
	domainTypeHash = keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId)"))
	loginTypeHash  = keccak256([]byte("Login(string nonce)"))
)

// separator returns the EIP-712 domain separator for d.
func (d Domain) separator() []byte {
	// abi.encode(typeHash, keccak256(name), keccak256(version), uint256(chainID))
	// Each element is 32 bytes.
	buf := make([]byte, 128)
	copy(buf[0:32], domainTypeHash)
	copy(buf[32:64], keccak256([]byte(d.Name)))
	copy(buf[64:96], keccak256([]byte(d.Version)))
	// chainID as uint256: right-aligned in the 32-byte slot [96:128]
	binary.BigEndian.PutUint64(buf[120:128], d.ChainID)
	return keccak256(buf)
}

// LoginDigest returns the EIP-712 typed-data digest for a Login{nonce} struct.
// This is the 32-byte hash the wallet signs with eth_signTypedData_v4.
func (d Domain) LoginDigest(nonce string) []byte {
	// hashStruct = keccak256(typeHash ++ keccak256(nonce))
	structBuf := make([]byte, 64)
	copy(structBuf[0:32], loginTypeHash)
	copy(structBuf[32:64], keccak256([]byte(nonce)))
	structHash := keccak256(structBuf)

	// digest = keccak256("\x19\x01" ++ domainSeparator ++ structHash)
	sep := d.separator()
	msg := make([]byte, 66)
	msg[0] = 0x19
	msg[1] = 0x01
	copy(msg[2:34], sep)
	copy(msg[34:66], structHash)
	return keccak256(msg)
}

// RecoverAddress recovers the lowercase hex Ethereum address (with 0x prefix)
// that produced sig over digest.
//
// sig is a 65-byte hex string (with or without 0x prefix) in Ethereum format:
//
//	[R(32)] [S(32)] [V(1)]   where V is 0, 1, 27, or 28.
func RecoverAddress(digest []byte, sig string) (string, error) {
	sig = strings.TrimPrefix(sig, "0x")
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return "", fmt.Errorf("eip712: decode signature: %w", err)
	}
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("eip712: signature must be 65 bytes, got %d", len(sigBytes))
	}

	// Normalise v to the decred compact format: 27 or 28.
	v := sigBytes[64]
	if v < 27 {
		v += 27
	}

	// Decred compact signature: [V(1)] [R(32)] [S(32)]
	compact := make([]byte, 65)
	compact[0] = v
	copy(compact[1:33], sigBytes[0:32])  // R
	copy(compact[33:65], sigBytes[32:64]) // S

	pub, _, err := ecdsa.RecoverCompact(compact, digest)
	if err != nil {
		return "", fmt.Errorf("eip712: recover pubkey: %w", err)
	}

	return pubkeyToAddress(pub), nil
}

// pubkeyToAddress converts a secp256k1 public key to a lowercase Ethereum address.
func pubkeyToAddress(pub *secp256k1.PublicKey) string {
	// Uncompressed public key: [04 || X(32) || Y(32)] → strip the 04 prefix.
	uncompressed := pub.SerializeUncompressed()
	hash := keccak256(uncompressed[1:])
	// Ethereum address = last 20 bytes of that hash.
	return "0x" + hex.EncodeToString(hash[12:])
}

// keccak256 computes the Ethereum-compatible Keccak-256 hash.
func keccak256(data ...[]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	for _, b := range data {
		h.Write(b)
	}
	return h.Sum(nil)
}
