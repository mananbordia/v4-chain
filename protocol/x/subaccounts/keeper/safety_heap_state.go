package keeper

import (
	"cosmossdk.io/store/prefix"
	gogotypes "github.com/cosmos/gogoproto/types"
	"github.com/dydxprotocol/v4-chain/protocol/lib"
	"github.com/dydxprotocol/v4-chain/protocol/x/subaccounts/types"
)

type PositionSide uint

const (
	Long PositionSide = iota
	Short
)

// AppendToLast inserts a subaccount into the safety heap.
func (k Keeper) AppendToLast(
	store prefix.Store,
	subaccountId types.SubaccountId,
) {
	length := k.GetSubaccountHeapLength(store)

	k.SetSubaccountAtIndex(store, subaccountId, length)
	k.SetSubaccountHeapLength(store, length+1)
}

// MustRemoveLast inserts a subaccount into the safety heap.
func (k Keeper) MustRemoveLast(
	store prefix.Store,
) (subaccountId types.SubaccountId) {
	length := k.GetSubaccountHeapLength(store)

	if length == 0 {
		panic(types.ErrSafetyHeapEmpty)
	}

	subaccountId, _ = k.GetSubaccountAtIndex(store, length-1)
	k.DeleteSubaccountAtIndex(store, length-1)
	k.SetSubaccountHeapLength(store, length-1)
	return subaccountId
}

// MustGetSubaccountAtIndex returns the subaccount at the given index.
func (k Keeper) MustGetSubaccountAtIndex(
	store prefix.Store,
	heapIndex uint32,
) (
	subaccountId types.SubaccountId,
) {
	subaccountId, found := k.GetSubaccountAtIndex(store, heapIndex)
	if !found {
		panic(types.ErrSafetyHeapSubaccountNotFoundAtIndex)
	}
	return subaccountId
}

// GetSubaccountAtIndex returns the subaccount at the given index.
func (k Keeper) GetSubaccountAtIndex(
	store prefix.Store,
	heapIndex uint32,
) (
	subaccountId types.SubaccountId,
	found bool,
) {
	key := lib.Uint32ToKey(heapIndex)

	b := store.Get(key)

	if b != nil {
		k.cdc.MustUnmarshal(b, &subaccountId)
	}
	return subaccountId, b != nil
}

// SetSubaccountAtIndex updates the subaccount at the given index.
func (k Keeper) SetSubaccountAtIndex(
	store prefix.Store,
	subaccountId types.SubaccountId,
	heapIndex uint32,
) {
	key := lib.Uint32ToKey(heapIndex)

	store.Set(
		key,
		k.cdc.MustMarshal(&subaccountId),
	)
}

// DeleteSubaccountAtIndex deletes the subaccount at the given index.
func (k Keeper) DeleteSubaccountAtIndex(
	store prefix.Store,
	heapIndex uint32,
) {
	key := lib.Uint32ToKey(heapIndex)
	store.Delete(key)
}

// MustGetSubaccountHeapIndex returns the heap index of the subaccount.
func (k Keeper) MustGetSubaccountHeapIndex(
	store prefix.Store,
	subaccountId types.SubaccountId,
) (
	heapIndex uint32,
) {
	key := subaccountId.ToStateKey()

	index := gogotypes.UInt32Value{Value: 0}
	b := store.Get(key)

	if b == nil {
		panic(types.ErrSafetyHeapSubaccountNotFoundAtIndex)
	}

	k.cdc.MustUnmarshal(b, &index)

	return index.Value
}

// SetSubaccountHeapIndex sets the heap index of the subaccount.
func (k Keeper) SetSubaccountHeapIndex(
	store prefix.Store,
	subaccountId types.SubaccountId,
	heapIndex uint32,
) {
	key := subaccountId.ToStateKey()

	index := gogotypes.UInt32Value{Value: heapIndex}
	store.Set(
		key,
		k.cdc.MustMarshal(&index),
	)
}

// GetSubaccountHeapLength returns the length of heap.
func (k Keeper) GetSubaccountHeapLength(
	store prefix.Store,
) (
	length uint32,
) {
	key := []byte(types.SafetyHeapLengthPrefix)

	index := gogotypes.UInt32Value{Value: 0}
	b := store.Get(key)

	if b != nil {
		k.cdc.MustUnmarshal(b, &index)
	}

	return index.Value
}

// SetSubaccountHeapLength sets the heap length.
func (k Keeper) SetSubaccountHeapLength(
	store prefix.Store,
	length uint32,
) {
	key := []byte(types.SafetyHeapLengthPrefix)

	index := gogotypes.UInt32Value{Value: length}
	store.Set(
		key,
		k.cdc.MustMarshal(&index),
	)
}
