package bsc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/world-future/fatecast-be/utils"
	"github.com/world-future/fatecast-be/utils/evm/bsc/bindings"
)

// BscGetReserves 查询流动性池的资产储备
// 参数 - poolAddress: 流动性池合约地址（如 PancakeSwap 的某个 Pair）
func BscGetReserves(client *ethclient.Client, poolAddress common.Address) (*big.Int, *big.Int, error) {
	// 创建合约实例
	pairContract, err := bindings.NewPancakeV2PairCaller(poolAddress, client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to instantiate pair contract: %w%s", err, utils.LineInfo())
	}

	// 调用 getReserves 方法
	reserves, err := pairContract.GetReserves(&bind.CallOpts{Context: context.Background()})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get reserves: %w", err)
	}

	return reserves.Reserve0, reserves.Reserve1, nil
}

// BscGetPair 获取两个代币在 PancakeSwap V2 上的交易对合约地址
func BscGetPair(client *ethclient.Client, factoryAddress common.Address, tokenA common.Address, tokenB common.Address) (common.Address, error) {
	// 创建 factory 合约实例
	factoryContract, err := bindings.NewPancakeV2FactoryCaller(factoryAddress, client)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to instantiate factory contract: %w%s", err, utils.LineInfo())
	}

	// 调用 getPair 方法
	pairAddress, err := factoryContract.GetPair(&bind.CallOpts{Context: context.Background()}, tokenA, tokenB)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get pair: %w%s", err, utils.LineInfo())
	}

	return pairAddress, nil
}

// 通过pair合约查询token0和token1的符号
func FetchTokenSymbols(client *ethclient.Client, pairAddress common.Address) (string, string, error) {
	pairContract, err := bindings.NewPancakeV2PairCaller(pairAddress, client)
	if err != nil {
		return "", "", fmt.Errorf("failed to instantiate PancakeSwap Pair contract: %w", err)
	}

	token0Address, err := pairContract.Token0(&bind.CallOpts{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get token0 address: %w", err)
	}

	token1Address, err := pairContract.Token1(&bind.CallOpts{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get token1 address: %w", err)
	}

	token0Contract, err := bindings.NewErc20(token0Address, client)
	if err != nil {
		return "", "", fmt.Errorf("failed to instantiate token0 contract: %w", err)
	}

	token1Contract, err := bindings.NewErc20(token1Address, client)
	if err != nil {
		return "", "", fmt.Errorf("failed to instantiate token1 contract: %w", err)
	}

	symbol0, err := token0Contract.Symbol(&bind.CallOpts{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get token0 symbol: %w", err)
	}

	symbol1, err := token1Contract.Symbol(&bind.CallOpts{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get token1 symbol: %w", err)
	}

	return symbol0, symbol1, nil
}

func BscGetAmountsOut(client *ethclient.Client, tokenAddress string, usdtAddress string, routerAddress string, decimal uint, usdtDecimal uint) (string, error) {
	// 创建 router 合约实例
	routerContract, err := bindings.NewPancakeV2RouterCaller(common.HexToAddress(routerAddress), client)
	if err != nil {
		return "0", fmt.Errorf("failed to instantiate router contract: %w%s", err, utils.LineInfo())
	}

	// 计算输入金额（1 个 token）
	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimal)), nil)

	// 设置交易路径
	path := []common.Address{
		common.HexToAddress(tokenAddress),
		common.HexToAddress(usdtAddress),
	}

	// 调用 getAmountsOut 方法
	amounts, err := routerContract.GetAmountsOut(&bind.CallOpts{Context: context.Background()}, amountIn, path)
	if err != nil {
		return "0", fmt.Errorf("failed to get amounts out: %w%s", err, utils.LineInfo())
	}

	// 计算并返回价格
	if len(amounts) > 1 {
		// 将输出金额转换为价格（除以 USDT 精度）
		usdtDecimalBigInt := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(usdtDecimal)), nil)
		price := new(big.Float).Quo(
			new(big.Float).SetInt(amounts[1]),
			new(big.Float).SetInt(usdtDecimalBigInt),
		)
		return price.Text('f', 7), nil
	}

	return "0", nil
}
