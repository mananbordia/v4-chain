package ante

import (
	errorsmod "cosmossdk.io/errors"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/dydxprotocol/v4-chain/protocol/x/clob/types"
)

var _ sdktypes.AnteDecorator = (*ClobTimestampDecorator)(nil)

type ClobTimestampDecorator struct {
	clobKeeper types.ClobKeeper
}

func NewTimestampDecorator(clobKeeper types.ClobKeeper) ClobTimestampDecorator {
	return ClobTimestampDecorator{
		clobKeeper,
	}
}

func (r ClobTimestampDecorator) AnteHandle(
	ctx sdktypes.Context,
	tx sdktypes.Tx,
	simulate bool,
	next sdktypes.AnteHandler,
) (newCtx sdktypes.Context, err error) {
	for _, msg := range tx.GetMsgs() {
		switch msg := msg.(type) {
		case *types.MsgXOperate:
			sigTx, ok := tx.(authsigning.Tx)
			if !ok {
				return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
			}
			sigs, err := sigTx.GetSignaturesV2()
			if err != nil {
				return ctx, err
			}
			if len(sigs) != 1 {
				return ctx, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "only one signature is allowed")
			}
			sequence := sigs[0].Sequence
			accountNumber, _ := types.SplitSID(msg.Sid)

			ok = r.clobKeeper.PostTimestamp(ctx, accountNumber, sequence)
			if !ok {
				return ctx, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "invalid timestamp")
			}
		}
	}
	return next(ctx, tx, simulate)
}
