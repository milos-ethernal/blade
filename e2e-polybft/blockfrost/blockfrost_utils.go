package blockfrost

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ethernal-Tech/cardano-infrastructure/wallet"
)

func WaitUntil(
	t *testing.T,
	ctx context.Context, provider wallet.ITxProvider,
	timeoutDuration time.Duration,
	handler func(wallet.QueryTipData) bool,
) error {
	t.Helper()

	timeout := time.NewTimer(timeoutDuration)
	defer timeout.Stop()

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("timeout")
		case <-ticker.C:
		}

		tip, err := provider.GetTip(ctx)
		if err != nil {
			t.Log("error while retrieving tip", "err", err)
		} else if handler(tip) {
			return nil
		}
	}
}

func WaitUntilBlock(
	t *testing.T,
	ctx context.Context, provider wallet.ITxProvider,
	blockNum uint64, timeoutDuration time.Duration,
) error {
	t.Helper()

	return WaitUntil(t, ctx, provider, timeoutDuration, func(qtd wallet.QueryTipData) bool {
		return qtd.Block >= blockNum
	})
}

func ResetDBSync(
	t *testing.T,
	timeoutDuration time.Duration, startAfter time.Duration, dbSyncContainer string,
) error {
	time.Sleep(startAfter)

	timeout := time.NewTimer(timeoutDuration)
	defer timeout.Stop()

	ticker := time.NewTicker(time.Second * 20)
	defer ticker.Stop()

	const numberOfBlocksToConsiderStarted = 6

	cnt := 0

	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("timeout")
		case <-ticker.C:
		}

		t.Log("Check Db Sync logs")

		res, err := runCommand("docker", []string{"logs", dbSyncContainer})
		if err != nil {
			return err
		}

		logs := strings.Split(res, "\n")
		if len(logs) < 2 {
			continue
		}

		lastLog := logs[len(logs)-2] // last is empty string so we take one before last

		if strings.Contains(lastLog, "Creating Indexes. This may take a while.") {
			t.Log("Restarting db sync docker container")
			_, _ = runCommand("docker", []string{"restart", dbSyncContainer})
		} else if strings.Contains(lastLog, "Insert Babbage Block") {
			cnt++

			if cnt == numberOfBlocksToConsiderStarted {
				break
			}
		}
	}

	return nil
}
