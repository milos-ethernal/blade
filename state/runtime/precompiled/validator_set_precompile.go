package precompiled

import (
	"errors"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/state/runtime"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
)

var (
	hasQuorumAbiType = abi.MustNewType("address[]")

	errValidatorSetPrecompileNotEnabled = errors.New("validator set precompile is not enabled")
)

type ValidatoSetPrecompileBackend interface {
	GetValidatorsForBlock(blockNumber uint64) (validator.AccountSet, error)
}

type validatorSetPrecompile struct {
	backend ValidatoSetPrecompileBackend
}

// gas returns the gas required to execute the pre-compiled contract
func (c *validatorSetPrecompile) gas(input []byte, _ *chain.ForksInTime) uint64 {
	return 150000
}

// Run runs the precompiled contract with the given input.
// There are two functions:
// isValidator(address addr) bool
// hasConsensus(address[] addrs) bool
// Input must be ABI encoded: address or (address[])
// Output could be an error or ABI encoded "bool" value
func (c *validatorSetPrecompile) run(input []byte, caller types.Address, host runtime.Host) ([]byte, error) {
	// isValidator case
	if len(input) == 32 {
		validatorSet, err := createValidatorSet(host, c.backend) // we are calling validators for previous block
		if err != nil {
			return nil, err
		}

		addr := types.BytesToAddress(input[0:32])

		if validatorSet.Includes(addr) {
			return abiBoolTrue, nil
		}

		return abiBoolFalse, nil
	}

	rawData, err := abi.Decode(hasQuorumAbiType, input)
	if err != nil {
		return nil, err
	}

	addresses, ok := rawData.([]ethgo.Address)
	if !ok {
		return nil, errBLSVerifyAggSignsInputs
	}

	validatorSet, err := createValidatorSet(host, c.backend)
	if err != nil {
		return nil, err
	}

	signers := make(map[types.Address]struct{}, len(addresses))
	for _, x := range addresses {
		signers[types.Address(x)] = struct{}{}
	}

	if validatorSet.HasQuorum(uint64(host.GetTxContext().Number), signers) {
		return abiBoolTrue, nil
	}

	return abiBoolFalse, nil
}

func createValidatorSet(host runtime.Host, backend ValidatoSetPrecompileBackend) (validator.ValidatorSet, error) {
	if backend == nil {
		return nil, errValidatorSetPrecompileNotEnabled
	}

	// if its payable tx we need to look for validator in previous block
	blockNumber := uint64(host.GetTxContext().Number)
	if !host.GetTxContext().NonPayable {
		blockNumber--
	}

	accounts, err := backend.GetValidatorsForBlock(blockNumber)
	if err != nil {
		return nil, err
	}

	return validator.NewValidatorSet(accounts, hclog.NewNullLogger()), nil
}
