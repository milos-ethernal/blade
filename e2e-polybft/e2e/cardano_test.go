package e2e

import (
	"context"
	"fmt"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/e2e-polybft/cardanofw"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/stretchr/testify/assert"
)

// Download Cardano executables from https://github.com/IntersectMBO/cardano-node/releases/tag/8.7.3 and unpack tar.gz file
// Add directory where unpacked files are located to the $PATH (in example bellow `~/Apps/cardano`)
// eq add line `export PATH=$PATH:~/Apps/cardano` to  `~/.bashrc`
func TestE2E_CardanoTwoClustersBasic(t *testing.T) {
	const (
		clusterCnt = 2
	)

	var (
		errors      [clusterCnt]error
		wg          sync.WaitGroup
		baseLogsDir = path.Join("../..", fmt.Sprintf("e2e-logs-cardano-%d", time.Now().UTC().Unix()), t.Name())
	)

	for i := 0; i < clusterCnt; i++ {
		id := i

		wg.Add(1)

		go func() {
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

			defer cluster.StopDocker() //nolint:errcheck

			t.Log("Waiting for blocks", "id", id+1)

			errors[id] = cluster.WaitForBlockWithState(context.Background(), 10, time.Second*300)
		}()
	}

	wg.Wait()

	for i := 0; i < clusterCnt; i++ {
		assert.NoError(t, errors[i])
	}
}
