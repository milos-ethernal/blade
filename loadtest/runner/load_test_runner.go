package runner

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/0xPolygon/polygon-edge/crypto"
	"github.com/0xPolygon/polygon-edge/types"
)

const (
	EOATestType    = "eoa"
	ERC20TestType  = "erc20"
	ERC721TestType = "erc721"
)

var receiverAddr = types.StringToAddress("0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF")

type account struct {
	nonce uint64
	key   *crypto.ECDSAKey
}

type txStats struct {
	hash  types.Hash
	block uint64
}

type blockInfo struct {
	number    uint64
	createdAt uint64
	numTxs    int

	gasUsed        *big.Int
	gasLimit       *big.Int
	gasUtilization float64
}

// LoadTestConfig represents the configuration for a load test.
type LoadTestConfig struct {
	Mnemonnic string // Mnemonnic is the mnemonic phrase used for account generation, and VUs funding.

	LoadTestType string // LoadTestType is the type of load test.
	LoadTestName string // LoadTestName is the name of the load test.

	JSONRPCUrl      string        // JSONRPCUrl is the URL of the JSON-RPC server.
	ReceiptsTimeout time.Duration // ReceiptsTimeout is the timeout for waiting for transaction receipts.
	TxPoolTimeout   time.Duration // TxPoolTimeout is the timeout for waiting for tx pool to empty.

	VUs        int  // VUs is the number of virtual users.
	TxsPerUser int  // TxsPerUser is the number of transactions per user.
	DynamicTxs bool // DynamicTxs indicates whether the load test should generate dynamic transactions.
}

// LoadTestRunner represents a runner for load tests.
type LoadTestRunner struct{}

// Run executes the load test based on the provided LoadTestConfig.
// It determines the load test type from the configuration and creates
// the corresponding runner. Then, it runs the load test using the
// created runner and returns any error encountered during the process.
func (r *LoadTestRunner) Run(cfg LoadTestConfig) error {
	switch strings.ToLower(cfg.LoadTestType) {
	case EOATestType:
		eoaRunner, err := NewEOARunner(cfg)
		if err != nil {
			return err
		}

		return eoaRunner.Run()
	case ERC20TestType:
		erc20Runner, err := NewERC20Runner(cfg)
		if err != nil {
			return err
		}

		return erc20Runner.Run()
	default:
		return fmt.Errorf("unknown load test type %s", cfg.LoadTestType)
	}
}
