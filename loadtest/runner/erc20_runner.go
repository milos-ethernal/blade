package runner

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/jsonrpc"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"golang.org/x/sync/errgroup"
)

// ERC20Runner represents a load test runner for ERC20 tokens.
type ERC20Runner struct {
	*BaseLoadTestRunner

	erc20Token         types.Address
	erc20TokenArtifact *contracts.Artifact
}

// NewERC20Runner creates a new ERC20Runner instance with the given LoadTestConfig.
// It returns a pointer to the created ERC20Runner and an error, if any.
func NewERC20Runner(cfg LoadTestConfig) (*ERC20Runner, error) {
	runner, err := NewBaseLoadTestRunner(cfg)
	if err != nil {
		return nil, err
	}

	return &ERC20Runner{BaseLoadTestRunner: runner}, nil
}

// Run executes the ERC20 load test.
// It performs the following steps:
// 1. Creates virtual users (VUs).
// 2. Funds the VUs with native tokens.
// 3. Deploys the ERC20 token contract.
// 4. Mints ERC20 tokens to the VUs.
// 5. Sends transactions using the VUs.
// 6. Waits for the transaction pool to empty.
// 7. Waits for transaction receipts.
// 8. Calculates the transactions per second (TPS) based on block information and transaction statistics.
// Returns an error if any of the steps fail.
func (e *ERC20Runner) Run() error {
	if err := e.createVUs(); err != nil {
		return err
	}

	if err := e.fundVUs(); err != nil {
		return err
	}

	if err := e.deployERC20Token(); err != nil {
		return err
	}

	if err := e.mintERC20TokenToVUs(); err != nil {
		return err
	}

	txHashes, err := e.sendTransactions()
	if err != nil {
		return err
	}

	if err := e.waitForTxPoolToEmpty(); err != nil {
		return err
	}

	blockInfo, txnStats, err := e.waitForReceipts(txHashes)
	if err != nil {
		return err
	}

	return e.calculateTPS(blockInfo, txnStats)
}

// deployERC20Token deploys an ERC20 token contract.
// It loads the contract artifact from the specified file path,
// encodes the constructor inputs, creates a new transaction,
// sends the transaction using a transaction relayer,
// and retrieves the deployment receipt.
// If the deployment is successful, it sets the ERC20 token address
// and artifact in the ERC20Runner instance.
// Returns an error if any step of the deployment process fails.
func (e *ERC20Runner) deployERC20Token() error {
	artifact, err := contracts.LoadArtifactFromFile("../contracts/ZexCoinERC20.json")
	if err != nil {
		return err
	}

	input, err := artifact.Abi.Constructor.Inputs.Encode(map[string]interface{}{
		"coinName":   "ZexCoin",
		"coinSymbol": "ZEX",
		"total":      500000000000,
	})

	if err != nil {
		return err
	}

	txn := types.NewTx(types.NewLegacyTx(
		types.WithTo(nil),
		types.WithInput(append(artifact.Bytecode, input...)),
		types.WithFrom(e.loadTestAccount.key.Address()),
	))

	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithClient(e.client),
		txrelayer.WithReceiptsTimeout(e.cfg.ReceiptsTimeout),
	)
	if err != nil {
		return err
	}

	receipt, err := txRelayer.SendTransaction(txn, e.loadTestAccount.key)
	if err != nil {
		return err
	}

	if receipt == nil || receipt.Status == uint64(types.ReceiptFailed) {
		return fmt.Errorf("failed to deploy ERC20 token")
	}

	e.erc20Token = types.Address(receipt.ContractAddress)
	e.erc20TokenArtifact = artifact

	return nil
}

// mintERC20TokenToVUs mints ERC20 tokens to the specified virtual users (VUs).
// It sends a transfer transaction to each VU's address, minting the specified number of tokens.
// The transaction is sent using a transaction relayer, and the result is checked for success.
// If any error occurs during the minting process, an error is returned.
func (e *ERC20Runner) mintERC20TokenToVUs() error {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithClient(e.client),
		txrelayer.WithReceiptsTimeout(e.cfg.ReceiptsTimeout),
		txrelayer.WithoutNonceGet(),
	)
	if err != nil {
		return err
	}

	nonce, err := e.client.GetNonce(e.loadTestAccount.key.Address(), jsonrpc.PendingBlockNumberOrHash)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(context.Background())
	for i, vu := range e.vus {
		i := i
		vu := vu
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				input, err := e.erc20TokenArtifact.Abi.Methods["transfer"].Encode(map[string]interface{}{
					"receiver":  vu.key.Address(),
					"numTokens": big.NewInt(int64(e.cfg.TxsPerUser)),
				})
				if err != nil {
					return err
				}

				tx := types.NewTx(types.NewLegacyTx(
					types.WithTo(&e.erc20Token),
					types.WithInput(input),
					types.WithNonce(nonce+uint64(i)),
					types.WithFrom(e.loadTestAccount.key.Address()),
				))

				receipt, err := txRelayer.SendTransaction(tx, e.loadTestAccount.key)
				if err != nil {
					return err
				}

				if receipt == nil || receipt.Status != uint64(types.ReceiptSuccess) {
					return fmt.Errorf("failed to mint ERC20 tokens to %s", vu.key.Address())
				}

				return nil
			}
		})
	}

	return g.Wait()
}

// sendTransactions sends transactions for each virtual user (vu) and returns the transaction hashes.
// It retrieves the chain ID from the client and starts a timer to measure the execution time.
// The method uses an errgroup to concurrently send transactions for each vu.
// If the context is canceled, the method returns the context error.
// After sending transactions for each vu, it appends the transaction hashes to the allTxnHashes slice.
// Finally, it prints the execution time and returns the transaction hashes and any error encountered.
func (e *ERC20Runner) sendTransactions() ([]types.Hash, error) {
	chainID, err := e.client.ChainID()
	if err != nil {
		return nil, err
	}

	start := time.Now().UTC()

	allTxnHashes := make([]types.Hash, 0, e.cfg.VUs*e.cfg.TxsPerUser)

	g, ctx := errgroup.WithContext(context.Background())

	for _, vu := range e.vus {
		vu := vu
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()

			default:
				txnHashes, err := e.sendTransactionsForUser(vu, chainID)
				if err != nil {
					return err
				}

				allTxnHashes = append(allTxnHashes, txnHashes...)

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	fmt.Println("Sending transactions took", time.Since(start))

	return allTxnHashes, nil
}

// sendTransactionsForUser sends ERC20 token transactions for a given user account.
// It takes an account pointer and a chainID as input parameters.
// It returns a slice of transaction hashes and an error if any.
func (e *ERC20Runner) sendTransactionsForUser(account *account, chainID *big.Int) ([]types.Hash, error) {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithClient(e.client),
		txrelayer.WithChainID(chainID),
		txrelayer.WithCollectTxnHashes(),
		txrelayer.WithNoWaiting(),
		txrelayer.WithEstimateGasFallback(),
		txrelayer.WithoutNonceGet(),
	)
	if err != nil {
		return nil, err
	}

	gasPrice, err := e.client.GasPrice()
	if err != nil {
		return nil, err
	}

	bigGasPrice := new(big.Int).SetUint64(gasPrice)

	var (
		maxFeePerGas         *big.Int
		maxPriorityFeePerGas *big.Int
	)

	if e.cfg.DynamicTxs {
		mpfpg, err := e.client.MaxPriorityFeePerGas()
		if err != nil {
			return nil, err
		}

		maxPriorityFeePerGas = new(big.Int).Mul(mpfpg, big.NewInt(2))

		feeHistory, err := e.client.FeeHistory(1, jsonrpc.LatestBlockNumber, nil)
		if err != nil {
			return nil, err
		}

		baseFee := big.NewInt(0)

		if len(feeHistory.BaseFee) != 0 {
			baseFee = baseFee.SetUint64(feeHistory.BaseFee[len(feeHistory.BaseFee)-1])
		}

		maxFeePerGas = new(big.Int).Add(baseFee, mpfpg)
		maxFeePerGas.Mul(maxFeePerGas, big.NewInt(2))
	}

	for i := 0; i < e.cfg.TxsPerUser; i++ {
		input, err := e.erc20TokenArtifact.Abi.Methods["transfer"].Encode(map[string]interface{}{
			"receiver":  receiverAddr,
			"numTokens": big.NewInt(1),
		})
		if err != nil {
			return nil, err
		}

		if e.cfg.DynamicTxs {
			_, err = txRelayer.SendTransaction(types.NewTx(types.NewDynamicFeeTx(
				types.WithNonce(account.nonce),
				types.WithTo(&e.erc20Token),
				types.WithFrom(account.key.Address()),
				types.WithGasFeeCap(maxFeePerGas),
				types.WithGasTipCap(maxPriorityFeePerGas),
				types.WithChainID(chainID),
				types.WithInput(input),
			)), account.key)
		} else {
			_, err = txRelayer.SendTransaction(types.NewTx(types.NewLegacyTx(
				types.WithNonce(account.nonce),
				types.WithTo(&e.erc20Token),
				types.WithGasPrice(bigGasPrice),
				types.WithFrom(account.key.Address()),
				types.WithInput(input),
			)), account.key)
		}

		if err != nil {
			return nil, err
		}

		account.nonce++
	}

	return txRelayer.GetTxnHashes(), nil
}
