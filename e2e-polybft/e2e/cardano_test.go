package e2e

import (
	"context"
	"fmt"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/e2e-polybft/blockfrost"
	"github.com/0xPolygon/polygon-edge/e2e-polybft/cardanofw"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/Ethernal-Tech/cardano-infrastructure/wallet"
	"github.com/stretchr/testify/assert"
)

// Download Cardano executables from https://github.com/IntersectMBO/cardano-node/releases/tag/8.7.3 and unpack tar.gz file
// Add directory where unpacked files are located to the $PATH (in example bellow `~/Apps/cardano`)
// eq add line `export PATH=$PATH:~/Apps/cardano` to  `~/.bashrc`
func TestE2E_CardanoTwoClustersBasic(t *testing.T) {
	const (
		clusterCnt = 1
	)

	var (
		errors      [clusterCnt]error
		wg          sync.WaitGroup
		baseLogsDir = path.Join("../..", fmt.Sprintf("e2e-logs-cardano-%d", time.Now().UTC().Unix()), t.Name())
	)

	for i := 0; i < clusterCnt; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			logsDir := fmt.Sprintf("%s/%d", baseLogsDir, id)
			if err := common.CreateDirSafe(logsDir, 0750); err != nil {
				errors[id] = err

				return
			}

			cluster, err := cardanofw.NewCardanoTestCluster(t,
				cardanofw.WithID(id+1),
				cardanofw.WithNodesCount(4),
				cardanofw.WithStartTimeDelay(time.Second*5),
				cardanofw.WithPort(5000+id*100),
				cardanofw.WithLogsDir(logsDir),
				cardanofw.WithNetworkMagic(42+id))
			if err != nil {
				errors[id] = err

				return
			}

			_ = cluster.StartDocker()

			defer cluster.StopDocker() //nolint:errcheck

			t.Log("Waiting for sockets to be ready")

			// if errors[id] = cluster.WaitForReady(context.Background(), time.Second*300); errors[id] != nil {
			// 	return
			// }

			bf, err := blockfrost.NewBlockFrost(cluster, id+1)
			if err != nil {
				errors[id] = err
				return
			}

			if errors[id] = bf.Start(); errors[id] != nil {
				return
			}

			defer bf.Stop()

			errors[id] = blockfrost.NewResetDBSync(30, "cluster-1-blockfrost_db-sync_1")

			errors[id] = CheckBlock("http://localhost:12001", context.Background())
		}(i)
	}

	wg.Wait()

	for i := 0; i < clusterCnt; i++ {
		assert.NoError(t, errors[i])
	}
}

func CheckBlock(url string, ctx context.Context) error {
	blockfrostProvider, err := wallet.NewTxProviderBlockFrost(url, "")
	if err != nil {
		return err
	}

	tipData, err := blockfrostProvider.GetTip(ctx)
	if err != nil {
		return err
	}

	fmt.Println(tipData)

	return nil
}
