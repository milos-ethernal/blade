package jsonrpc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/valyala/fastjson"
)

var (
	defaultArena fastjson.ArenaPool
	defaultPool  fastjson.ParserPool
)

const jsonRPCMetric = "json_rpc"

// For union type of transaction and types.Hash
type transactionOrHash interface {
	getHash() types.Hash
}

type transaction struct {
	Nonce       argUint64          `json:"nonce"`
	GasPrice    *argBig            `json:"gasPrice,omitempty"`
	GasTipCap   *argBig            `json:"maxPriorityFeePerGas,omitempty"`
	GasFeeCap   *argBig            `json:"maxFeePerGas,omitempty"`
	Gas         argUint64          `json:"gas"`
	To          *types.Address     `json:"to"`
	Value       argBig             `json:"value"`
	Input       argBytes           `json:"input"`
	V           argBig             `json:"v"`
	R           argBig             `json:"r"`
	S           argBig             `json:"s"`
	Hash        types.Hash         `json:"hash"`
	From        types.Address      `json:"from"`
	BlockHash   *types.Hash        `json:"blockHash"`
	BlockNumber *argUint64         `json:"blockNumber"`
	TxIndex     *argUint64         `json:"transactionIndex"`
	ChainID     *argBig            `json:"chainId,omitempty"`
	Type        argUint64          `json:"type"`
	AccessList  types.TxAccessList `json:"accessList,omitempty"`
}

func (t transaction) getHash() types.Hash { return t.Hash }

// Redefine to implement getHash() of transactionOrHash
type transactionHash types.Hash

func (h transactionHash) getHash() types.Hash { return types.Hash(h) }

func (h transactionHash) MarshalText() ([]byte, error) {
	return []byte(types.Hash(h).String()), nil
}

func toPendingTransaction(t *types.Transaction) *transaction {
	return toTransaction(t, nil, nil, nil)
}

func toTransaction(
	t *types.Transaction,
	blockNumber *argUint64,
	blockHash *types.Hash,
	txIndex *int,
) *transaction {
	v, r, s := t.RawSignatureValues()
	res := &transaction{
		Nonce:       argUint64(t.Nonce()),
		Gas:         argUint64(t.Gas()),
		To:          t.To(),
		Value:       argBig(*t.Value()),
		Input:       t.Input(),
		V:           argBig(*v),
		R:           argBig(*r),
		S:           argBig(*s),
		Hash:        t.Hash(),
		From:        t.From(),
		Type:        argUint64(t.Type()),
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
	}

	if t.GasPrice() != nil && t.Type() != types.DynamicFeeTxType {
		gasPrice := argBig(*(t.GasPrice()))
		res.GasPrice = &gasPrice
	}

	if t.GasTipCap() != nil && t.Type() == types.DynamicFeeTxType {
		gasTipCap := argBig(*(t.GasTipCap()))
		res.GasTipCap = &gasTipCap
	}

	if t.GasFeeCap() != nil && t.Type() == types.DynamicFeeTxType {
		gasFeeCap := argBig(*(t.GasFeeCap()))
		res.GasFeeCap = &gasFeeCap
	}

	if t.ChainID() != nil {
		chainID := argBig(*(t.ChainID()))
		res.ChainID = &chainID
	}

	if txIndex != nil {
		res.TxIndex = argUintPtr(uint64(*txIndex))
	}

	if t.AccessList() != nil {
		res.AccessList = t.AccessList()
	}

	return res
}

type header struct {
	ParentHash      types.Hash  `json:"parentHash"`
	Sha3Uncles      types.Hash  `json:"sha3Uncles"`
	Miner           argBytes    `json:"miner"`
	StateRoot       types.Hash  `json:"stateRoot"`
	TxRoot          types.Hash  `json:"transactionsRoot"`
	ReceiptsRoot    types.Hash  `json:"receiptsRoot"`
	LogsBloom       types.Bloom `json:"logsBloom"`
	Difficulty      argUint64   `json:"difficulty"`
	TotalDifficulty argUint64   `json:"totalDifficulty"`
	Number          argUint64   `json:"number"`
	GasLimit        argUint64   `json:"gasLimit"`
	GasUsed         argUint64   `json:"gasUsed"`
	Timestamp       argUint64   `json:"timestamp"`
	ExtraData       argBytes    `json:"extraData"`
	MixHash         types.Hash  `json:"mixHash"`
	Nonce           types.Nonce `json:"nonce"`
	Hash            types.Hash  `json:"hash"`
	BaseFee         argUint64   `json:"baseFeePerGas,omitempty"`
}

type accessListResult struct {
	Accesslist types.TxAccessList `json:"accessList"`
	Error      error              `json:"error,omitempty"`
	GasUsed    argUint64          `json:"gasUsed"`
}

type block struct {
	header
	Size         argUint64           `json:"size"`
	Transactions []transactionOrHash `json:"transactions"`
	Uncles       []types.Hash        `json:"uncles"`
}

func (b *block) Copy() *block {
	bb := new(block)
	*bb = *b

	bb.Miner = make([]byte, len(b.Miner))
	copy(bb.Miner[:], b.Miner[:])

	bb.ExtraData = make([]byte, len(b.ExtraData))
	copy(bb.ExtraData[:], b.ExtraData[:])

	return bb
}

func toBlock(b *types.Block, fullTx bool) *block {
	h := b.Header
	resHeader := header{
		ParentHash:      h.ParentHash,
		Sha3Uncles:      h.Sha3Uncles,
		Miner:           argBytes(h.Miner),
		StateRoot:       h.StateRoot,
		TxRoot:          h.TxRoot,
		ReceiptsRoot:    h.ReceiptsRoot,
		LogsBloom:       h.LogsBloom,
		Difficulty:      argUint64(h.Difficulty),
		TotalDifficulty: argUint64(h.Difficulty), // not needed for POS
		Number:          argUint64(h.Number),
		GasLimit:        argUint64(h.GasLimit),
		GasUsed:         argUint64(h.GasUsed),
		Timestamp:       argUint64(h.Timestamp),
		ExtraData:       argBytes(h.ExtraData),
		MixHash:         h.MixHash,
		Nonce:           h.Nonce,
		Hash:            h.Hash,
		BaseFee:         argUint64(h.BaseFee),
	}

	res := &block{
		header:       resHeader,
		Size:         argUint64(b.Size()),
		Transactions: []transactionOrHash{},
		Uncles:       []types.Hash{},
	}

	for idx, txn := range b.Transactions {
		if fullTx {
			txn.SetGasPrice(txn.GetGasPrice(b.Header.BaseFee))
			res.Transactions = append(
				res.Transactions,
				toTransaction(
					txn,
					argUintPtr(b.Number()),
					argHashPtr(b.Hash()),
					&idx,
				),
			)
		} else {
			res.Transactions = append(
				res.Transactions,
				transactionHash(txn.Hash()),
			)
		}
	}

	for _, uncle := range b.Uncles {
		res.Uncles = append(res.Uncles, uncle.Hash)
	}

	return res
}

func toHeader(h *types.Header) *header {
	res := &header{
		ParentHash:      h.ParentHash,
		Sha3Uncles:      h.Sha3Uncles,
		Miner:           argBytes(h.Miner),
		StateRoot:       h.StateRoot,
		TxRoot:          h.TxRoot,
		ReceiptsRoot:    h.ReceiptsRoot,
		LogsBloom:       h.LogsBloom,
		Difficulty:      argUint64(h.Difficulty),
		TotalDifficulty: argUint64(h.Difficulty), // not needed for POS
		Number:          argUint64(h.Number),
		GasLimit:        argUint64(h.GasLimit),
		GasUsed:         argUint64(h.GasUsed),
		Timestamp:       argUint64(h.Timestamp),
		ExtraData:       argBytes(h.ExtraData),
		MixHash:         h.MixHash,
		Nonce:           h.Nonce,
		Hash:            h.Hash,
		BaseFee:         argUint64(h.BaseFee),
	}

	return res
}

type receipt struct {
	Root              types.Hash     `json:"root"`
	CumulativeGasUsed argUint64      `json:"cumulativeGasUsed"`
	LogsBloom         types.Bloom    `json:"logsBloom"`
	Logs              []*Log         `json:"logs"`
	Status            argUint64      `json:"status"`
	TxHash            types.Hash     `json:"transactionHash"`
	TxIndex           argUint64      `json:"transactionIndex"`
	BlockHash         types.Hash     `json:"blockHash"`
	BlockNumber       argUint64      `json:"blockNumber"`
	GasUsed           argUint64      `json:"gasUsed"`
	ContractAddress   *types.Address `json:"contractAddress"`
	FromAddr          types.Address  `json:"from"`
	ToAddr            *types.Address `json:"to"`
}

func toReceipt(src *types.Receipt, tx *types.Transaction,
	txIndex uint64, header *types.Header, logs []*Log) *receipt {
	return &receipt{
		Root:              src.Root,
		CumulativeGasUsed: argUint64(src.CumulativeGasUsed),
		LogsBloom:         src.LogsBloom,
		Status:            argUint64(*src.Status),
		TxHash:            tx.Hash(),
		TxIndex:           argUint64(txIndex),
		BlockHash:         header.Hash,
		BlockNumber:       argUint64(header.Number),
		GasUsed:           argUint64(src.GasUsed),
		ContractAddress:   src.ContractAddress,
		FromAddr:          tx.From(),
		ToAddr:            tx.To(),
		Logs:              logs,
	}
}

type Log struct {
	Address     types.Address `json:"address"`
	Topics      []types.Hash  `json:"topics"`
	Data        argBytes      `json:"data"`
	BlockNumber argUint64     `json:"blockNumber"`
	TxHash      types.Hash    `json:"transactionHash"`
	TxIndex     argUint64     `json:"transactionIndex"`
	BlockHash   types.Hash    `json:"blockHash"`
	LogIndex    argUint64     `json:"logIndex"`
	Removed     bool          `json:"removed"`
}

func toLogs(srcLogs []*types.Log, baseIdx, txIdx uint64, header *types.Header, txHash types.Hash) []*Log {
	logs := make([]*Log, len(srcLogs))
	for i, srcLog := range srcLogs {
		logs[i] = toLog(srcLog, baseIdx+uint64(i), txIdx, header, txHash)
	}

	return logs
}

func toLog(src *types.Log, logIdx, txIdx uint64, header *types.Header, txHash types.Hash) *Log {
	return &Log{
		Address:     src.Address,
		Topics:      src.Topics,
		Data:        argBytes(src.Data),
		BlockNumber: argUint64(header.Number),
		BlockHash:   header.Hash,
		TxHash:      txHash,
		TxIndex:     argUint64(txIdx),
		LogIndex:    argUint64(logIdx),
	}
}

type argBig big.Int

func argBigPtr(b *big.Int) *argBig {
	v := argBig(*b)

	return &v
}

func (a *argBig) UnmarshalText(input []byte) error {
	buf, err := decodeToHex(input)
	if err != nil {
		return err
	}

	b := new(big.Int)
	b.SetBytes(buf)
	*a = argBig(*b)

	return nil
}

func (a argBig) MarshalText() ([]byte, error) {
	b := (*big.Int)(&a)

	return []byte("0x" + b.Text(16)), nil
}

func argAddrPtr(a types.Address) *types.Address {
	return &a
}

func argHashPtr(h types.Hash) *types.Hash {
	return &h
}

type argUint64 uint64

func argUintPtr(n uint64) *argUint64 {
	v := argUint64(n)

	return &v
}

func (u argUint64) MarshalText() ([]byte, error) {
	buf := make([]byte, 2, 10)
	copy(buf, `0x`)
	buf = strconv.AppendUint(buf, uint64(u), 16)

	return buf, nil
}

func (u *argUint64) UnmarshalText(input []byte) error {
	str := strings.Trim(string(input), "\"")

	num, err := common.ParseUint64orHex(&str)
	if err != nil {
		return err
	}

	*u = argUint64(num)

	return nil
}

func (u *argUint64) UnmarshalJSON(buffer []byte) error {
	return u.UnmarshalText(buffer)
}

type argBytes []byte

func argBytesPtr(b []byte) *argBytes {
	bb := argBytes(b)

	return &bb
}

func (b argBytes) MarshalText() ([]byte, error) {
	return encodeToHex(b), nil
}

func (b *argBytes) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}

	aux := make([]byte, len(hh))
	copy(aux[:], hh[:])
	*b = aux

	return nil
}

func decodeToHex(b []byte) ([]byte, error) {
	str := string(b)
	str = strings.TrimPrefix(str, "0x")

	if len(str)%2 != 0 {
		str = "0" + str
	}

	return hex.DecodeString(str)
}

func encodeToHex(b []byte) []byte {
	str := hex.EncodeToString(b)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return []byte("0x" + str)
}

// txnArgs is the transaction argument for the rpc endpoints
type txnArgs struct {
	From       *types.Address      `json:"from"`
	To         *types.Address      `json:"to"`
	Gas        *argUint64          `json:"gas"`
	GasPrice   *argBytes           `json:"gasPrice,omitempty"`
	GasTipCap  *argBytes           `json:"maxFeePerGas,omitempty"`
	GasFeeCap  *argBytes           `json:"maxPriorityFeePerGas,omitempty"`
	Value      *argBytes           `json:"value"`
	Data       *argBytes           `json:"data"`
	Input      *argBytes           `json:"input"`
	Nonce      *argUint64          `json:"nonce"`
	Type       *argUint64          `json:"type"`
	AccessList *types.TxAccessList `json:"accessList,omitempty"`
	ChainID    *argUint64          `json:"chainId,omitempty"`
}

type progression struct {
	Type          string    `json:"type"`
	StartingBlock argUint64 `json:"startingBlock"`
	CurrentBlock  argUint64 `json:"currentBlock"`
	HighestBlock  argUint64 `json:"highestBlock"`
}

type feeHistoryResult struct {
	OldestBlock   argUint64     `json:"oldestBlock"`
	BaseFeePerGas []argUint64   `json:"baseFeePerGas,omitempty"`
	GasUsedRatio  []float64     `json:"gasUsedRatio"`
	Reward        [][]argUint64 `json:"reward,omitempty"`
}

func convertToArgUint64Slice(slice []uint64) []argUint64 {
	argSlice := make([]argUint64, len(slice))
	for i, value := range slice {
		argSlice[i] = argUint64(value)
	}

	return argSlice
}

func convertToArgUint64SliceSlice(slice [][]uint64) [][]argUint64 {
	argSlice := make([][]argUint64, len(slice))
	for i, value := range slice {
		argSlice[i] = convertToArgUint64Slice(value)
	}

	return argSlice
}

type OverrideAccount struct {
	Nonce     *argUint64                 `json:"nonce"`
	Code      *argBytes                  `json:"code"`
	Balance   *argUint64                 `json:"balance"`
	State     *map[types.Hash]types.Hash `json:"state"`
	StateDiff *map[types.Hash]types.Hash `json:"stateDiff"`
}

func (o *OverrideAccount) ToType() types.OverrideAccount {
	res := types.OverrideAccount{}

	if o.Nonce != nil {
		res.Nonce = (*uint64)(o.Nonce)
	}

	if o.Code != nil {
		res.Code = *o.Code
	}

	if o.Balance != nil {
		res.Balance = new(big.Int).SetUint64(*(*uint64)(o.Balance))
	}

	if o.State != nil {
		res.State = *o.State
	}

	if o.StateDiff != nil {
		res.StateDiff = *o.StateDiff
	}

	return res
}

// StateOverride is the collection of overridden accounts
type StateOverride map[types.Address]OverrideAccount

// MarshalJSON marshals the StateOverride to JSON
func (s StateOverride) MarshalJSON() ([]byte, error) {
	a := defaultArena.Get()
	defer a.Reset()

	o := a.NewObject()

	for addr, obj := range s {
		oo := a.NewObject()
		if obj.Nonce != nil {
			oo.Set("nonce", a.NewString(fmt.Sprintf("0x%x", *obj.Nonce)))
		}

		if obj.Balance != nil {
			oo.Set("balance", a.NewString(fmt.Sprintf("0x%x", obj.Balance)))
		}

		if obj.Code != nil {
			oo.Set("code", a.NewString("0x"+hex.EncodeToString(*obj.Code)))
		}

		if obj.State != nil {
			ooo := a.NewObject()
			for k, v := range *obj.State {
				ooo.Set(k.String(), a.NewString(v.String()))
			}

			oo.Set("state", ooo)
		}

		if obj.StateDiff != nil {
			ooo := a.NewObject()
			for k, v := range *obj.StateDiff {
				ooo.Set(k.String(), a.NewString(v.String()))
			}

			oo.Set("stateDiff", ooo)
		}

		o.Set(addr.String(), oo)
	}

	res := o.MarshalTo(nil)

	defaultArena.Put(a)

	return res, nil
}

// CallMsg contains parameters for contract calls
type CallMsg struct {
	From       types.Address  // the sender of the 'transaction'
	To         *types.Address // the destination contract (nil for contract creation)
	Gas        uint64         // if 0, the call executes with near-infinite gas
	GasPrice   *big.Int       // wei <-> gas exchange ratio
	GasFeeCap  *big.Int       // EIP-1559 fee cap per gas
	GasTipCap  *big.Int       // EIP-1559 tip per gas
	Value      *big.Int       // amount of wei sent along with the call
	Data       []byte         // input data, usually an ABI-encoded contract method invocation
	Type       uint64
	AccessList types.TxAccessList // EIP-2930 access list
}

// MarshalJSON implements the Marshal interface.
func (c *CallMsg) MarshalJSON() ([]byte, error) {
	a := defaultArena.Get()
	defer a.Reset()

	o := a.NewObject()
	o.Set("from", a.NewString(c.From.String()))

	if c.Gas != 0 {
		o.Set("gas", a.NewString(fmt.Sprintf("0x%x", c.Gas)))
	}

	if c.To != nil {
		o.Set("to", a.NewString(c.To.String()))
	}

	if len(c.Data) != 0 {
		o.Set("data", a.NewString("0x"+hex.EncodeToString(c.Data)))
	}

	if c.GasPrice != nil {
		o.Set("gasPrice", a.NewString(fmt.Sprintf("0x%x", c.GasPrice)))
	}

	if c.Value != nil {
		o.Set("value", a.NewString(fmt.Sprintf("0x%x", c.Value)))
	}

	if c.GasFeeCap != nil {
		o.Set("maxFeePerGas", a.NewString(fmt.Sprintf("0x%x", c.GasFeeCap)))
	}

	if c.GasTipCap != nil {
		o.Set("maxPriorityFeePerGas", a.NewString(fmt.Sprintf("0x%x", c.GasTipCap)))
	}

	if c.Type != 0 {
		o.Set("type", a.NewString(fmt.Sprintf("0x%x", c.Type)))
	}

	if c.AccessList != nil {
		o.Set("accessList", c.AccessList.MarshalJSONWith(a))
	}

	res := o.MarshalTo(nil)

	defaultArena.Put(a)

	return res, nil
}

// FeeHistory represents the fee history data returned by an rpc node
type FeeHistory struct {
	OldestBlock  uint64     `json:"oldestBlock"`
	Reward       [][]uint64 `json:"reward,omitempty"`
	BaseFee      []uint64   `json:"baseFeePerGas,omitempty"`
	GasUsedRatio []float64  `json:"gasUsedRatio"`
}

// UnmarshalJSON unmarshals the FeeHistory object from JSON
func (f *FeeHistory) UnmarshalJSON(data []byte) error {
	var raw feeHistoryResult

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	f.OldestBlock = uint64(raw.OldestBlock)

	if raw.Reward != nil {
		f.Reward = make([][]uint64, 0, len(raw.Reward))

		for _, r := range raw.Reward {
			elem := make([]uint64, 0, len(r))
			for _, i := range r {
				elem = append(elem, uint64(i))
			}

			f.Reward = append(f.Reward, elem)
		}
	}

	f.BaseFee = make([]uint64, 0, len(raw.BaseFeePerGas))
	for _, i := range raw.BaseFeePerGas {
		f.BaseFee = append(f.BaseFee, uint64(i))
	}

	f.GasUsedRatio = raw.GasUsedRatio

	return nil
}

// Transaction is the json rpc transaction object
// (types.Transaction object, expanded with block number, hash and index)
type Transaction struct {
	*types.Transaction

	// BlockNumber is the number of the block in which the transaction was included.
	BlockNumber uint64 `json:"blockNumber"`

	// BlockHash is the hash of the block in which the transaction was included.
	BlockHash types.Hash `json:"blockHash"`

	// TxnIndex is the index of the transaction within the block.
	TxnIndex uint64 `json:"transactionIndex"`
}

// UnmarshalJSON unmarshals the transaction object from JSON
func (t *Transaction) UnmarshalJSON(data []byte) error {
	p := defaultPool.Get()
	defer defaultPool.Put(p)

	v, err := p.Parse(string(data))
	if err != nil {
		return err
	}

	t.Transaction = new(types.Transaction)
	if err := t.Transaction.UnmarshalJSONWith(v); err != nil {
		return err
	}

	if types.HasJSONKey(v, "blockNumber") {
		t.BlockNumber, err = types.UnmarshalJSONUint64(v, "blockNumber")
		if err != nil {
			return err
		}
	}

	if types.HasJSONKey(v, "blockHash") {
		t.BlockHash, err = types.UnmarshalJSONHash(v, "blockHash")
		if err != nil {
			return err
		}
	}

	if types.HasJSONKey(v, "transactionIndex") {
		t.TxnIndex, err = types.UnmarshalJSONUint64(v, "transactionIndex")
		if err != nil {
			return err
		}
	}

	return nil
}
