package keeper

import (
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4-chain/protocol/x/subaccounts/types"
)

func (k Keeper) UpdateSafetyHeap(
	ctx sdk.Context,
	subaccount types.Subaccount,
) {
	subaccountId := subaccount.Id
	oldSubaccount := k.GetSubaccount(ctx, *subaccountId)

	// TODO: optimize this
	for _, position := range oldSubaccount.PerpetualPositions {
		var side PositionSide
		if position.Quantums.BigInt().Sign() == 1 {
			side = Long
		} else {
			side = Short
		}

		store := k.GetSafetyHeapStore(ctx, position.PerpetualId, side)
		index := k.MustGetSubaccountHeapIndex(store, *subaccountId)
		_, _ = k.RemoveElementAtIndex(ctx, store, index)
	}

	for _, position := range subaccount.PerpetualPositions {
		var side PositionSide
		if position.Quantums.BigInt().Sign() == 1 {
			side = Long
		} else {
			side = Short
		}

		store := k.GetSafetyHeapStore(ctx, position.PerpetualId, side)
		k.Insert(ctx, store, *subaccountId)
	}
}

func (k Keeper) Insert(
	ctx sdk.Context,
	store prefix.Store,
	subaccountId types.SubaccountId,
) {
	length := k.GetSubaccountHeapLength(store)

	k.SetSubaccountAtIndex(store, subaccountId, length)
	k.SetSubaccountHeapLength(store, length+1)

	k.HeapifyUp(ctx, store, length)
}

func (k Keeper) RemoveElementAtIndex(
	ctx sdk.Context,
	store prefix.Store,
	index uint32,
) (
	subaccountId types.SubaccountId,
	exists bool,
) {
	length := k.GetSubaccountHeapLength(store)
	if index >= length {
		return types.SubaccountId{}, false
	}

	// Swap the min element with the last element.
	k.Swap(store, index, length-1)

	// Remove the min element.
	minSubaccount := k.MustRemoveLast(store)

	// Heapify down the root element.
	k.HeapifyDown(ctx, store, index)

	return minSubaccount, true
}

func (k Keeper) ExtractMin(
	ctx sdk.Context,
	store prefix.Store,
) (
	subaccountId types.SubaccountId,
	exists bool,
) {
	return k.RemoveElementAtIndex(ctx, store, 0)
}

func (k Keeper) HeapifyUp(
	ctx sdk.Context,
	store prefix.Store,
	index uint32,
) {
	if index == 0 {
		return
	}

	parentIndex := (index - 1) / 2
	if k.Less(ctx, store, index, parentIndex) {
		k.Swap(store, index, parentIndex)
		k.HeapifyUp(ctx, store, parentIndex)
	}
}

func (k Keeper) HeapifyDown(
	ctx sdk.Context,
	store prefix.Store,
	index uint32,
) {
	last := k.GetSubaccountHeapLength(store) - 1
	leftIndex, rightIndex := 2*index+1, 2*index+2

	if rightIndex <= last && k.Less(ctx, store, rightIndex, leftIndex) {
		// Compare the current node with the right child.
		if k.Less(ctx, store, rightIndex, index) {
			k.Swap(store, index, rightIndex)
			k.HeapifyDown(ctx, store, rightIndex)
		}
	} else if leftIndex <= last {
		// Compare the current node with the left child.
		if k.Less(ctx, store, leftIndex, index) {
			k.Swap(store, index, leftIndex)
			k.HeapifyDown(ctx, store, leftIndex)
		}
	}
}

func (k Keeper) Swap(
	store prefix.Store,
	index1 uint32,
	index2 uint32,
) {
	first := k.MustGetSubaccountAtIndex(store, index1)
	second := k.MustGetSubaccountAtIndex(store, index2)
	k.SetSubaccountAtIndex(store, first, index2)
	k.SetSubaccountAtIndex(store, second, index1)
}

func (k Keeper) Less(
	ctx sdk.Context,
	store prefix.Store,
	first uint32,
	second uint32,
) bool {
	settledUpdates, _, err := k.getSettledUpdates(
		ctx,
		[]types.Update{
			{
				SubaccountId: k.MustGetSubaccountAtIndex(store, first),
			},
			{
				SubaccountId: k.MustGetSubaccountAtIndex(store, second),
			},
		},
		true,
	)
	if err != nil {
		panic(types.ErrFailedToCompareSafety)
	}

	firstTnc, _, firstMmr, err :=
		k.internalGetNetCollateralAndMarginRequirements(
			ctx,
			settledUpdates[0],
		)
	if err != nil {
		panic(types.ErrFailedToCompareSafety)
	}

	secondTnc, _, secondMmr, err :=
		k.internalGetNetCollateralAndMarginRequirements(
			ctx,
			settledUpdates[1],
		)
	if err != nil {
		panic(types.ErrFailedToCompareSafety)
	}

	// first tnc / first mmr < second tnc / second mmr
	// or equivalengthly,
	// first tnc * second mmr < second tnc * first mmr
	return firstTnc.Mul(firstTnc, secondMmr).Cmp(secondTnc.Mul(secondTnc, firstMmr)) < 0
}
