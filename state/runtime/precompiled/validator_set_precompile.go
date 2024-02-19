package precompiled

import (
	"encoding/binary"
	"errors"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/state/runtime"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
)

const addrOffset = 32 - types.AddressLength

var (
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
	// if its payable tx we need to look for validator in previous block
	blockNumber := uint64(host.GetTxContext().Number)
	if !host.GetTxContext().NonPayable {
		blockNumber--
	}

	// isValidator case
	if len(input) == 32 {
		validatorSet, err := createValidatorSet(blockNumber, c.backend) // we are calling validators for previous block
		if err != nil {
			return nil, err
		}

		addr := types.BytesToAddress(input[0:32])

		if validatorSet.Includes(addr) {
			return abiBoolTrue, nil
		}

		return abiBoolFalse, nil
	}

	addresses, err := abiDecodeAddresses(input)
	if err != nil {
		return nil, err
	}

	validatorSet, err := createValidatorSet(blockNumber, c.backend)
	if err != nil {
		return nil, err
	}

	signers := make(map[types.Address]struct{}, len(addresses))
	for _, x := range addresses {
		signers[x] = struct{}{}
	}

	if validatorSet.HasQuorum(blockNumber, signers) {
		return abiBoolTrue, nil
	}

	return abiBoolFalse, nil
}

func createValidatorSet(blockNumber uint64, backend ValidatoSetPrecompileBackend) (validator.ValidatorSet, error) {
	if backend == nil {
		return nil, errValidatorSetPrecompileNotEnabled
	}

	accounts, err := backend.GetValidatorsForBlock(blockNumber)
	if err != nil {
		return nil, err
	}

	return validator.NewValidatorSet(accounts, hclog.NewNullLogger()), nil
}

func abiDecodeAddresses(input []byte) ([]types.Address, error) {
	if len(input) < 64 || len(input)%32 != 0 {
		return nil, runtime.ErrInvalidInputData
	}

	// abi.encode encodes addresses[] with slice of bytes where initial 31 bytes
	// are set to 0, and 32nd is 32
	// then goes length of slice (32 bytes)
	// then each address is 32 bytes
	dummy := [32]byte{}
	dummy[31] = 32

	input = input[32:]

	size := binary.BigEndian.Uint32(input[28:32])
	if uint32(len(input)) != size*32+32 {
		return nil, runtime.ErrInvalidInputData
	}

	res := make([]types.Address, size)
	for i, offset := 0, 32; offset < len(input); i, offset = i+1, offset+32 {
		res[i] = types.Address(input[offset+addrOffset : offset+32])
	}

	return res, nil
}

func abiEncodeAddresses(addrs []types.Address) []byte {
	res := make([]byte, len(addrs)*32+64)
	res[31] = 32

	binary.BigEndian.PutUint32(res[60:64], uint32(len(addrs))) // 60 == 32 + 28

	offset := 64

	for _, addr := range addrs {
		copy(res[offset+addrOffset:offset+32], addr.Bytes())
		offset += 32
	}

	return res
}
