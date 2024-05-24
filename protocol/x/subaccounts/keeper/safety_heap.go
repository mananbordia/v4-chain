package keeper

import (
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4-chain/protocol/lib"
	"github.com/dydxprotocol/v4-chain/protocol/x/subaccounts/types"
)

// TODO: optimize this
func (k Keeper) GetSubaccountsWithOpenPositionsOnSide(
	ctx sdk.Context,
	perpetualId uint32,
	side types.PositionSide,
) []types.SubaccountId {
	store := k.GetSafetyHeapStore(ctx, perpetualId, side)

	iterator := storetypes.KVStoreReversePrefixIterator(store, []byte{})
	defer iterator.Close()

	result := make([]types.SubaccountId, 0)
	for ; iterator.Valid(); iterator.Next() {
		var val types.SubaccountId
		k.cdc.MustUnmarshal(iterator.Value(), &val)
		result = append(result, val)
	}
	return result
}

func (k Keeper) GetAllNegativeTncSubaccounts(
	ctx sdk.Context,
) []types.SubaccountId {
	perpetuals := k.perpetualsKeeper.GetAllPerpetuals(ctx)
	negativeTncSubaccounts := make(map[types.SubaccountId]bool)

	for _, perpetual := range perpetuals {
		for _, side := range []types.PositionSide{types.Long, types.Short} {
			store := k.GetSafetyHeapStore(ctx, perpetual.GetId(), side)
			subaccounts := k.GetNegativeTncSubaccounts(ctx, store, 0)

			for _, subaccountId := range subaccounts {
				negativeTncSubaccounts[subaccountId] = true
			}
		}
	}

	sortedSubaccountIds := lib.GetSortedKeys[types.SortedSubaccountIds](negativeTncSubaccounts)
	return sortedSubaccountIds
}

func (k Keeper) GetNegativeTncSubaccounts(
	ctx sdk.Context,
	store prefix.Store,
	index uint32,
) []types.SubaccountId {
	result := []types.SubaccountId{}

	subaccountId, found := k.GetSubaccountAtIndex(store, index)
	if !found {
		return result
	}

	settledUpdates, _, err := k.getSettledUpdates(
		ctx,
		[]types.Update{
			{
				SubaccountId: subaccountId,
			},
		},
		true,
	)
	if err != nil {
		panic(err)
	}

	tnc, _, _, err :=
		k.internalGetNetCollateralAndMarginRequirements(
			ctx,
			settledUpdates[0],
		)
	if err != nil {
		panic(err)
	}

	if tnc.Sign() == -1 {
		result = append(result, subaccountId)

		// Recursively get the negative TNC subaccounts for left and right children.
		result = append(result, k.GetNegativeTncSubaccounts(ctx, store, 2*index+1)...)
		result = append(result, k.GetNegativeTncSubaccounts(ctx, store, 2*index+2)...)
	}
	return result
}

func (k Keeper) RemoveSubaccountFromSafetyHeap(
	ctx sdk.Context,
	subaccountId types.SubaccountId,
) {
	oldSubaccount := k.GetSubaccount(ctx, subaccountId)

	// TODO: optimize this
	for _, position := range oldSubaccount.PerpetualPositions {
		var side types.PositionSide
		if position.Quantums.BigInt().Sign() == 1 {
			side = types.Long
		} else {
			side = types.Short
		}

		store := k.GetSafetyHeapStore(ctx, position.PerpetualId, side)
		index := k.MustGetSubaccountHeapIndex(store, subaccountId)
		k.MustRemoveElementAtIndex(ctx, store, index)
	}
}

func (k Keeper) AddSubaccountToSafetyHeap(
	ctx sdk.Context,
	subaccountId types.SubaccountId,
) {
	subaccount := k.GetSubaccount(ctx, subaccountId)

	// TODO: optimize this
	for _, position := range subaccount.PerpetualPositions {
		var side types.PositionSide
		if position.Quantums.BigInt().Sign() == 1 {
			side = types.Long
		} else {
			side = types.Short
		}

		store := k.GetSafetyHeapStore(ctx, position.PerpetualId, side)
		k.Insert(ctx, store, subaccountId)
	}
}

func (k Keeper) Insert(
	ctx sdk.Context,
	store prefix.Store,
	subaccountId types.SubaccountId,
) {
	length := k.GetSubaccountHeapLength(store)
	k.AppendToLast(store, subaccountId)
	k.HeapifyUp(ctx, store, length)
}

func (k Keeper) MustRemoveElementAtIndex(
	ctx sdk.Context,
	store prefix.Store,
	index uint32,
) {
	length := k.GetSubaccountHeapLength(store)
	if index >= length {
		panic(types.ErrSafetyHeapSubaccountNotFoundAtIndex)
	}

	if index == length-1 {
		k.MustRemoveLast(store)
		return
	}

	// Swap the min element with the last element.
	k.Swap(store, index, length-1)

	// Remove the min element.
	k.MustRemoveLast(store)

	// Heapify down the root element.
	// Only do this when we are not removing the last element.
	k.HeapifyDown(ctx, store, index)
	k.HeapifyUp(ctx, store, index)
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
	length := k.GetSubaccountHeapLength(store)
	leftIndex, rightIndex := 2*index+1, 2*index+2
	if rightIndex < length && k.Less(ctx, store, rightIndex, leftIndex) {
		// Compare the current node with the right child.
		if k.Less(ctx, store, rightIndex, index) {
			k.Swap(store, index, rightIndex)
			k.HeapifyDown(ctx, store, rightIndex)
		}
	} else if leftIndex < length {
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
		panic(err)
	}

	firstTnc, _, firstMmr, err :=
		k.internalGetNetCollateralAndMarginRequirements(
			ctx,
			settledUpdates[0],
		)
	if err != nil {
		panic(err)
	}

	secondTnc, _, secondMmr, err :=
		k.internalGetNetCollateralAndMarginRequirements(
			ctx,
			settledUpdates[1],
		)
	if err != nil {
		panic(err)
	}

	// first tnc / first mmr < second tnc / second mmr
	// or equivalengthly,
	// first tnc * second mmr < second tnc * first mmr
	return firstTnc.Mul(firstTnc, secondMmr).Cmp(secondTnc.Mul(secondTnc, firstMmr)) < 0
}
