package runner

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/jsonrpc"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/schollz/progressbar/v3"
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
	fmt.Println("Running ERC20 load test", e.cfg.LoadTestName)

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
	fmt.Println("=============================================================")
	fmt.Println("Deploying ERC20 token contract")

	start := time.Now().UTC()
	artifact := contractsapi.ZexCoinERC20

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

	fmt.Printf("Deploying ERC20 token took %s\n", time.Since(start))

	return nil
}

// mintERC20TokenToVUs mints ERC20 tokens to the specified virtual users (VUs).
// It sends a transfer transaction to each VU's address, minting the specified number of tokens.
// The transaction is sent using a transaction relayer, and the result is checked for success.
// If any error occurs during the minting process, an error is returned.
func (e *ERC20Runner) mintERC20TokenToVUs() error {
	fmt.Println("=============================================================")

	start := time.Now().UTC()

	bar := progressbar.Default(int64(e.cfg.VUs), "Minting ERC20 tokens to VUs")
	defer bar.Close()

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

				bar.Add(1)

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	fmt.Printf("Minting ERC20 tokens took %s\n", time.Since(start))

	return nil
}

// sendTransactions sends transactions for the load test.
func (e *ERC20Runner) sendTransactions() ([]types.Hash, error) {
	return e.BaseLoadTestRunner.sendTransactions(e.sendTransactionsForUser)
}

// sendTransactionsForUser sends ERC20 token transactions for a given user account.
// It takes an account pointer and a chainID as input parameters.
// It returns a slice of transaction hashes and an error if any.
func (e *ERC20Runner) sendTransactionsForUser(account *account, chainID *big.Int,
	bar *progressbar.ProgressBar) ([]types.Hash, []error, error) {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithClient(e.client),
		txrelayer.WithChainID(chainID),
		txrelayer.WithCollectTxnHashes(),
		txrelayer.WithNoWaiting(),
		txrelayer.WithEstimateGasFallback(),
		txrelayer.WithoutNonceGet(),
	)
	if err != nil {
		return nil, nil, err
	}

	var (
		gasPrice             *big.Int
		maxFeePerGas         *big.Int
		maxPriorityFeePerGas *big.Int
	)

	if e.cfg.DynamicTxs {
		mpfpg, err := e.client.MaxPriorityFeePerGas()
		if err != nil {
			return nil, nil, err
		}

		maxPriorityFeePerGas = new(big.Int).Mul(mpfpg, big.NewInt(2))

		feeHistory, err := e.client.FeeHistory(1, jsonrpc.LatestBlockNumber, nil)
		if err != nil {
			return nil, nil, err
		}

		baseFee := big.NewInt(0)

		if len(feeHistory.BaseFee) != 0 {
			baseFee = baseFee.SetUint64(feeHistory.BaseFee[len(feeHistory.BaseFee)-1])
		}

		maxFeePerGas = new(big.Int).Add(baseFee, mpfpg)
		maxFeePerGas.Mul(maxFeePerGas, big.NewInt(2))
	} else {
		gp, err := e.client.GasPrice()
		if err != nil {
			return nil, nil, err
		}

		gasPrice = new(big.Int).SetUint64(gp * 3)
	}

	sendErrs := make([]error, 0)

	for i := 0; i < e.cfg.TxsPerUser; i++ {
		input, err := e.erc20TokenArtifact.Abi.Methods["transfer"].Encode(map[string]interface{}{
			"receiver":  receiverAddr,
			"numTokens": big.NewInt(1),
		})
		if err != nil {
			return nil, nil, err
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
				types.WithGasPrice(gasPrice),
				types.WithFrom(account.key.Address()),
				types.WithInput(input),
			)), account.key)
		}

		if err != nil {
			sendErrs = append(sendErrs, err)
		}

		account.nonce++
		bar.Add(1)
	}

	return txRelayer.GetTxnHashes(), sendErrs, nil
}
