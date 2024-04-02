package leveldb

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain/storagev2"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func newStorage(t *testing.T) (*storagev2.Storage, func()) {
	t.Helper()

	path, err := os.MkdirTemp("/tmp", "minimal_storage")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewLevelDBStorage(path, hclog.NewNullLogger())
	if err != nil {
		t.Fatal(err)
	}

	closeFn := func() {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}

		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}

	return s, closeFn
}

func newStorageP(t *testing.T) (*storagev2.Storage, func(), string) {
	t.Helper()

	p, err := os.MkdirTemp("", "leveldbV2-test")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(p, 0755))

	s, err := NewLevelDBStorage(p, hclog.NewNullLogger())
	require.NoError(t, err)

	closeFn := func() {
		require.NoError(t, s.Close())

		if err := s.Close(); err != nil {
			t.Fatal(err)
		}

		require.NoError(t, os.RemoveAll(p))
	}

	return s, closeFn, p
}

func countLdbFilesInPath(path string) int {
	pattern := filepath.Join(path, "*.ldb")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return -1
	}

	return len(files)
}

func dirSize(t *testing.T, path string) int64 {
	t.Helper()

	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fail()
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})
	if err != nil {
		t.Log(err)
	}

	return size
}

func writeBlock(t *testing.T, s *storagev2.Storage, b *types.FullBlock) {
	t.Helper()

	batchWriter := s.NewWriter()

	batchWriter.PutBody(b.Block.Number(), b.Block.Hash(), b.Block.Body())

	for _, tx := range b.Block.Transactions {
		batchWriter.PutTxLookup(tx.Hash(), b.Block.Number())
	}

	batchWriter.PutHeadHash(b.Block.Header.Hash)
	batchWriter.PutHeadNumber(b.Block.Number())
	batchWriter.PutBlockLookup(b.Block.Hash(), b.Block.Number())
	batchWriter.PutHeader(b.Block.Header)
	batchWriter.PutReceipts(b.Block.Number(), b.Block.Hash(), b.Receipts)
	batchWriter.PutCanonicalHash(b.Block.Number(), b.Block.Hash())
	require.NoError(t, batchWriter.WriteBatch())
}

func readBlock(t *testing.T, s *storagev2.Storage, blockCount int, wg *sync.WaitGroup, ctx context.Context) {
	t.Helper()

	defer wg.Done()

	ticker := time.NewTicker(20 * time.Millisecond)

	readCount := 1000
	for i := 1; i <= readCount; i++ {
		n := uint64(1 + rand.Intn(blockCount))

		hn, ok := s.ReadHeadNumber()
		if ok && n <= hn {
			// If head number is read and chain progresed enough to contain canonical block #n
			h, ok := s.ReadCanonicalHash(n)
			require.True(t, ok)

			_, err := s.ReadBody(n, h)
			require.NoError(t, err)

			_, err = s.ReadHeader(n, h)
			require.NoError(t, err)

			_, err = s.ReadReceipts(n, h)
			require.NoError(t, err)

			b, err := s.ReadBlockLookup(h)
			require.NoError(t, err)

			require.Equal(t, n, b)
		}

		select {
		case <-ctx.Done():
			ticker.Stop()

			return
		case <-ticker.C:
		}
	}

	t.Logf("\tRead thread finished")
}

func TestStorage(t *testing.T) {
	storagev2.TestStorage(t, newStorage)
}

func TestWriteReadFullBlockInParallel(t *testing.T) {
	s, _, path := newStorageP(t)
	defer s.Close()

	var wg sync.WaitGroup

	blockCount := 100
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)

	signchan := make(chan os.Signal, 1)
	signal.Notify(signchan, syscall.SIGINT)

	go func() {
		<-signchan
		cancel()
	}()

	blockchain := make(chan *types.FullBlock, 1)
	go storagev2.GenerateBlocks(t, blockCount, blockchain, ctx)

	readThreads := 3
	for i := 1; i <= readThreads; i++ {
		wg.Add(1)

		go readBlock(t, s, blockCount, &wg, ctx)
	}

insertloop:
	for i := 1; i <= blockCount; i++ {
		select {
		case <-ctx.Done():
			break insertloop
		case b := <-blockchain:
			writeBlock(t, s, b)
			t.Logf("writing block %d", i)
		}
	}

	size := dirSize(t, path)
	t.Logf("\tldb file count: %d", countLdbFilesInPath(path))
	t.Logf("\tdir size %d MBs", size/(1*opt.MiB))
	wg.Wait()
}
