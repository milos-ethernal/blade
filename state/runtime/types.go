package runtime

import (
	"math/big"

	"github.com/0xPolygon/polygon-edge/types"
)

var (
	CallTypes = map[int]string{
		0: "CALL",
		1: "CALLCODE",
		2: "DELEGATECALL",
		3: "STATICCALL",
		4: "CREATE",
		5: "CREATE2",
	}
)

// TracerContext contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type TracerContext struct {
	BlockHash   types.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int   // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int        // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      types.Hash // Hash of the transaction being traced (zero if dangling call)
}

// ScopeContext contains the things that are per-call, such as stack and memory,
// but not transients like pc and gas
type ScopeContext struct {
	Memory       []byte
	Stack        []*big.Int
	Sp           int
	ContractAddr types.Address
}

type VMState interface {
	// Halt tells VM to terminate its process
	Halt()
	GetContract() *Contract
}

type Tracer interface {
	// Cancel tells termination of execution and tracing
	Cancel(error)
	// Clear clears the tracked data
	Clear()
	// GetResult returns a result based on tracked data
	GetResult() (interface{}, error)

	// Tx-level
	TxStart(gasLimit uint64)
	TxEnd(gasLeft uint64)

	// Top call level
	CaptureStart(
		from, to types.Address,
		callType int,
		input []byte,
		gas uint64,
		value *big.Int,
		host Host)

	CaptureEnd(output []byte, gasUsed uint64, err error)

	// Call-level
	CallStart(
		depth int, // begins from 1
		from, to types.Address,
		callType int,
		gas uint64,
		value *big.Int,
		input []byte,
		host Host,
	)
	CallEnd(
		depth int, // begins from 1
		gasUsed uint64,
		output []byte,
		err error,
	)

	// Op-level
	CaptureState(
		memory []byte,
		stack []*big.Int,
		opCode int,
		contractAddress types.Address,
		sp int,
		host Host,
		state VMState,
	)
	ExecuteState(
		contractAddress types.Address,
		ip uint64,
		opcode int,
		availableGas uint64,
		cost uint64,
		lastReturnData []byte,
		depth int,
		err error,
		host Host,
	)
	CaptureStateBre(
		opCode, depth int,
		ip, gas, cost uint64,
		returnData []byte,
		scope *ScopeContext,
		host Host,
		state VMState,
		err error,
	)
}
