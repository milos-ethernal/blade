package runner

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/jsonrpc"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
	"golang.org/x/sync/errgroup"
)

// EOARunner represents a runner for executing load tests specific to EOAs (Externally Owned Accounts).
type EOARunner struct {
	*BaseLoadTestRunner
}

// NewEOARunner creates a new EOARunner instance with the given LoadTestConfig.
// It returns a pointer to the created EOARunner and an error, if any.
func NewEOARunner(cfg LoadTestConfig) (*EOARunner, error) {
	runner, err := NewBaseLoadTestRunner(cfg)
	if err != nil {
		return nil, err
	}

	return &EOARunner{runner}, nil
}

// Run executes the load test by creating virtual users, funding them,
// sending transactions, waiting for the transaction pool to empty,
// waiting for transaction receipts, and calculating the transactions
// per second (TPS) based on the block information and transaction statistics.
// It returns an error if any of the steps fail.
func (e *EOARunner) Run() error {
	if err := e.createVUs(); err != nil {
		return err
	}

	if err := e.fundVUs(); err != nil {
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

// sendTransactions sends transactions for each virtual user (vu) and returns the transaction hashes.
// It retrieves the chain ID from the client and uses it to send transactions for each user.
// The function runs concurrently for each user using errgroup.
// If the context is canceled, the function returns the context error.
// The transaction hashes are appended to the allTxnHashes slice.
// Finally, the function prints the time taken to send the transactions and returns the transaction hashes and nil error.
func (e *EOARunner) sendTransactions() ([]types.Hash, error) {
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

// sendTransactionsForUser sends multiple transactions for a user account on a specific chain.
// It uses the provided client and chain ID to send transactions using either dynamic or legacy fee models.
// For each transaction, it increments the account's nonce and returns the transaction hashes.
// If an error occurs during the transaction sending process, it returns the error.
func (e *EOARunner) sendTransactionsForUser(account *account, chainID *big.Int) ([]types.Hash, error) {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithClient(e.client),
		txrelayer.WithChainID(chainID),
		txrelayer.WithCollectTxnHashes(),
		txrelayer.WithNoWaiting(),
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
		var err error
		if e.cfg.DynamicTxs {
			_, err = txRelayer.SendTransaction(types.NewTx(types.NewDynamicFeeTx(
				types.WithNonce(account.nonce),
				types.WithTo(&receiverAddr),
				types.WithValue(ethgo.Gwei(1)),
				types.WithGas(21000),
				types.WithFrom(account.key.Address()),
				types.WithGasFeeCap(maxFeePerGas),
				types.WithGasTipCap(maxPriorityFeePerGas),
				types.WithChainID(chainID),
			)), account.key)
		} else {
			_, err = txRelayer.SendTransaction(types.NewTx(types.NewLegacyTx(
				types.WithNonce(account.nonce),
				types.WithTo(&receiverAddr),
				types.WithValue(ethgo.Gwei(1)),
				types.WithGas(21000),
				types.WithGasPrice(bigGasPrice),
				types.WithFrom(account.key.Address()),
			)), account.key)
		}

		if err != nil {
			return nil, err
		}

		account.nonce++
	}

	return txRelayer.GetTxnHashes(), nil
}
