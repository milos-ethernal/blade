package validators

import (
	"fmt"
	"time"

	"github.com/0xPolygon/polygon-edge/command"
	bridgeHelper "github.com/0xPolygon/polygon-edge/command/bridge/helper"
	"github.com/0xPolygon/polygon-edge/command/helper"
	polybftsecrets "github.com/0xPolygon/polygon-edge/command/secrets/init"
	validatorHelper "github.com/0xPolygon/polygon-edge/command/validator/helper"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
)

var (
	params validatorInfoParams
)

func GetCommand() *cobra.Command {
	validatorInfoCmd := &cobra.Command{
		Use:     "info",
		Short:   "Gets validator info",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	helper.RegisterJSONRPCFlag(validatorInfoCmd)
	setFlags(validatorInfoCmd)

	return validatorInfoCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.accountDir,
		polybftsecrets.AccountDirFlag,
		"",
		polybftsecrets.AccountDirFlagDesc,
	)

	cmd.Flags().StringVar(
		&params.accountConfig,
		polybftsecrets.AccountConfigFlag,
		"",
		polybftsecrets.AccountConfigFlagDesc,
	)

	cmd.Flags().Int64Var(
		&params.chainID,
		polybftsecrets.ChainIDFlag,
		0,
		polybftsecrets.ChainIDFlagDesc,
	)

	cmd.Flags().Uint64Var(
		&params.txTimeout,
		bridgeHelper.TxTimeoutFlag,
		5000,
		"timeout for receipts in milliseconds",
	)

	cmd.Flags().Uint64Var(
		&params.txPollFreq,
		bridgeHelper.TxPollFreqFlag,
		50,
		"frequency in milliseconds for poll transactions",
	)

	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	params.jsonRPC = helper.GetJSONRPCAddress(cmd)

	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) error {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	validatorAccount, err := validatorHelper.GetAccount(params.accountDir, params.accountConfig)
	if err != nil {
		return err
	}

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptsTimeout(time.Duration(params.txTimeout*uint64(time.Millisecond))),
		txrelayer.WithReceiptsPollFreq(time.Duration(params.txPollFreq*uint64(time.Millisecond))))
	if err != nil {
		return err
	}

	validatorAddr := validatorAccount.Ecdsa.Address()
	supernetManagerAddr := types.StringToAddress(params.supernetManagerAddress)
	stakeManagerAddr := types.StringToAddress(params.stakeManagerAddress)

	validatorInfo, err := bridgeHelper.GetValidatorInfo(validatorAddr,
		supernetManagerAddr, stakeManagerAddr, txRelayer)
	if err != nil {
		return fmt.Errorf("failed to get validator info for %s: %w", validatorAddr, err)
	}

	outputter.WriteCommandResult(&validatorsInfoResult{
		Address:     validatorInfo.Address.String(),
		Stake:       validatorInfo.Stake.Uint64(),
		Active:      validatorInfo.IsActive,
		Whitelisted: validatorInfo.IsWhitelisted,
	})

	return nil
}
