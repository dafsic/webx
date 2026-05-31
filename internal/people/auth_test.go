package main

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerifySignature(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	msg := SignMessage(addr, "nonce-123")

	hash := accounts.TextHash([]byte(msg))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatal(err)
	}

	// Recovery id as produced by go-ethereum (0/1).
	if err := verifySignature(addr, msg, hexutil.Encode(sig)); err != nil {
		t.Errorf("valid signature (v=0/1) rejected: %v", err)
	}

	// Recovery id as produced by browser wallets (27/28).
	walletSig := make([]byte, len(sig))
	copy(walletSig, sig)
	walletSig[64] += 27
	if err := verifySignature(addr, msg, hexutil.Encode(walletSig)); err != nil {
		t.Errorf("valid signature (v=27/28) rejected: %v", err)
	}

	// Wrong address must fail.
	other, _ := crypto.GenerateKey()
	otherAddr := strings.ToLower(crypto.PubkeyToAddress(other.PublicKey).Hex())
	if err := verifySignature(otherAddr, msg, hexutil.Encode(sig)); err == nil {
		t.Error("expected signature/address mismatch to fail")
	}

	// Malformed signature must fail.
	if err := verifySignature(addr, msg, "0x1234"); err == nil {
		t.Error("expected malformed signature to fail")
	}
}
