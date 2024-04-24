package polybft

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/signer"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo/abi"
)

const (
	contractCallGasLimit = 100_000_000
)

func initStakeManager(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	startValidators := make([]*contractsapi.GenesisValidator, len(polyBFTConfig.InitialValidatorSet))

	for i, validator := range polyBFTConfig.InitialValidatorSet {
		blsRaw, err := hex.DecodeHex(validator.BlsKey)
		if err != nil {
			return err
		}

		key, err := bls.UnmarshalPublicKey(blsRaw)
		if err != nil {
			return err
		}

		startValidators[i] = &contractsapi.GenesisValidator{
			Addr:   validator.Address,
			Stake:  validator.Stake,
			BlsKey: key.ToBigInt(),
		}

		approveFn := &contractsapi.ApproveNativeERC20MintableFn{
			Spender: contracts.StakeManagerContract,
			Amount:  validator.Stake,
		}

		input, err := approveFn.EncodeAbi()
		if err != nil {
			return fmt.Errorf("StakingERC20.approve params encoding failed: %w", err)
		}

		err = callContract(validator.Address, polyBFTConfig.StakeTokenAddr, input, "StakingERC20.approve", transition)
		if err != nil {
			return fmt.Errorf("error while calling contract %w", err)
		}
	}

	initFn := &contractsapi.InitializeStakeManagerFn{
		GenesisValidators: startValidators,
		NewStakingToken:   polyBFTConfig.StakeTokenAddr,
		NewBls:            contracts.BLSContract,
		EpochManager:      contracts.EpochManagerContract,
		NewDomain:         signer.DomainValidatorSetString,
		Owner:             polyBFTConfig.BladeAdmin,
		NetworkParams:     polyBFTConfig.GovernanceConfig.NetworkParamsAddr,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("StakeManager.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.StakeManagerContract, input, "StakeManager.initialize", transition)
}

// initEpochManager initializes EpochManager SC
func initEpochManager(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initFn := &contractsapi.InitializeEpochManagerFn{
		NewRewardToken:   polyBFTConfig.RewardConfig.TokenAddress,
		NewRewardWallet:  polyBFTConfig.RewardConfig.WalletAddress,
		NewStakeManager:  contracts.StakeManagerContract,
		NewNetworkParams: polyBFTConfig.GovernanceConfig.NetworkParamsAddr,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("EpochManager.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.EpochManagerContract, input, "EpochManager.initialize", transition)
}

// getInitERC20PredicateInput builds initialization input parameters for child chain ERC20Predicate SC
func getInitERC20PredicateInput(config *BridgeConfig, childChainMintable bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childChainMintable {
		params = &contractsapi.InitializeRootMintableERC20PredicateFn{
			NewL2StateSender:       contracts.L2StateSenderContract,
			NewStateReceiver:       contracts.StateReceiverContract,
			NewChildERC20Predicate: config.ChildMintableERC20PredicateAddr,
			NewChildTokenTemplate:  config.ChildERC20Addr,
		}
	} else {
		params = &contractsapi.InitializeChildERC20PredicateFn{
			NewL2StateSender:          contracts.L2StateSenderContract,
			NewStateReceiver:          contracts.StateReceiverContract,
			NewRootERC20Predicate:     config.RootERC20PredicateAddr,
			NewChildTokenTemplate:     contracts.ChildERC20Contract,
			NewNativeTokenRootAddress: config.RootNativeERC20Addr,
		}
	}

	return params.EncodeAbi()
}

// getInitERC20PredicateACLInput builds initialization input parameters for child chain ERC20PredicateAccessList SC
func getInitERC20PredicateACLInput(config *BridgeConfig, owner types.Address,
	useAllowList, useBlockList, childChainMintable bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childChainMintable {
		params = &contractsapi.InitializeRootMintableERC20PredicateACLFn{
			NewL2StateSender:       contracts.L2StateSenderContract,
			NewStateReceiver:       contracts.StateReceiverContract,
			NewChildERC20Predicate: config.ChildMintableERC20PredicateAddr,
			NewChildTokenTemplate:  config.ChildERC20Addr,
			NewUseAllowList:        useAllowList,
			NewUseBlockList:        useBlockList,
			NewOwner:               owner,
		}
	} else {
		params = &contractsapi.InitializeChildERC20PredicateACLFn{
			NewL2StateSender:          contracts.L2StateSenderContract,
			NewStateReceiver:          contracts.StateReceiverContract,
			NewRootERC20Predicate:     config.RootERC20PredicateAddr,
			NewChildTokenTemplate:     contracts.ChildERC20Contract,
			NewNativeTokenRootAddress: config.RootNativeERC20Addr,
			NewUseAllowList:           useAllowList,
			NewUseBlockList:           useBlockList,
			NewOwner:                  owner,
		}
	}

	return params.EncodeAbi()
}

// getInitERC721PredicateInput builds initialization input parameters for child chain ERC721Predicate SC
func getInitERC721PredicateInput(config *BridgeConfig, childOriginatedTokens bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childOriginatedTokens {
		params = &contractsapi.InitializeRootMintableERC721PredicateFn{
			NewL2StateSender:        contracts.L2StateSenderContract,
			NewStateReceiver:        contracts.StateReceiverContract,
			NewChildERC721Predicate: config.ChildMintableERC721PredicateAddr,
			NewChildTokenTemplate:   config.ChildERC721Addr,
		}
	} else {
		params = &contractsapi.InitializeChildERC721PredicateFn{
			NewL2StateSender:       contracts.L2StateSenderContract,
			NewStateReceiver:       contracts.StateReceiverContract,
			NewRootERC721Predicate: config.RootERC721PredicateAddr,
			NewChildTokenTemplate:  contracts.ChildERC721Contract,
		}
	}

	return params.EncodeAbi()
}

// getInitERC721PredicateACLInput builds initialization input parameters
// for child chain ERC721PredicateAccessList SC
func getInitERC721PredicateACLInput(config *BridgeConfig, owner types.Address,
	useAllowList, useBlockList, childChainMintable bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childChainMintable {
		params = &contractsapi.InitializeRootMintableERC721PredicateACLFn{
			NewL2StateSender:        contracts.L2StateSenderContract,
			NewStateReceiver:        contracts.StateReceiverContract,
			NewChildERC721Predicate: config.ChildMintableERC721PredicateAddr,
			NewChildTokenTemplate:   config.ChildERC721Addr,
			NewUseAllowList:         useAllowList,
			NewUseBlockList:         useBlockList,
			NewOwner:                owner,
		}
	} else {
		params = &contractsapi.InitializeChildERC721PredicateACLFn{
			NewL2StateSender:       contracts.L2StateSenderContract,
			NewStateReceiver:       contracts.StateReceiverContract,
			NewRootERC721Predicate: config.RootERC721PredicateAddr,
			NewChildTokenTemplate:  contracts.ChildERC721Contract,
			NewUseAllowList:        useAllowList,
			NewUseBlockList:        useBlockList,
			NewOwner:               owner,
		}
	}

	return params.EncodeAbi()
}

// getInitERC1155PredicateInput builds initialization input parameters for child chain ERC1155Predicate SC
func getInitERC1155PredicateInput(config *BridgeConfig, childChainMintable bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childChainMintable {
		params = &contractsapi.InitializeRootMintableERC1155PredicateFn{
			NewL2StateSender:         contracts.L2StateSenderContract,
			NewStateReceiver:         contracts.StateReceiverContract,
			NewChildERC1155Predicate: config.ChildMintableERC1155PredicateAddr,
			NewChildTokenTemplate:    config.ChildERC1155Addr,
		}
	} else {
		params = &contractsapi.InitializeChildERC1155PredicateFn{
			NewL2StateSender:        contracts.L2StateSenderContract,
			NewStateReceiver:        contracts.StateReceiverContract,
			NewRootERC1155Predicate: config.RootERC1155PredicateAddr,
			NewChildTokenTemplate:   contracts.ChildERC1155Contract,
		}
	}

	return params.EncodeAbi()
}

// getInitERC1155PredicateACLInput builds initialization input parameters
// for child chain ERC1155PredicateAccessList SC
func getInitERC1155PredicateACLInput(config *BridgeConfig, owner types.Address,
	useAllowList, useBlockList, childChainMintable bool) ([]byte, error) {
	var params contractsapi.StateTransactionInput
	if childChainMintable {
		params = &contractsapi.InitializeRootMintableERC1155PredicateACLFn{
			NewL2StateSender:         contracts.L2StateSenderContract,
			NewStateReceiver:         contracts.StateReceiverContract,
			NewChildERC1155Predicate: config.ChildMintableERC1155PredicateAddr,
			NewChildTokenTemplate:    config.ChildERC1155Addr,
			NewUseAllowList:          useAllowList,
			NewUseBlockList:          useBlockList,
			NewOwner:                 owner,
		}
	} else {
		params = &contractsapi.InitializeChildERC1155PredicateACLFn{
			NewL2StateSender:        contracts.L2StateSenderContract,
			NewStateReceiver:        contracts.StateReceiverContract,
			NewRootERC1155Predicate: config.RootERC1155PredicateAddr,
			NewChildTokenTemplate:   contracts.ChildERC1155Contract,
			NewUseAllowList:         useAllowList,
			NewUseBlockList:         useBlockList,
			NewOwner:                owner,
		}
	}

	return params.EncodeAbi()
}

// initNetworkParamsContract initializes NetworkParams contract on child chain
func initNetworkParamsContract(baseFeeChangeDenom uint64, cfg PolyBFTConfig,
	transition *state.Transition) error {
	initFn := &contractsapi.InitializeNetworkParamsFn{
		InitParams: &contractsapi.InitParams{
			// only timelock controller can execute transactions on network params
			// so we set it as its owner
			NewOwner:                   cfg.GovernanceConfig.ChildTimelockAddr,
			NewCheckpointBlockInterval: new(big.Int).SetUint64(cfg.CheckpointInterval),
			NewSprintSize:              new(big.Int).SetUint64(cfg.SprintSize),
			NewEpochSize:               new(big.Int).SetUint64(cfg.EpochSize),
			NewEpochReward:             new(big.Int).SetUint64(cfg.EpochReward),
			NewMinValidatorSetSize:     new(big.Int).SetUint64(cfg.MinValidatorSetSize),
			NewMaxValidatorSetSize:     new(big.Int).SetUint64(cfg.MaxValidatorSetSize),
			NewWithdrawalWaitPeriod:    new(big.Int).SetUint64(cfg.WithdrawalWaitPeriod),
			NewBlockTime:               new(big.Int).SetUint64(uint64(cfg.BlockTime.Duration)),
			NewBlockTimeDrift:          new(big.Int).SetUint64(cfg.BlockTimeDrift),
			NewVotingDelay:             new(big.Int).Set(cfg.GovernanceConfig.VotingDelay),
			NewVotingPeriod:            new(big.Int).Set(cfg.GovernanceConfig.VotingPeriod),
			NewProposalThreshold:       new(big.Int).Set(cfg.GovernanceConfig.ProposalThreshold),
			NewBaseFeeChangeDenom:      new(big.Int).SetUint64(baseFeeChangeDenom),
		},
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("NetworkParams.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		cfg.GovernanceConfig.NetworkParamsAddr, input, "NetworkParams.initialize", transition)
}

// initForkParamsContract initializes ForkParams contract on child chain
func initForkParamsContract(cfg PolyBFTConfig, transition *state.Transition) error {
	initFn := &contractsapi.InitializeForkParamsFn{
		NewOwner: cfg.GovernanceConfig.ChildTimelockAddr,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("ForkParams.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		cfg.GovernanceConfig.ForkParamsAddr, input, "ForkParams.initialize", transition)
}

// initChildTimelock initializes ChildTimelock contract on child chain
func initChildTimelock(cfg PolyBFTConfig, transition *state.Transition) error {
	addresses := make([]types.Address, len(cfg.InitialValidatorSet)+1)
	// we need to add child governor to list of proposers and executors as well
	addresses[0] = cfg.GovernanceConfig.ChildGovernorAddr

	for i := 0; i < len(cfg.InitialValidatorSet); i++ {
		addresses[i+1] = cfg.InitialValidatorSet[i].Address
	}

	initFn := &contractsapi.InitializeChildTimelockFn{
		Admin:     cfg.BladeAdmin,
		Proposers: addresses,
		Executors: addresses,
		MinDelay:  big.NewInt(1), // for now
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("ChildTimelock.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		cfg.GovernanceConfig.ChildTimelockAddr, input, "ChildTimelock.initialize", transition)
}

// initChildGovernor initializes ChildGovernor contract on child chain
func initChildGovernor(cfg PolyBFTConfig, transition *state.Transition) error {
	addresses := make([]types.Address, len(cfg.InitialValidatorSet))
	for i := 0; i < len(cfg.InitialValidatorSet); i++ {
		addresses[i] = cfg.InitialValidatorSet[i].Address
	}

	initFn := &contractsapi.InitializeChildGovernorFn{
		Token_:           contracts.StakeManagerContract,
		Timelock_:        cfg.GovernanceConfig.ChildTimelockAddr,
		NetworkParams:    cfg.GovernanceConfig.NetworkParamsAddr,
		QuorumNumerator_: new(big.Int).SetUint64(cfg.GovernanceConfig.ProposalQuorumPercentage),
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("ChildGovernor.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		cfg.GovernanceConfig.ChildGovernorAddr, input, "ChildGovernor.initialize", transition)
}

// mintRewardTokensToWallet mints configured amount of reward tokens to reward wallet address
func mintRewardTokensToWallet(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	if isNativeRewardToken(polyBFTConfig) {
		// if reward token is a native erc20 token, we don't need to mint an amount of tokens
		// for given wallet address to it since this is done in premine
		return nil
	}

	mintFn := contractsapi.MintRootERC20Fn{
		To:     polyBFTConfig.RewardConfig.WalletAddress,
		Amount: polyBFTConfig.RewardConfig.WalletAmount,
	}

	input, err := mintFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("RewardERC20Token.mint params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller, polyBFTConfig.RewardConfig.TokenAddress, input,
		"RewardERC20Token.mint", transition)
}

// mintStakeToken mints configured amount of stake token to stake token address
func mintStakeToken(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	if IsNativeStakeToken(polyBFTConfig.StakeTokenAddr) {
		return nil
	}

	for _, validator := range polyBFTConfig.InitialValidatorSet {
		mintFn := contractsapi.MintRootERC20Fn{
			To:     validator.Address,
			Amount: validator.Stake,
		}

		input, err := mintFn.EncodeAbi()
		if err != nil {
			return fmt.Errorf("StakingERC20.mint params encoding failed: %w", err)
		}

		if err := callContract(polyBFTConfig.BladeAdmin, polyBFTConfig.StakeTokenAddr,
			input, "StakingERC20.mint", transition); err != nil {
			return err
		}
	}

	return nil
}

// approveEpochManagerAsSpender approves EpochManager contract as reward token spender
// since EpochManager distributes rewards
func approveEpochManagerAsSpender(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	approveFn := &contractsapi.ApproveRootERC20Fn{
		Spender: contracts.EpochManagerContract,
		Amount:  polyBFTConfig.RewardConfig.WalletAmount,
	}

	input, err := approveFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("RewardToken.approve params encoding failed: %w", err)
	}

	return callContract(polyBFTConfig.RewardConfig.WalletAddress,
		polyBFTConfig.RewardConfig.TokenAddress, input, "RewardToken.approve", transition)
}

// callContract calls given smart contract function, encoded in input parameter
func callContract(from, to types.Address, input []byte, contractName string, transition *state.Transition) error {
	result := transition.Call2(from, to, input, big.NewInt(0), contractCallGasLimit)
	if result.Failed() {
		if result.Reverted() {
			if revertReason, err := abi.UnpackRevertError(result.ReturnValue); err == nil {
				return fmt.Errorf("%s contract call was reverted: %s", contractName, revertReason)
			}
		}

		return fmt.Errorf("%s contract call failed: %w", contractName, result.Err)
	}

	return nil
}

// isNativeRewardToken returns true in case a native token is used as a reward token as well
func isNativeRewardToken(cfg PolyBFTConfig) bool {
	return cfg.RewardConfig.TokenAddress == contracts.NativeERC20TokenContract
}

// IsNativeStakeToken return true in case a native token is used for staking
func IsNativeStakeToken(stakeTokenAddr types.Address) bool {
	return stakeTokenAddr == contracts.NativeERC20TokenContract
}

// initProxies initializes proxy contracts, that allow upgradeability of contracts implementation
func initApexProxies(transition *state.Transition, admin types.Address,
	proxyToImplMap map[types.Address]types.Address, polyBFTConfig PolyBFTConfig) error {
	for proxyAddress, implAddress := range proxyToImplMap {
		protectSetupProxyFn := &contractsapi.ProtectSetUpProxyGenesisProxyFn{Initiator: contracts.SystemCaller}

		proxyInput, err := protectSetupProxyFn.EncodeAbi()
		if err != nil {
			return fmt.Errorf("GenesisProxy.protectSetUpProxy params encoding failed: %w", err)
		}

		err = callContract(contracts.SystemCaller, proxyAddress, proxyInput, "GenesisProxy.protectSetUpProxy", transition)
		if err != nil {
			return err
		}

		data, err := getDataForApexContract(proxyAddress, polyBFTConfig)
		if err != nil {
			return fmt.Errorf("initialize encoding for %v proxy failed: %w", proxyAddress, err)
		}

		setUpproxyFn := &contractsapi.SetUpProxyGenesisProxyFn{
			Logic: implAddress,
			Admin: admin,
			Data:  data,
		}

		proxyInput, err = setUpproxyFn.EncodeAbi()
		if err != nil {
			return fmt.Errorf("apex GenesisProxy.setUpProxy params encoding failed: %w", err)
		}

		err = callContract(contracts.SystemCaller, proxyAddress, proxyInput, "GenesisProxy.setUpProxy", transition)
		if err != nil {
			return err
		}
	}

	return nil
}

func getDataForApexContract(contract types.Address, polyBFTConfig PolyBFTConfig) ([]byte, error) {
	switch contract {
	case contracts.Bridge:
		return (&contractsapi.InitializeBridgeFn{}).EncodeAbi()
	case contracts.SignedBatches:
		return (&contractsapi.InitializeSignedBatchesFn{}).EncodeAbi()
	case contracts.ClaimsHelper:
		return (&contractsapi.InitializeClaimsHelperFn{}).EncodeAbi()
	case contracts.Validators:
		var validatorAddresses = make([]types.Address, len(polyBFTConfig.InitialValidatorSet))
		for i, validator := range polyBFTConfig.InitialValidatorSet {
			validatorAddresses[i] = validator.Address
		}

		return (&contractsapi.InitializeValidatorsFn{
			Validators: validatorAddresses,
		}).EncodeAbi()
	case contracts.Slots:
		return (&contractsapi.InitializeSlotsFn{}).EncodeAbi()
	case contracts.Claims:
		return (&contractsapi.InitializeClaimsFn{
			MaxNumberOfTransactions: 0, // APEX-TODO: Define
			TimeoutBlocksNumber:     0,
		}).EncodeAbi()
	case contracts.UTXOsc:
		return (&contractsapi.InitializeUTXOscFn{}).EncodeAbi()
	}

	return nil, fmt.Errorf("no contract defined at address %v", contract)
}

// Apex smart contracts initialization

// initBridge initializes Bridge and it's proxy SC
func initBridge(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesBridgeFn{
		ClaimsAddress:        contracts.Claims,
		SignedBatchesAddress: contracts.SignedBatches,
		SlotsAddress:         contracts.Slots,
		UtxoscAddress:        contracts.UTXOsc,
		ValidatorsAddress:    contracts.Validators,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("Bridge.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.Bridge, input, "Bridge.setDependencies", transition)
}

// initSignedBatches initializes SignedBatches SC
func initSignedBatches(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesSignedBatchesFn{
		BridgeAddress:       contracts.Bridge,
		ClaimsHelperAddress: contracts.ClaimsHelper,
		ValidatorsAddress:   contracts.Validators,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("SignedBatches.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.SignedBatches, input, "SignedBatches.setDependencies", transition)
}

// initClaimsHelper initializes ClaimsHelper SC
func initClaimsHelper(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesClaimsHelperFn{
		ClaimsAddress:        contracts.Claims,
		SignedBatchesAddress: contracts.SignedBatches,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("ClaimsHelper.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.ClaimsHelper, input, "ClaimsHelper.setDependencies", transition)
}

// initValidators initializes Validators SC
func initValidators(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesValidatorsFn{
		BridgeAddress: contracts.Bridge,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("Validators.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.Validators, input, "Validators.setDependencies", transition)
}

// initSlots initializes Slots SC
func initSlots(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesSlotsFn{
		BridgeAddress:     contracts.Bridge,
		ValidatorsAddress: contracts.Validators,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("Slots.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.Slots, input, "Slots.setDependencies", transition)
}

// initClaims initializes Claims SC
func initClaims(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesClaimsFn{
		BridgeAddress:       contracts.Bridge,
		ClaimsHelperAddress: contracts.ClaimsHelper,
		Utxosc:              contracts.UTXOsc,
		ValidatorsAddress:   contracts.Validators,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("Claims.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.Claims, input, "Claims.setDependencies", transition)
}

// initUTXOsc initializes UTXOsc SC
func initUTXOsc(transition *state.Transition) error {
	setDependenciesFn := &contractsapi.SetDependenciesUTXOscFn{
		BridgeAddress: contracts.Bridge,
		ClaimsAddress: contracts.Claims,
	}

	input, err := setDependenciesFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("UTXOsc.setDependencies params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.UTXOsc, input, "UTXOsc.setDependencies", transition)
}
