package e2e

import (
	"bytes"
	"fmt"
	"math/big"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/genesis"
	validatorHelper "github.com/0xPolygon/polygon-edge/command/validator/helper"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/crypto"
	"github.com/0xPolygon/polygon-edge/e2e-polybft/framework"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/jsonrpc"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	cardano_wallet "github.com/Ethernal-Tech/cardano-infrastructure/wallet"
)

var uint256ABIType = abi.MustNewType("tuple(uint256)")

func TestE2E_Consensus_Basic(t *testing.T) {
	const (
		epochSize     = 4
		validatorsNum = 5
	)

	cluster := framework.NewTestCluster(t, validatorsNum,
		framework.WithEpochSize(epochSize),
		framework.WithNonValidators(2),
	)
	defer cluster.Stop()

	cluster.WaitForReady(t)

	// initialize tx relayer
	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(cluster.Servers[0].JSONRPC()))
	require.NoError(t, err)

	initialTotalSupply := new(big.Int).Mul(big.NewInt(validatorsNum+1 /*because of reward token*/),
		command.DefaultPremineBalance)

	// check if initial total supply of native ERC20 token is the same as expected
	totalSupply := queryNativeERC20Metadata(t, "totalSupply", uint256ABIType, relayer)
	require.True(t, initialTotalSupply.Cmp(totalSupply.(*big.Int)) == 0)

	t.Run("consensus protocol", func(t *testing.T) {
		require.NoError(t, cluster.WaitForBlock(2*epochSize+1, 1*time.Minute))
	})

	t.Run("sync protocol, drop single validator node", func(t *testing.T) {
		// query the current block number, as it is a starting point for the test
		currentBlockNum, err := cluster.Servers[0].JSONRPC().BlockNumber()
		require.NoError(t, err)

		// stop one node
		node := cluster.Servers[0]
		node.Stop()

		// wait for 2 epochs to elapse, so that rest of the network progresses
		require.NoError(t, cluster.WaitForBlock(currentBlockNum+2*epochSize, 2*time.Minute))

		// start the node again
		node.Start()

		// wait 2 more epochs to elapse and make sure that stopped node managed to catch up
		require.NoError(t, cluster.WaitForBlock(currentBlockNum+4*epochSize, 2*time.Minute))
	})

	t.Run("sync protocol, drop single non-validator node", func(t *testing.T) {
		// query the current block number, as it is a starting point for the test
		currentBlockNum, err := cluster.Servers[0].JSONRPC().BlockNumber()
		require.NoError(t, err)

		// stop one non-validator node
		node := cluster.Servers[6]
		node.Stop()

		// wait for 2 epochs to elapse, so that rest of the network progresses
		require.NoError(t, cluster.WaitForBlock(currentBlockNum+2*epochSize, 2*time.Minute))

		// start the node again
		node.Start()

		// wait 2 more epochs to elapse and make sure that stopped node managed to catch up
		require.NoError(t, cluster.WaitForBlock(currentBlockNum+4*epochSize, 2*time.Minute))
	})
}

func TestE2E_Consensus_BulkDrop(t *testing.T) {
	const (
		clusterSize = 5
		bulkToDrop  = 4
		epochSize   = 5
	)

	cluster := framework.NewTestCluster(t, clusterSize,
		framework.WithEpochSize(epochSize),
		framework.WithBlockTime(time.Second),
	)
	defer cluster.Stop()

	// wait for cluster to start
	cluster.WaitForReady(t)

	var wg sync.WaitGroup
	// drop bulk of nodes from cluster
	for i := 0; i < bulkToDrop; i++ {
		node := cluster.Servers[i]

		wg.Add(1)

		go func(node *framework.TestServer) {
			defer wg.Done()
			node.Stop()
		}(node)
	}

	wg.Wait()

	// start dropped nodes again
	for i := 0; i < bulkToDrop; i++ {
		node := cluster.Servers[i]
		node.Start()
	}

	// wait to proceed to the 2nd epoch
	require.NoError(t, cluster.WaitForBlock(epochSize+1, 2*time.Minute))
}

func TestE2E_Consensus_RegisterValidator(t *testing.T) {
	const (
		validatorSetSize = 5
		epochSize        = 5
	)

	var (
		firstValidatorDataDir  = fmt.Sprintf("test-chain-%d", validatorSetSize+1) // directory where the first validator secrets will be stored
		secondValidatorDataDir = fmt.Sprintf("test-chain-%d", validatorSetSize+2) // directory where the second validator secrets will be stored

		premineBalance = ethgo.Ether(2e6) // 2M native tokens (so that we have enough balance to fund new validator)
		stakeAmount    = ethgo.Ether(500)
	)

	// start cluster with 'validatorSize' validators
	cluster := framework.NewTestCluster(t, validatorSetSize,
		framework.WithEpochSize(epochSize),
		framework.WithEpochReward(int(ethgo.Ether(1).Uint64())),
		framework.WithSecretsCallback(func(addresses []types.Address, config *framework.TestClusterConfig) {
			for _, a := range addresses {
				config.Premine = append(config.Premine, fmt.Sprintf("%s:%s", a, premineBalance))
				config.StakeAmounts = append(config.StakeAmounts, stakeAmount)
			}
		}),
	)
	defer cluster.Stop()

	cluster.WaitForReady(t)

	// first validator is the owner of ChildValidator set smart contract
	owner := cluster.Servers[0]
	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(owner.JSONRPCAddr()))
	require.NoError(t, err)

	polybftConfig, err := polybft.LoadPolyBFTConfig(path.Join(cluster.Config.TmpDir, chainConfigFileName))
	require.NoError(t, err)

	// create the first account and extract the address
	addrs, err := cluster.InitSecrets(firstValidatorDataDir, 1)
	require.NoError(t, err)

	firstValidatorAddr := addrs[0]

	// create the second account and extract the address
	addrs, err = cluster.InitSecrets(secondValidatorDataDir, 1)
	require.NoError(t, err)

	secondValidatorAddr := addrs[0]

	// assert that accounts are created
	validatorSecrets, err := genesis.GetValidatorKeyFiles(cluster.Config.TmpDir, cluster.Config.ValidatorPrefix)
	require.NoError(t, err)
	require.Equal(t, validatorSetSize+2, len(validatorSecrets))

	genesisBlock, err := owner.JSONRPC().GetBlockByNumber(0, false)
	require.NoError(t, err)

	_, err = polybft.GetIbftExtra(genesisBlock.Header.ExtraData)
	require.NoError(t, err)

	// owner whitelists both new validators
	require.NoError(t, owner.WhitelistValidators([]string{
		firstValidatorAddr.String(),
		secondValidatorAddr.String(),
	}))

	// set the initial balance of the new validators
	initialBalance := ethgo.Ether(1000)

	// mint tokens to new validators
	require.NoError(t, owner.MintERC20Token([]string{firstValidatorAddr.String(), secondValidatorAddr.String()},
		[]*big.Int{initialBalance, initialBalance}, polybftConfig.StakeTokenAddr))

	// first validator's balance to be received
	firstBalance, err := relayer.Client().GetBalance(firstValidatorAddr, jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("First validator balance=%d\n", firstBalance)

	// second validator's balance to be received
	secondBalance, err := relayer.Client().GetBalance(secondValidatorAddr, jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("Second validator balance=%d\n", secondBalance)

	// start the first and the second validator
	cluster.InitTestServer(t, cluster.Config.ValidatorPrefix+strconv.Itoa(validatorSetSize+1),
		cluster.Bridge.JSONRPCAddr(), framework.Validator)

	cluster.InitTestServer(t, cluster.Config.ValidatorPrefix+strconv.Itoa(validatorSetSize+2),
		cluster.Bridge.JSONRPCAddr(), framework.Validator)

	// wait for couple of epochs until new validators start
	require.NoError(t, cluster.WaitForBlock(epochSize*3, time.Minute))

	// collect the first and the second validator from the cluster
	firstValidator := cluster.Servers[validatorSetSize]
	secondValidator := cluster.Servers[validatorSetSize+1]

	// register the first validator with stake
	require.NoError(t, firstValidator.RegisterValidatorWithStake(stakeAmount))

	// register the second validator with stake
	require.NoError(t, secondValidator.RegisterValidatorWithStake(stakeAmount))

	firstValidatorInfo, err := validatorHelper.GetValidatorInfo(firstValidatorAddr, relayer)
	require.NoError(t, err)
	require.True(t, firstValidatorInfo.IsActive)
	require.True(t, firstValidatorInfo.Stake.Cmp(stakeAmount) == 0)

	secondValidatorInfo, err := validatorHelper.GetValidatorInfo(secondValidatorAddr, relayer)
	require.NoError(t, err)
	require.True(t, secondValidatorInfo.IsActive)
	require.True(t, secondValidatorInfo.Stake.Cmp(stakeAmount) == 0)

	currentBlock, err := owner.JSONRPC().GetBlockByNumber(jsonrpc.LatestBlockNumber, false)
	require.NoError(t, err)

	// wait for couple of epochs to have some rewards accumulated
	require.NoError(t, cluster.WaitForBlock(currentBlock.Header.Number+(polybftConfig.EpochSize*2), time.Minute))

	bigZero := big.NewInt(0)

	firstValidatorInfo, err = validatorHelper.GetValidatorInfo(firstValidatorAddr, relayer)
	require.NoError(t, err)
	require.True(t, firstValidatorInfo.IsActive)
	require.True(t, firstValidatorInfo.WithdrawableRewards.Cmp(bigZero) > 0)

	secondValidatorInfo, err = validatorHelper.GetValidatorInfo(secondValidatorAddr, relayer)
	require.NoError(t, err)
	require.True(t, secondValidatorInfo.IsActive)
	require.True(t, secondValidatorInfo.WithdrawableRewards.Cmp(bigZero) > 0)

	// wait until one of the validators mine one block to check if they joined consensus
	require.NoError(t, cluster.WaitUntil(3*time.Minute, 2*time.Second, func() bool {
		latestBlock, err := cluster.Servers[0].JSONRPC().GetBlockByNumber(jsonrpc.LatestBlockNumber, false)
		require.NoError(t, err)

		blockMiner := latestBlock.Header.Miner

		return bytes.Equal(firstValidatorAddr.Bytes(), blockMiner) ||
			bytes.Equal(secondValidatorAddr.Bytes(), blockMiner)
	}))
}

func TestE2E_Consensus_Validator_Unstake(t *testing.T) {
	var (
		stakeAmount = ethgo.Ether(100)
	)

	cluster := framework.NewTestCluster(t, 5,
		framework.WithEpochReward(int(ethgo.Ether(1).Uint64())),
		framework.WithEpochSize(5),
		framework.WithSecretsCallback(func(addresses []types.Address, config *framework.TestClusterConfig) {
			for range addresses {
				config.StakeAmounts = append(config.StakeAmounts, new(big.Int).Set(stakeAmount))
			}
		}),
	)

	polybftCfg, err := polybft.LoadPolyBFTConfig(path.Join(cluster.Config.TmpDir, chainConfigFileName))
	require.NoError(t, err)

	srv := cluster.Servers[0]

	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(srv.JSONRPCAddr()))
	require.NoError(t, err)

	validatorAcc, err := validatorHelper.GetAccountFromDir(srv.DataDir())
	require.NoError(t, err)

	cluster.WaitForReady(t)

	validatorAddr := validatorAcc.Ecdsa.Address()

	initialValidatorBalance, err := srv.JSONRPC().GetBalance(validatorAddr, jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("Balance (before unstake)=%d\n", initialValidatorBalance)

	// wait for some rewards to get accumulated
	require.NoError(t, cluster.WaitForBlock(polybftCfg.EpochSize*3, time.Minute))

	validatorInfo, err := validatorHelper.GetValidatorInfo(validatorAcc.Address(), relayer)
	require.NoError(t, err)
	require.True(t, validatorInfo.IsActive)

	initialStake := validatorInfo.Stake
	t.Logf("Stake (before unstake)=%d\n", initialStake)

	reward := validatorInfo.WithdrawableRewards
	t.Logf("Rewards=%d\n", reward)
	require.Greater(t, reward.Uint64(), uint64(0))

	// unstake entire balance (which should remove validator from the validator set in next epoch)
	require.NoError(t, srv.Unstake(initialStake))

	currentBlock, err := srv.JSONRPC().GetBlockByNumber(jsonrpc.LatestBlockNumber, false)
	require.NoError(t, err)

	// wait for couple of epochs to withdraw stake
	require.NoError(t, cluster.WaitForBlock(currentBlock.Header.Number+(polybftCfg.EpochSize*2), time.Minute))
	require.NoError(t, srv.WitdhrawStake())

	// check that validator is no longer active (out of validator set)
	validatorInfo, err = validatorHelper.GetValidatorInfo(validatorAcc.Address(), relayer)
	require.NoError(t, err)
	require.False(t, validatorInfo.IsActive)
	require.True(t, validatorInfo.Stake.Cmp(big.NewInt(0)) == 0)

	t.Logf("Stake (after unstake and withdraw)=%d\n", validatorInfo.Stake)

	balanceBeforeRewardsWithdraw, err := srv.JSONRPC().GetBalance(validatorAddr, jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("Balance (before withdraw rewards)=%d\n", balanceBeforeRewardsWithdraw)

	// withdraw pending rewards
	require.NoError(t, srv.WithdrawRewards())

	newValidatorBalance, err := srv.JSONRPC().GetBalance(validatorAddr, jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("Balance (after withdrawal of rewards)=%s\n", newValidatorBalance)
	require.True(t, newValidatorBalance.Cmp(balanceBeforeRewardsWithdraw) > 0)
}

func TestE2E_Consensus_MintableERC20NativeToken(t *testing.T) {
	const (
		validatorCount = 5
		epochSize      = 10

		tokenName   = "Edge Coin"
		tokenSymbol = "EDGE"
		decimals    = uint8(5)
	)

	validatorsAddrs := make([]types.Address, validatorCount)
	initValidatorsBalance := ethgo.Ether(1)
	initMinterBalance := ethgo.Ether(100000)

	minter, err := crypto.GenerateECDSAKey()
	require.NoError(t, err)

	receiver, err := crypto.GenerateECDSAKey()
	require.NoError(t, err)

	// because we are using native token as reward wallet, and it has default premine balance
	initialTotalSupply := new(big.Int).Set(command.DefaultPremineBalance)

	cluster := framework.NewTestCluster(t,
		validatorCount,
		framework.WithNativeTokenConfig(
			fmt.Sprintf("%s:%s:%d:true", tokenName, tokenSymbol, decimals)),
		framework.WithBladeAdmin(minter.Address().String()),
		framework.WithEpochSize(epochSize),
		framework.WithBaseFeeConfig(""),
		framework.WithSecretsCallback(func(addrs []types.Address, config *framework.TestClusterConfig) {
			config.Premine = append(config.Premine, fmt.Sprintf("%s:%d", minter.Address(), initMinterBalance))
			initialTotalSupply.Add(initialTotalSupply, initMinterBalance)

			for i, addr := range addrs {
				config.Premine = append(config.Premine, fmt.Sprintf("%s:%d", addr, initValidatorsBalance))
				config.StakeAmounts = append(config.StakeAmounts, new(big.Int).Set(initValidatorsBalance))
				validatorsAddrs[i] = addr

				initialTotalSupply.Add(initialTotalSupply, initValidatorsBalance)
			}
		}))
	defer cluster.Stop()

	targetJSONRPC := cluster.Servers[0].JSONRPC()

	cluster.WaitForReady(t)

	// initialize tx relayer
	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(targetJSONRPC))
	require.NoError(t, err)

	// check are native token metadata correctly initialized
	stringABIType := abi.MustNewType("tuple(string)")
	uint8ABIType := abi.MustNewType("tuple(uint8)")

	totalSupply := queryNativeERC20Metadata(t, "totalSupply", uint256ABIType, relayer)
	require.True(t, initialTotalSupply.Cmp(totalSupply.(*big.Int)) == 0)

	// check if initial total supply of native ERC20 token is the same as expected
	name := queryNativeERC20Metadata(t, "name", stringABIType, relayer)
	require.Equal(t, tokenName, name)

	symbol := queryNativeERC20Metadata(t, "symbol", stringABIType, relayer)
	require.Equal(t, tokenSymbol, symbol)

	decimalsCount := queryNativeERC20Metadata(t, "decimals", uint8ABIType, relayer)
	require.Equal(t, decimals, decimalsCount)

	// send mint transactions
	mintAmount := ethgo.Ether(1)

	// make sure minter account can mint tokens
	balanceBefore, err := targetJSONRPC.GetBalance(receiver.Address(), jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)
	t.Logf("Pre-mint balance: %v=%d\n", receiver.Address(), balanceBefore)

	mintFn := &contractsapi.MintRootERC20Fn{
		To:     receiver.Address(),
		Amount: mintAmount,
	}

	mintInput, err := mintFn.EncodeAbi()
	require.NoError(t, err)

	tx := types.NewTx(
		types.NewDynamicFeeTx(
			types.WithTo(&contracts.NativeERC20TokenContract),
			types.WithInput(mintInput),
		),
	)

	receipt, err := relayer.SendTransaction(tx, minter)
	require.NoError(t, err)
	require.Equal(t, uint64(types.ReceiptSuccess), receipt.Status)

	balanceAfter, err := targetJSONRPC.GetBalance(receiver.Address(), jsonrpc.LatestBlockNumberOrHash)
	require.NoError(t, err)

	t.Logf("Post-mint balance: %v=%d\n", receiver.Address(), balanceAfter)
	require.True(t, balanceAfter.Cmp(new(big.Int).Add(mintAmount, balanceBefore)) >= 0)

	// try sending mint transaction from non minter account and make sure it would fail
	nonMinterAcc, err := validatorHelper.GetAccountFromDir(cluster.Servers[1].DataDir())
	require.NoError(t, err)

	mintFn = &contractsapi.MintRootERC20Fn{To: validatorsAddrs[0], Amount: mintAmount}
	mintInput, err = mintFn.EncodeAbi()
	require.NoError(t, err)

	tx = types.NewTx(types.NewDynamicFeeTx(
		types.WithTo(&contracts.NativeERC20TokenContract),
		types.WithInput(mintInput),
	))

	receipt, err = relayer.SendTransaction(tx, nonMinterAcc.Ecdsa)
	require.Error(t, err)
	require.Nil(t, receipt)
}

func TestE2E_Consensus_CustomRewardToken(t *testing.T) {
	const epochSize = 5

	cluster := framework.NewTestCluster(t, 5,
		framework.WithEpochSize(epochSize),
		framework.WithEpochReward(1000000),
		framework.WithTestRewardToken(),
	)
	defer cluster.Stop()

	cluster.WaitForReady(t)

	// wait for couple of epochs to accumulate some rewards
	require.NoError(t, cluster.WaitForBlock(epochSize*3, 3*time.Minute))

	// first validator is the owner of ChildValidator set smart contract
	owner := cluster.Servers[0]
	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(owner.JSONRPCAddr()))
	require.NoError(t, err)

	validatorAcc, err := validatorHelper.GetAccountFromDir(owner.DataDir())
	require.NoError(t, err)

	validatorInfo, err := validatorHelper.GetValidatorInfo(validatorAcc.Ecdsa.Address(), relayer)
	t.Logf("[Validator#%v] Witdhrawable rewards=%d\n", validatorInfo.Address, validatorInfo.WithdrawableRewards)

	require.NoError(t, err)
	require.True(t, validatorInfo.WithdrawableRewards.Cmp(big.NewInt(0)) > 0)
}

// TestE2E_Consensus_EIP1559Check sends a legacy and a dynamic tx to the cluster
// and check if balance of sender, receiver, burn contract and miner is updates correctly
// in accordance with EIP-1559 specifications
func TestE2E_Consensus_EIP1559Check(t *testing.T) {
	t.Skip("TODO - since we removed burn from evm, this should be fixed after the burn solution")

	sender, err := crypto.GenerateECDSAKey()
	require.NoError(t, err)

	recipient := types.StringToAddress("1234")

	// sender must have premined some native tokens
	cluster := framework.NewTestCluster(t, 5,
		framework.WithPremine(types.Address(sender.Address())),
		framework.WithBurnContract(&polybft.BurnContractInfo{BlockNumber: 0, Address: types.ZeroAddress}),
		framework.WithSecretsCallback(func(a []types.Address, config *framework.TestClusterConfig) {
			for range a {
				config.StakeAmounts = append(config.StakeAmounts, command.DefaultPremineBalance)
			}
		}),
	)
	defer cluster.Stop()

	cluster.WaitForReady(t)

	client := cluster.Servers[0].JSONRPC()

	waitUntilBalancesChanged := func(acct types.Address, initialBalance *big.Int) error {
		err := cluster.WaitUntil(30*time.Second, 1*time.Second, func() bool {
			balance, err := client.GetBalance(acct, jsonrpc.LatestBlockNumberOrHash)
			if err != nil {
				return true
			}

			return balance.Cmp(initialBalance) > 0
		})

		return err
	}

	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(cluster.Servers[0].JSONRPC()))
	require.NoError(t, err)

	sendAmount := ethgo.Gwei(1)

	txns := []*types.Transaction{
		types.NewTx(types.NewLegacyTx(
			types.WithValue(sendAmount),
			types.WithTo(&recipient),
			types.WithGas(21000),
			types.WithNonce(uint64(0)),
			types.WithGasPrice(ethgo.Gwei(1)),
		)),
		types.NewTx(types.NewDynamicFeeTx(
			types.WithValue(sendAmount),
			types.WithTo(&recipient),
			types.WithGas(21000),
			types.WithNonce(uint64(0)),
			types.WithGasFeeCap(ethgo.Gwei(1)),
			types.WithGasTipCap(ethgo.Gwei(1)),
		)),
	}

	initialMinerBalance := big.NewInt(0)

	prevMiner := types.ZeroAddress.Bytes()

	for i, txn := range txns {
		senderInitialBalance, _ := client.GetBalance(sender.Address(), jsonrpc.LatestBlockNumberOrHash)
		receiverInitialBalance, _ := client.GetBalance(recipient, jsonrpc.LatestBlockNumberOrHash)
		burnContractInitialBalance, _ := client.GetBalance(types.ZeroAddress, jsonrpc.LatestBlockNumberOrHash)

		receipt, err := relayer.SendTransaction(txn, sender)
		require.NoError(t, err)
		require.Equal(t, uint64(types.ReceiptSuccess), receipt.Status)

		// wait for recipient's balance to increase
		err = waitUntilBalancesChanged(recipient, receiverInitialBalance)
		require.NoError(t, err)

		block, _ := client.GetBlockByHash(types.Hash(receipt.BlockHash), true)
		finalMinerFinalBalance, _ := client.GetBalance(types.BytesToAddress(block.Header.Miner), jsonrpc.LatestBlockNumberOrHash)

		if i == 0 {
			prevMiner = block.Header.Miner
		}

		senderFinalBalance, _ := client.GetBalance(sender.Address(), jsonrpc.LatestBlockNumberOrHash)
		receiverFinalBalance, _ := client.GetBalance(recipient, jsonrpc.LatestBlockNumberOrHash)
		burnContractFinalBalance, _ := client.GetBalance(types.ZeroAddress, jsonrpc.LatestBlockNumberOrHash)

		diffReceiverBalance := new(big.Int).Sub(receiverFinalBalance, receiverInitialBalance)
		require.Equal(t, sendAmount, diffReceiverBalance, "Receiver balance should be increased by send amount")

		if i == 1 && bytes.Equal(prevMiner, block.Header.Miner) {
			initialMinerBalance = big.NewInt(0)
		}

		diffBurnContractBalance := new(big.Int).Sub(burnContractFinalBalance, burnContractInitialBalance)
		diffSenderBalance := new(big.Int).Sub(senderInitialBalance, senderFinalBalance)
		diffMinerBalance := new(big.Int).Sub(finalMinerFinalBalance, initialMinerBalance)

		diffSenderBalance.Sub(diffSenderBalance, diffReceiverBalance)
		diffSenderBalance.Sub(diffSenderBalance, diffBurnContractBalance)
		diffSenderBalance.Sub(diffSenderBalance, diffMinerBalance)

		require.Zero(t, diffSenderBalance.Int64(), "Sender balance should be decreased by send amount + gas")

		initialMinerBalance = finalMinerFinalBalance
	}
}

func TestE2E_Consensus_ChangeVotingPowerByStakingPendingRewards(t *testing.T) {
	const (
		votingPowerChanges       = 2
		epochSize                = 10
		numOfEpochsToCheckChange = 2
	)

	stakeAmount := ethgo.Ether(1)

	cluster := framework.NewTestCluster(t, 5,
		framework.WithEpochSize(epochSize),
		framework.WithEpochReward(1000000),
		framework.WithSecretsCallback(func(addresses []types.Address, config *framework.TestClusterConfig) {
			for range addresses {
				config.StakeAmounts = append(config.StakeAmounts, stakeAmount)
			}
		}),
	)
	defer cluster.Stop()

	validatorSecretFiles, err := genesis.GetValidatorKeyFiles(cluster.Config.TmpDir, cluster.Config.ValidatorPrefix)
	require.NoError(t, err)

	votingPowerChangeValidators := make([]types.Address, votingPowerChanges)

	for i := 0; i < votingPowerChanges; i++ {
		validator, err := validatorHelper.GetAccountFromDir(path.Join(cluster.Config.TmpDir, validatorSecretFiles[i]))
		require.NoError(t, err)

		votingPowerChangeValidators[i] = validator.Ecdsa.Address()
	}

	// tx relayer
	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(cluster.Servers[0].JSONRPCAddr()))
	require.NoError(t, err)

	epochEndingBlock := uint64(2 * epochSize)

	// waiting two epochs, so that some rewards get accumulated
	require.NoError(t, cluster.WaitForBlock(epochEndingBlock, 1*time.Minute))

	queryValidators := func(handler func(idx int, validatorInfo *polybft.ValidatorInfo)) {
		for i, validatorAddr := range votingPowerChangeValidators {
			// query validator info
			validatorInfo, err := validatorHelper.GetValidatorInfo(
				validatorAddr,
				relayer)
			require.NoError(t, err)

			handler(i, validatorInfo)
		}
	}

	bigZero := big.NewInt(0)

	// validatorsMap holds only changed validators
	validatorsMap := make(map[types.Address]*polybft.ValidatorInfo, votingPowerChanges)

	queryValidators(func(idx int, validator *polybft.ValidatorInfo) {
		t.Logf("[Validator#%d] Voting power (original)=%d, rewards=%d\n",
			idx+1, validator.Stake, validator.WithdrawableRewards)

		validatorsMap[validator.Address] = validator
		validatorSrv := cluster.Servers[idx]

		// validator should have some withdrawable rewards by now
		require.True(t, validator.WithdrawableRewards.Cmp(bigZero) > 0)

		// withdraw pending rewards
		require.NoError(t, validatorSrv.WithdrawRewards())

		// stake withdrawable rewards (since rewards are in native erc20 token in this test)
		require.NoError(t, validatorSrv.Stake(types.ZeroAddress, validator.WithdrawableRewards))
	})

	queryValidators(func(idx int, validator *polybft.ValidatorInfo) {
		t.Logf("[Validator#%d] Voting power (after stake)=%d\n", idx+1, validator.Stake)

		previousValidatorInfo := validatorsMap[validator.Address]
		stakedAmount := new(big.Int).Add(previousValidatorInfo.WithdrawableRewards, previousValidatorInfo.Stake)

		// assert that total stake has increased by staked amount
		require.Equal(t, stakedAmount, validator.Stake)

		validatorsMap[validator.Address] = validator
	})

	// start checking for delta from this epoch ending block
	epochEndingBlock += epochSize

	didVotingPowerChangeInConsensus := false

	// we will check for couple of epoch ending blocks to see Voting Power change
	for i := 0; i < numOfEpochsToCheckChange; i++ {
		require.NoError(t, cluster.WaitForBlock(epochEndingBlock, time.Minute))

		latestBlock, err := cluster.Servers[0].JSONRPC().GetBlockByNumber(jsonrpc.BlockNumber(epochEndingBlock), false)
		require.NoError(t, err)

		epochEndingBlock += epochSize

		currentExtra, err := polybft.GetIbftExtra(latestBlock.Header.ExtraData)
		require.NoError(t, err)

		if currentExtra.Validators == nil || currentExtra.Validators.IsEmpty() {
			continue
		}

		for addr, validator := range validatorsMap {
			if !currentExtra.Validators.Updated.ContainsAddress(addr) {
				continue
			}

			if currentExtra.Validators.Updated.GetValidatorMetadata(addr).VotingPower.Cmp(validator.Stake) != 0 {
				continue
			}
		}

		didVotingPowerChangeInConsensus = true

		break
	}

	if !didVotingPowerChangeInConsensus {
		t.Errorf("voting power did not change in consensus for %d epochs", numOfEpochsToCheckChange)
	}
}

func TestE2E_Deploy_Nested_Contract(t *testing.T) {
	numberToPersist := big.NewInt(234586)

	admin, err := crypto.GenerateECDSAKey()
	require.NoError(t, err)

	cluster := framework.NewTestCluster(t, 5, framework.WithBladeAdmin(admin.Address().String()))
	defer cluster.Stop()

	cluster.WaitForReady(t)

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(cluster.Servers[0].JSONRPC()))
	require.NoError(t, err)

	// deploy Wrapper contract
	receipt, err := txRelayer.SendTransaction(
		types.NewTx(&types.LegacyTx{
			BaseTx: &types.BaseTx{
				Input: contractsapi.Wrapper.Bytecode,
			},
		}),
		admin)
	require.NoError(t, err)

	// address of Wrapper contract
	wrapperAddr := types.Address(receipt.ContractAddress)

	// getAddressFn returns address of the NumberPersister (nested) contract
	getAddressFn := contractsapi.Wrapper.Abi.GetMethod("getAddress")

	getAddrInput, err := getAddressFn.Encode([]interface{}{})
	require.NoError(t, err)

	response, err := txRelayer.Call(types.ZeroAddress, wrapperAddr, getAddrInput)
	require.NoError(t, err)

	// address of the NumberPersister (nested) contract
	numberPersisterAddr := ethgo.Address(types.StringToAddress(response))
	require.NotEqual(t, ethgo.ZeroAddress, numberPersisterAddr)

	// set value to NumberPersister contract
	setNumberFn := contractsapi.Wrapper.Abi.GetMethod("setNumber")

	setValueInput, err := setNumberFn.Encode([]interface{}{numberToPersist})
	require.NoError(t, err)

	txn := types.NewTx(types.NewLegacyTx(
		types.WithFrom(admin.Address()),
		types.WithTo(&wrapperAddr),
		types.WithInput(setValueInput),
	))

	receipt, err = txRelayer.SendTransaction(txn, admin)
	require.NoError(t, err)
	require.Equal(t, uint64(types.ReceiptSuccess), receipt.Status)

	// getting new value from wrapper contract
	getNumberFn := contractsapi.Wrapper.Abi.GetMethod("getNumber")

	getNumberInput, err := getNumberFn.Encode([]interface{}{})
	require.NoError(t, err)

	response, err = txRelayer.Call(types.ZeroAddress, wrapperAddr, getNumberInput)
	require.NoError(t, err)

	parsedResponse, err := common.ParseUint256orHex(&response)
	require.NoError(t, err)

	require.Equal(t, numberToPersist, parsedResponse)
}

func TestE2E_TestCardanoVerifySignaturePrecompile(t *testing.T) {
	admin, err := wallet.GenerateAccount()
	require.NoError(t, err)

	cluster := framework.NewTestCluster(t, 4,
		framework.WithBladeAdmin(admin.Address().String()),
	)

	defer cluster.Stop()

	cluster.WaitForReady(t)

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(cluster.Servers[0].JSONRPC()))
	require.NoError(t, err)

	// deploy contract
	receipt, err := txRelayer.SendTransaction(
		types.NewTx(types.NewLegacyTx(
			types.WithFrom(admin.Ecdsa.Address()),
			types.WithInput(contractsapi.TestCardanoVerifySignature.Bytecode),
		)),
		admin.Ecdsa)
	require.NoError(t, err)

	dummyKeyHash := "0837676734ffaab34"
	contractAddr := types.Address(receipt.ContractAddress)
	txRaw := "0x84a40083825820098236134e0f2077a6434dd9d7727126fa8b3627bcab3ae030a194d46eded73e00825820d1fd0d772be7741d9bfaf0b037d02d2867a987ccba3e6ba2ee9aa2a861b7314502825820e99a5bde15aa05f24fcc04b7eabc1520d3397283b1ee720de9fe2653abbb0c9f00018382581d60244877c1aeefc7fd5405a6e14d927d91758d45e37c20fa2ac89cb1671a000f424082581d700c25e4ff24cfa0dfebcec382095161271dc9bb744ca4149ec604dc991482581d70a5caf9ce4bed09c794ee87bddb6505822db5bd476a4f61e0cd4074a21a000b495f021a0003f8e103196dc0a10182830304858200581cd6b67f93ffa4e2651271cc9bcdbdedb2539911266b534d9c163cba218200581ccba89c7084bf0ce4bf404346b668a7e83c8c9c250d1cafd8d8996e418200581c79df3577e4c7d7da04872c2182b8d8829d7b477912dbf35d89287c398200581c2368e8113bd5f32d713751791d29acee9e1b5a425b0454b963b2558b8200581c06b4c7f5254d6395b527ac3de60c1d77194df7431d85fe55ca8f107d830304858200581cf0f4837b3a306752a2b3e52394168bc7391de3dce11364b723cc55cf8200581c47344d5bd7b2fea56336ba789579705a944760032585ef64084c92db8200581cf01018c1d8da54c2f557679243b09af1c4dd4d9c671512b01fa5f92b8200581c6837232854849427dae7c45892032d7ded136c5beb13c68fda635d878200581cd215701e2eb17c741b9d306cba553f9fbaaca1e12a5925a065b90fa8f5f6"
	txHash, _ := hex.DecodeString("1bfa0aec9f7284fb3a2d2536fda62f780e8405950c6dbef27547e34f0bf95d9d")
	signingKey := []byte{
		179, 184, 22, 151, 216, 145, 80, 77, 17, 87, 101, 112, 4, 129, 121, 34, 163, 63, 48, 147, 158, 0, 210, 219, 136, 198, 72, 36, 76, 190, 77, 151,
	}
	verificationKey := []byte{
		220, 19, 136, 5, 47, 117, 121, 12, 156, 173, 178, 183, 153, 81, 141, 128, 174, 237, 153, 102, 235, 18, 191, 119, 130, 13, 59, 176, 30, 181, 253, 122,
	}
	invalidKey := [32]byte{}

	checkValidity := func(t *testing.T,
		txRawOrMsg string, signature []byte, verificationKey []byte, isTx bool, shouldBeValid bool) {
		t.Helper()

		var fn *abi.Method
		if isTx {
			fn = contractsapi.TestCardanoVerifySignature.Abi.GetMethod("check")
		} else {
			fn = contractsapi.TestCardanoVerifySignature.Abi.GetMethod("checkMsg")
		}

		hasQuorumFnBytes, err := fn.Encode([]interface{}{
			txRawOrMsg, hex.EncodeToString(signature), hex.EncodeToString(verificationKey),
		})
		require.NoError(t, err)

		response, err := txRelayer.Call(types.ZeroAddress, contractAddr, hasQuorumFnBytes)
		require.NoError(t, err)

		if shouldBeValid {
			require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", response)
		} else {
			require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", response)
		}
	}

	signature, err := cardano_wallet.SignMessage(signingKey, verificationKey, txHash)
	require.NoError(t, err)

	checkValidity(t, txRaw, signature, verificationKey, true, true)
	checkValidity(t, txRaw, signature, invalidKey[:], true, false)
	checkValidity(t, txRaw, append([]byte{0}, signature...), verificationKey, true, false)

	// message with key hash example
	signature, err = cardano_wallet.SignMessage(signingKey, verificationKey, []byte("hello world: "+dummyKeyHash))
	require.NoError(t, err)

	checkValidity(t, dummyKeyHash, signature, verificationKey, false, true)
	checkValidity(t, dummyKeyHash, signature, invalidKey[:], false, false)
	checkValidity(t, dummyKeyHash, append([]byte{0}, signature...), verificationKey, false, false)
}
