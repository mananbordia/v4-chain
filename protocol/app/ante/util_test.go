package ante_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4-chain/protocol/app/ante"
	"github.com/dydxprotocol/v4-chain/protocol/testutil/constants"
	"github.com/stretchr/testify/require"
)

type ValidationUtilTestCase struct {
	msgs     []sdk.Msg
	expected uint64
}

func TestGetSequenceValidationType(t *testing.T) {
	testCases := map[string]ValidationUtilTestCase{
		"single place order message": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder,
			},
			expected: ante.SeqVal_GoodTilBlock,
		},
		"single cancel order message": {
			msgs: []sdk.Msg{
				constants.Msg_CancelOrder,
			},
			expected: ante.SeqVal_GoodTilBlock,
		},
		"single transfer message": {
			msgs: []sdk.Msg{
				constants.Msg_Transfer,
			},
			expected: ante.SeqVal_Default,
		},
		"single send message": {
			msgs: []sdk.Msg{
				constants.Msg_Send,
			},
			expected: ante.SeqVal_Default,
		},
		"single long term order": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder_LongTerm,
			},
			expected: ante.SeqVal_Default,
		},
		"single long term cancel": {
			msgs: []sdk.Msg{
				constants.Msg_CancelOrder_LongTerm,
			},
			expected: ante.SeqVal_Default,
		},
		"single conditional order": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder_Conditional,
			},
			expected: ante.SeqVal_Default,
		},
		"multiple GTB messages": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder,
				constants.Msg_CancelOrder,
			},
			expected: ante.SeqVal_GoodTilBlock,
		},
		"mix of GTB messages and non-GTB messages": {
			msgs: []sdk.Msg{
				constants.Msg_Transfer,
				constants.Msg_Send,
			},
			expected: ante.SeqVal_Default,
		},
		"mix of long term orders and short term orders": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder,
				constants.Msg_PlaceOrder_LongTerm,
			},
			expected: ante.SeqVal_Default,
		},
		"mix of conditional orders and short term orders": {
			msgs: []sdk.Msg{
				constants.Msg_PlaceOrder,
				constants.Msg_PlaceOrder_Conditional,
			},
			expected: ante.SeqVal_Default,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(
				t,
				tc.expected,
				ante.GetSequenceValidationType(tc.msgs),
			)
		})
	}
}
