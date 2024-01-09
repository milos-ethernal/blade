package calltracer

import (
	"math/big"
	"sync"

	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/state/runtime"
	"github.com/0xPolygon/polygon-edge/types"
)

type Call struct {
	Type    string  `json:"type"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Value   string  `json:"value,omitempty"`
	Gas     string  `json:"gas"`
	GasUsed string  `json:"gasUsed"`
	Input   string  `json:"input"`
	Output  string  `json:"output"`
	Calls   []*Call `json:"calls,omitempty"`

	parent   *Call
	startGas uint64
}

type CallTracer struct {
	call               *Call
	activeCall         *Call
	activeGas          uint64
	activeAvailableGas uint64

	cancelLock sync.RWMutex
	reason     error
	stop       bool
}

func (c *CallTracer) Cancel(err error) {
	c.cancelLock.Lock()
	defer c.cancelLock.Unlock()

	c.reason = err
	c.stop = true
}

func (c *CallTracer) cancelled() bool {
	c.cancelLock.RLock()
	defer c.cancelLock.RUnlock()

	return c.stop
}

func (c *CallTracer) Clear() {
	c.call = nil
	c.activeCall = nil
}

func (c *CallTracer) GetResult() (interface{}, error) {
	c.cancelLock.RLock()
	defer c.cancelLock.RUnlock()

	if c.reason != nil {
		return nil, c.reason
	}

	return c.call, nil
}

func (c *CallTracer) TxStart(gasLimit uint64) {
}

func (c *CallTracer) TxEnd(gasLeft uint64) {
}

func (c *CallTracer) CallStart(depth int, from, to types.Address, callType int,
	gas uint64, value *big.Int, input []byte, host runtime.Host) {
	if c.cancelled() {
		return
	}

	typ, ok := runtime.CallTypes[callType]
	if !ok {
		typ = "UNKNOWN"
	}

	val := "0x0"
	if value != nil {
		val = hex.EncodeBig(value)
	}

	call := &Call{
		Type:     typ,
		From:     from.String(),
		To:       to.String(),
		Value:    val,
		Gas:      hex.EncodeUint64(gas),
		GasUsed:  "",
		Input:    hex.EncodeToHex(input),
		Output:   "",
		Calls:    nil,
		startGas: gas,
	}

	if depth == 1 {
		c.call = call
		c.activeCall = call
	} else {
		call.parent = c.activeCall
		c.activeCall.Calls = append(c.activeCall.Calls, call)
		c.activeCall = call
	}
}

func (c *CallTracer) CallEnd(depth int, totalGasUsed uint64, output []byte, err error) {
	c.activeCall.Output = hex.EncodeToHex(output)

	gasUsedByCall := uint64(0)
	if c.activeCall.startGas > c.activeAvailableGas {
		gasUsedByCall = c.activeCall.startGas - c.activeAvailableGas
	}

	c.activeCall.GasUsed = hex.EncodeUint64(gasUsedByCall)
	c.activeGas = 0

	if depth > 1 {
		c.activeCall = c.activeCall.parent
	}

	if err != nil {
		c.Cancel(err)
	}
}

func (c *CallTracer) CaptureState(memory []byte, stack []*big.Int, opCode int,
	contractAddress types.Address, sp int, host runtime.Host, state runtime.VMState) {
	if c.cancelled() {
		state.Halt()
	}
}

func (c *CallTracer) ExecuteState(contractAddress types.Address, ip uint64, opcode int,
	availableGas uint64, cost uint64, lastReturnData []byte, depth int, err error, host runtime.Host) {
	c.activeGas += cost
	c.activeAvailableGas = availableGas
}

func (t *CallTracer) CaptureStateBre(
	opCode, depth int,
	ip, gas, cost uint64,
	returnData []byte,
	scope *runtime.ScopeContext,
	host runtime.Host,
	state runtime.VMState,
	err error,
) {

}

func (t *CallTracer) CaptureStart(
	from, to types.Address,
	callType int,
	input []byte,
	gas uint64,
	value *big.Int,
	host runtime.Host) {
}

func (t *CallTracer) CaptureEnd(output []byte, gasUsed uint64, err error) {}
