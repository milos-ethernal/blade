package types

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/umbracle/fastrlp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codec interface {
	RLPMarshaler
	RLPUnmarshaler
}

func TestRLPEncoding(t *testing.T) {
	cases := []codec{
		&Header{},
		&Receipt{},
	}
	for _, c := range cases {
		buf := c.MarshalRLPTo(nil)

		res, ok := reflect.New(reflect.TypeOf(c).Elem()).Interface().(codec)
		if !ok {
			t.Fatalf("Unable to assert type")
		}

		if err := res.UnmarshalRLP(buf); err != nil {
			t.Fatal(err)
		}

		buf2 := c.MarshalRLPTo(nil)
		if !reflect.DeepEqual(buf, buf2) {
			t.Fatal("[ERROR] Buffers not equal")
		}
	}
}

func TestRLPMarshall_And_Unmarshall_Legacy_Transaction(t *testing.T) {
	addrTo := StringToAddress("11")
	txn := NewTx(&LegacyTx{
		GasPrice: big.NewInt(11),
		BaseTx: &BaseTx{
			Nonce: 0,
			Gas:   11,
			To:    &addrTo,
			Value: big.NewInt(1),
			Input: []byte{1, 2},
			V:     big.NewInt(25),
			S:     big.NewInt(26),
			R:     big.NewInt(27),
		},
	})

	txn.ComputeHash()

	unmarshalledTxn := NewTx(&LegacyTx{})
	marshaledRlp := txn.MarshalRLP()

	if err := unmarshalledTxn.UnmarshalRLP(marshaledRlp); err != nil {
		t.Fatal(err)
	}

	unmarshalledTxn.ComputeHash()

	assert.Equal(t, txn.Inner, unmarshalledTxn.Inner, "[ERROR] Unmarshalled transaction not equal to base transaction")
}

func TestRLPStorage_Marshall_And_Unmarshall_Receipt(t *testing.T) {
	var (
		addr = StringToAddress("11")
		hash = StringToHash("10")

		statusSuccess = ReceiptSuccess
		statusFailed  = ReceiptFailed
	)

	testTable := []struct {
		name    string
		receipt *Receipt
		status  *ReceiptStatus
	}{
		{
			"Marshal receipt with success status",
			&Receipt{
				CumulativeGasUsed: 10,
				GasUsed:           100,
				ContractAddress:   &addr,
				TxHash:            hash,
				Status:            &statusSuccess,
			},
			&statusSuccess,
		},
		{
			"Marshal receipt with failed status",
			&Receipt{
				CumulativeGasUsed: 10,
				GasUsed:           100,
				ContractAddress:   &addr,
				TxHash:            hash,
				Status:            &statusFailed,
			},
			&statusFailed,
		},
		{
			"Marshal receipt without status",
			&Receipt{
				Root:              hash,
				CumulativeGasUsed: 10,
				GasUsed:           100,
				ContractAddress:   &addr,
				TxHash:            hash,
			},
			nil,
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			receipt := testCase.receipt

			if testCase.status != nil {
				receipt.SetStatus(*testCase.status)
			}

			marshalledReceipt := receipt.MarshalStoreRLPTo(nil)

			unmarshalledReceipt := new(Receipt)

			err := unmarshalledReceipt.UnmarshalStoreRLP(marshalledReceipt)
			require.NoError(t, err)

			require.Exactly(t, receipt, unmarshalledReceipt, "Unmarshalled receipt not equal to the base receipt")
		})
	}
}

func TestRLPUnmarshal_Header_ComputeHash(t *testing.T) {
	// header computes hash after unmarshalling
	h := &Header{}
	h.ComputeHash()

	data := h.MarshalRLP()
	h2 := new(Header)
	assert.NoError(t, h2.UnmarshalRLP(data))
	assert.Equal(t, h.Hash, h2.Hash)
}

func TestRLPMarshall_And_Unmarshall_TypedTransaction(t *testing.T) {
	addrTo := StringToAddress("11")
	addrFrom := StringToAddress("22")

	originalTxs := []*Transaction{
		NewTx(&StateTx{
			GasPrice: big.NewInt(11),
			BaseTx: &BaseTx{
				Nonce: 0,
				Gas:   11,
				To:    &addrTo,
				From:  addrFrom,
				Value: big.NewInt(1),
				Input: []byte{1, 2},
				V:     big.NewInt(25),
				S:     big.NewInt(26),
				R:     big.NewInt(27),
			},
		}),
		NewTx(&LegacyTx{
			GasPrice: big.NewInt(11),
			BaseTx: &BaseTx{
				Nonce: 0,
				Gas:   11,
				To:    &addrTo,
				From:  addrFrom,
				Value: big.NewInt(1),
				Input: []byte{1, 2},
				V:     big.NewInt(25),
				S:     big.NewInt(26),
				R:     big.NewInt(27),
			},
		}),
		NewTx(&DynamicFeeTx{
			GasFeeCap: big.NewInt(12),
			GasTipCap: big.NewInt(13),
			BaseTx: &BaseTx{
				Nonce: 0,
				Gas:   11,
				To:    &addrTo,
				From:  addrFrom,
				Value: big.NewInt(1),
				Input: []byte{1, 2},
				V:     big.NewInt(25),
				S:     big.NewInt(26),
				R:     big.NewInt(27),
			},
		}),
	}

	for _, originalTx := range originalTxs {
		t.Run(originalTx.Type().String(), func(t *testing.T) {
			originalTx.ComputeHash()

			unmarshalledTx := &Transaction{}
			unmarshalledTx.InitInnerData(originalTx.Type())

			txRLP := originalTx.MarshalRLP()

			assert.NoError(t, unmarshalledTx.UnmarshalRLP(txRLP))

			unmarshalledTx.ComputeHash()
			assert.Equal(t, originalTx.Type(), unmarshalledTx.Type())
			assert.Equal(t, originalTx.Hash(), unmarshalledTx.Hash())
		})
	}
}

func TestRLPMarshall_Unmarshall_Missing_Data(t *testing.T) {
	t.Parallel()

	txTypes := []TxType{
		StateTxType,
		LegacyTxType,
		DynamicFeeTxType,
	}

	for _, txType := range txTypes {
		txType := txType
		testTable := []struct {
			name          string
			expectedErr   bool
			omittedValues map[string]bool
			fromAddrSet   bool
		}{
			{
				name:        fmt.Sprintf("[%s] Insufficient params", txType),
				expectedErr: true,
				omittedValues: map[string]bool{
					"Nonce":    true,
					"GasPrice": true,
				},
			},
			{
				name:        fmt.Sprintf("[%s] Missing From", txType),
				expectedErr: false,
				omittedValues: map[string]bool{
					"ChainID":    txType != DynamicFeeTxType,
					"GasTipCap":  txType != DynamicFeeTxType,
					"GasFeeCap":  txType != DynamicFeeTxType,
					"GasPrice":   txType == DynamicFeeTxType,
					"AccessList": txType != DynamicFeeTxType,
					"From":       txType != StateTxType,
				},
				fromAddrSet: txType == StateTxType,
			},
			{
				name:        fmt.Sprintf("[%s] Address set for state tx only", txType),
				expectedErr: false,
				omittedValues: map[string]bool{
					"ChainID":    txType != DynamicFeeTxType,
					"GasTipCap":  txType != DynamicFeeTxType,
					"GasFeeCap":  txType != DynamicFeeTxType,
					"GasPrice":   txType == DynamicFeeTxType,
					"AccessList": txType != DynamicFeeTxType,
					"From":       txType != StateTxType,
				},
				fromAddrSet: txType == StateTxType,
			},
		}

		for _, tt := range testTable {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				arena := fastrlp.DefaultArenaPool.Get()
				parser := fastrlp.DefaultParserPool.Get()
				testData := testRLPData(arena, tt.omittedValues)
				v, err := parser.Parse(testData)
				assert.Nil(t, err)

				unmarshalledTx := NewTxWithType(txType)

				if tt.expectedErr {
					assert.Error(t, unmarshalledTx.Inner.unmarshalRLPFrom(parser, v), tt.name)
				} else {
					assert.NoError(t, unmarshalledTx.Inner.unmarshalRLPFrom(parser, v), tt.name)
					assert.Equal(t, tt.fromAddrSet, len(unmarshalledTx.From()) != 0 && unmarshalledTx.From() != ZeroAddress, unmarshalledTx.Type().String(), unmarshalledTx.From())
				}

				fastrlp.DefaultParserPool.Put(parser)
				fastrlp.DefaultArenaPool.Put(arena)
			})
		}
	}
}

func TestRLPMarshall_And_Unmarshall_TxType(t *testing.T) {
	testTable := []struct {
		name        string
		txType      TxType
		expectedErr bool
	}{
		{
			name:   "StateTx",
			txType: StateTxType,
		},
		{
			name:   "LegacyTx",
			txType: LegacyTxType,
		},
		{
			name:   "DynamicFeeTx",
			txType: DynamicFeeTxType,
		},
		{
			name:        "undefined type",
			txType:      TxType(0x09),
			expectedErr: true,
		},
	}

	for _, tt := range testTable {
		ar := &fastrlp.Arena{}

		var txType TxType
		err := txType.unmarshalRLPFrom(nil, ar.NewBytes([]byte{byte(tt.txType)}))

		if tt.expectedErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.txType, txType)
		}
	}
}

func testRLPData(arena *fastrlp.Arena, omitValues map[string]bool) []byte {
	vv := arena.NewArray()

	if omit := omitValues["ChainID"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(0)))
	}

	if omit := omitValues["Nonce"]; !omit {
		vv.Set(arena.NewUint(10))
	}

	if omit := omitValues["GasTipCap"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(11)))
	}

	if omit := omitValues["GasFeeCap"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(11)))
	}

	if omit := omitValues["GasPrice"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(11)))
	}

	if omit := omitValues["Gas"]; !omit {
		vv.Set(arena.NewUint(12))
	}

	if omit := omitValues["To"]; !omit {
		vv.Set(arena.NewBytes((StringToAddress("13")).Bytes()))
	}

	if omit := omitValues["Value"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(14)))
	}

	if omit := omitValues["Input"]; !omit {
		vv.Set(arena.NewCopyBytes([]byte{1, 2}))
	}

	if omit := omitValues["AccessList"]; !omit {
		vv.Set(arena.NewArray())
	}

	if omit := omitValues["V"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(15)))
	}

	if omit := omitValues["R"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(16)))
	}

	if omit := omitValues["S"]; !omit {
		vv.Set(arena.NewBigInt(big.NewInt(17)))
	}

	if omit := omitValues["From"]; !omit {
		vv.Set(arena.NewBytes((StringToAddress("18")).Bytes()))
	}

	var testData []byte
	testData = vv.MarshalTo(testData)

	return testData
}

func Test_MarshalCorruptedBytesArray(t *testing.T) {
	t.Parallel()

	marshal := func(obj func(*fastrlp.Arena) *fastrlp.Value) []byte {
		ar := fastrlp.DefaultArenaPool.Get()
		defer fastrlp.DefaultArenaPool.Put(ar)

		return obj(ar).MarshalTo(nil)
	}

	emptyArray := [8]byte{}
	corruptedSlice := make([]byte, 32)
	corruptedSlice[29], corruptedSlice[30], corruptedSlice[31] = 5, 126, 64
	intOfCorruption := uint64(18_446_744_073_709_551_615) // 2^64-1

	marshalOne := func(ar *fastrlp.Arena) *fastrlp.Value {
		return ar.NewBytes(corruptedSlice)
	}

	marshalTwo := func(ar *fastrlp.Arena) *fastrlp.Value {
		return ar.NewUint(intOfCorruption)
	}

	marshal(marshalOne)

	require.Equal(t, emptyArray[:], corruptedSlice[:len(emptyArray)])

	marshal(marshalTwo) // without fixing this, marshaling will cause corruption of the corrupted slice

	require.Equal(t, emptyArray[:], corruptedSlice[:len(emptyArray)])
}
