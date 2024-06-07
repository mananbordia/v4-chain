package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	clobtypes "github.com/dydxprotocol/v4-chain/protocol/x/clob/types"
)

const (
	// Default is the default sequence validation type.
	SeqVal_Default uint64 = iota
	// GoodTilBlock is the sequence validation type for messages that use `GoodTilBlock`.
	SeqVal_GoodTilBlock
	// XOperate is the sequence validation type for `MsgXOperate`.
	SeqVal_XOperate
)

func GetSequenceValidationType(msgs []sdk.Msg) (version uint64) {
	for _, msg := range msgs {
		switch typedMsg := msg.(type) {
		case
			*clobtypes.MsgPlaceOrder:
			// Stateful orders need to use sequence numbers for replay prevention.
			orderId := typedMsg.GetOrder().OrderId
			if orderId.IsStatefulOrder() {
				return SeqVal_Default
			}
			// This is a short term order, continue to check the next message.
			continue
		case
			*clobtypes.MsgCancelOrder:
			orderId := typedMsg.GetOrderId()
			if orderId.IsStatefulOrder() {
				return SeqVal_Default
			}
			// This is a `GoodTilBlock` message, continue to check the next message.
			continue
		case
			*clobtypes.MsgXOperate:
			return SeqVal_XOperate
		default:
			// Early return for messages that require sequence number validation.
			return SeqVal_Default
		}
	}
	// All messages use `GoodTilBlock`.
	return SeqVal_GoodTilBlock
}
