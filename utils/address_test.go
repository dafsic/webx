package utils

import (
	"testing"
)

func TestIsEthereumAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid ethereum address",
			address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			want:    true,
		},
		{
			name:    "valid ethereum address lowercase",
			address: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
			want:    true,
		},
		{
			name:    "invalid - missing 0x prefix",
			address: "742d35Cc6634C0532925a3b844Bc454e4438f44e",
			want:    false,
		},
		{
			name:    "invalid - wrong length",
			address: "0x742d35Cc6634C0532925a3b844Bc454e4438f4",
			want:    false,
		},
		{
			name:    "invalid - non-hex characters",
			address: "0x742d35Cc6634C0532925a3b844Bc454e4438g44e",
			want:    false,
		},
		{
			name:    "empty string",
			address: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEthereumAddress(tt.address); got != tt.want {
				t.Errorf("IsEthereumAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToEthereumAddressFormat(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "lowercase address with 0x prefix",
			address: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			want:    "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			name:    "uppercase address with 0x prefix",
			address: "0xFB6916095CA1DF60BB79CE92CE3EA74C37C5D359",
			want:    "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		},
		{
			name:    "mixed case address with 0x prefix",
			address: "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
			want:    "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		},
		{
			name:    "address without 0x prefix",
			address: "5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			want:    "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			name:    "invalid - wrong length",
			address: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1be",
			want:    "",
		},
		{
			name:    "invalid - empty string",
			address: "",
			want:    "",
		},
		{
			name:    "all lowercase without prefix",
			address: "742d35cc6634c0532925a3b844bc454e4438f44e",
			want:    "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatEthereumAddress(tt.address); got != tt.want {
				t.Errorf("FormatEthereumAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
