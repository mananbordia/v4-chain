package keeper

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4-chain/protocol/lib"
	assettypes "github.com/dydxprotocol/v4-chain/protocol/x/assets/types"
	"github.com/dydxprotocol/v4-chain/protocol/x/clob/types"
	satypes "github.com/dydxprotocol/v4-chain/protocol/x/subaccounts/types"
)

func (k Keeper) GetXTimestampStore(ctx sdk.Context) prefix.Store {
	return prefix.NewStore(
		ctx.KVStore(k.storeKey),
		[]byte(types.XTimestampKeyPrefix),
	)
}
func (k Keeper) GetXOrderStore(ctx sdk.Context) prefix.Store {
	return prefix.NewStore(
		ctx.KVStore(k.storeKey),
		[]byte(types.XOrderKeyPrefix),
	)
}
func (k Keeper) GetXStopStore(ctx sdk.Context) prefix.Store {
	return prefix.NewStore(
		ctx.KVStore(k.storeKey),
		[]byte(types.XStopKeyPrefix),
	)
}
func (k Keeper) GetXRestingStore(ctx sdk.Context) prefix.Store {
	return prefix.NewStore(
		ctx.KVStore(k.storeKey),
		[]byte(types.XRestingKeyPrefix),
	)
}
func (k Keeper) GetXExpiryStore(ctx sdk.Context) prefix.Store {
	return prefix.NewStore(
		ctx.KVStore(k.storeKey),
		[]byte(types.XExpiryKeyPrefix),
	)
}

const (
	maxNumTimestamps = 20
)

// PostTimestamp adds a timestamp to the account's list of sorted timestamps.
// Returns true if successful.
// Is not successful if:
// - the timestamp already exists in the list
// - the list is already at maxNumTimestamps and the new timestamp is smaller
func (k Keeper) PostTimestamp(
	ctx sdk.Context,
	accountId uint64,
	timestamp uint64,
) (ok bool) {
	store := k.GetXTimestampStore(ctx)
	key := lib.Uint64ToBytes(accountId)
	rawBytes := store.Get(key)
	if len(rawBytes)%8 != 0 {
		panic(fmt.Errorf("PostTimestamp: unexpected length of rawBytes: %d", len(rawBytes)))
	}

	// Unmarshal the existing timestamps.
	// Return false if at-least-one of the following:
	// - the timestamp is already in the list
	// - the list is full and the new timestamp is smaller than the smallest element
	oldLen := len(rawBytes) / 8
	newLen := lib.Min(oldLen+1, maxNumTimestamps)
	timestamps := make([]uint64, newLen)

	// Write the new timestamp list backwards, adding in the new timestamp, sorted.
	hasAdded := false
	for o, n := oldLen-1, newLen-1; n >= 0; o, n = o-1, n-1 {
		u := lib.BytesToUint64(rawBytes[o*8 : (o+1)*8])

		// Don't allow duplicates
		if u == timestamp {
			return false
		}

		// Add the new timestamp if it is greater than the current timestamp and hasn't been added yet,
		// Otherwise copy from the old list.
		if !hasAdded && timestamp > u {
			timestamps[n] = timestamp
			hasAdded = true
			n--
		} else {
			timestamps[n] = u
		}
	}

	// The new value was too small to be added.
	if !hasAdded {
		return false
	}

	// Write the new timestamps to the store.
	rawBytes = make([]byte, newLen*8)
	for i, ts := range timestamps {
		binary.BigEndian.PutUint64(rawBytes[i*8:(i+1)*8], ts)
	}
	store.Set(key, rawBytes)
	return true
}

func (k Keeper) TriggerOrder(
	ctx sdk.Context,
	uidBytes []byte,
) (
	sizeSum uint64, // filled size
	sizeRem uint64, // remaining size
	err error,
) {
	// Get the order.
	orderStore := k.GetXOrderStore(ctx)
	var order types.XOrder
	k.cdc.MustUnmarshal(orderStore.Get(uidBytes), &order)
	priority := order.GetPriority()

	// Check if the order is triggerable.
	if !order.Base.IsStop() {
		return 0, 0, fmt.Errorf("order is not triggerable")
	}

	// Remove the order from the stop store.
	stopStore := k.GetXStopStore(ctx)
	key := order.ToStopKey(priority)
	if !stopStore.Has(key) {
		return 0, 0, fmt.Errorf("order not found in stop store")
	}
	stopStore.Delete(key)

	// Remove the order from the expiry store.
	if order.Base.HasGoodTilTime() {
		expiryStore := k.GetXExpiryStore(ctx)
		key = order.ToExpiryKey(priority)
		if !expiryStore.Has(key) {
			panic(fmt.Errorf("expiry does not exist with key %s", key))
		}
		expiryStore.Delete(key)
	}

	// Remove the order from the order store.
	orderStore.Delete(uidBytes)

	// Process the order.
	return k.ProcessLiveOrder(ctx, order)
}

func (k Keeper) GetOrderById(
	ctx sdk.Context,
	uidBytes []byte,
) (
	order types.XOrder,
	found bool,
) {
	orderStore := k.GetXOrderStore(ctx)
	orderBytes := orderStore.Get(uidBytes)
	if len(orderBytes) == 0 {
		return types.XOrder{}, false
	}
	k.cdc.MustUnmarshal(orderBytes, &order)
	return order, true
}

func (k Keeper) ProcessOrder(
	ctx sdk.Context,
	order types.XOrder,
	placeFlags uint32,
) (
	sizeSum uint64, // filled size
	sizeRem uint64, // remaining size
	err error,
) {
	// Get existing order.
	orderStore := k.GetXOrderStore(ctx)
	uidBytes := order.Uid.ToBytes()
	orderExists := orderStore.Has(uidBytes)

	// Replacement Flags
	replaceFlags := types.GetReplaceFlags(placeFlags)
	switch replaceFlags {
	// New-only orders: Return error if order already exists.
	case types.Order_REPLACE_FLAGS_NEW_ONLY:
		if orderExists {
			return 0, 0, fmt.Errorf("order already exists with key %s", uidBytes)
		}
	// Upsert orders: Remove existing order if it exists.
	case types.Order_REPLACE_FLAGS_UPSERT:
		if orderExists {
			k.RemoveOrderById(ctx, uidBytes)
		}
	// Incremental size orders: If order exists, remove it from state.
	// Place the new order at the incremented size, unless the size overflows, in which case use the size of the new order.
	case types.Order_REPLACE_FLAGS_INC_SIZE:
		if orderExists {
			existingOrder, _ := k.GetOrderById(ctx, uidBytes)
			order.Base.Quantums = lib.Max(order.Base.Quantums, order.Base.Quantums+existingOrder.Base.Quantums)
			k.RemoveOrderById(ctx, uidBytes)
		}
	// Decremental size orders: If order exists, remove it from state.
	// Place the new order at the decremented size, unless the size is zero.
	case types.Order_REPLACE_FLAGS_DEC_SIZE:
		existingOrder, found := k.GetOrderById(ctx, uidBytes)
		if !found {
			return 0, 0, fmt.Errorf("dec size: order does not exist with key %s", uidBytes)
		}
		if existingOrder.Base.Quantums <= order.Base.Quantums {
			k.RemoveOrderById(ctx, uidBytes)
			return 0, 0, fmt.Errorf("dec size: size decremented to zero")
		}
		order.Base.Quantums = existingOrder.Base.Quantums - order.Base.Quantums
		k.RemoveOrderById(ctx, uidBytes)
	default:
		panic(fmt.Errorf("invalid place flags: %d", placeFlags))
	}

	// Stop orders: Put it in state and let it trigger later. Return.
	if order.Base.IsStop() {
		err := k.AddOrderToState(ctx, order)
		if err != nil {
			return 0, 0, err
		}
		return 0, order.Base.Quantums, nil
	}

	// All other orders are live orders.
	return k.ProcessLiveOrder(ctx, order)
}

func (k Keeper) ProcessLiveOrder(ctx sdk.Context, order types.XOrder) (
	sizeSum uint64, // filled size
	sizeRem uint64, // remaining size
	err error,
) {
	// A live order is either:
	// - not a stop order
	// - a stop order that has been triggered
	sizeSum = 0
	sizeRem = order.Base.Quantums

	// Start matching the order against the resting orders.
	// Iterate only from the best possible price to the price of the order.
	orderStore := k.GetXOrderStore(ctx)
	isBidBook := order.Base.Side() != types.Order_SIDE_BUY // isBidBook is true for sell orders.
	itStartKey := types.ToRestingKey(
		order.Uid.Iid.ClobId,
		isBidBook,
		getStartSubticks(isBidBook),
		0,
	)
	itEndKey := types.ToRestingKey(
		order.Uid.Iid.ClobId,
		isBidBook,
		order.Base.Subticks,
		math.MaxUint64,
	)
	it := k.GetXRestingStore(ctx).Iterator(itStartKey, itEndKey)
	defer it.Close()
	for ; it.Valid() && sizeRem > 0; it.Next() {
		// Get resting order.
		makerUidBytes := it.Value()
		makerOrder, found := k.GetOrderById(ctx, makerUidBytes)
		if !found {
			panic(fmt.Errorf("maker order not found during iteration"))
		}

		// Self-Trade Prevention.
		if makerOrder.Uid.Sid == order.Uid.Sid {
			stpType := order.Base.GetStp()
			if stpType == types.Order_STP_EXPIRE_MAKER {
				k.RemoveOrderById(ctx, makerUidBytes)
				continue
			} else if stpType == types.Order_STP_EXPIRE_TAKER {
				return sizeSum, 0, fmt.Errorf("stp is expire taker")
			} else if stpType == types.Order_STP_EXPIRE_BOTH {
				k.RemoveOrderById(ctx, makerUidBytes)
				return sizeSum, 0, fmt.Errorf("stp is expire both")
			}
			panic(fmt.Errorf("invalid stp type: %d", stpType))
		}

		// Return early if matches and post-only.
		if order.Base.GetTif() == types.Order_TIME_IN_FORCE_POST_ONLY {
			return sizeSum, 0, fmt.Errorf("order is post-only and matches resting order")
		}

		// Get the fill size.
		fillSize := makerOrder.Base.Quantums
		if fillSize > sizeRem {
			fillSize = sizeRem
		}

		// Attempt to update subaccounts.
		updates, err := k.GetSubaccountUpdatesForMatch(ctx, order, makerOrder, fillSize)
		if err != nil {
			return sizeSum, 0, err
		}
		_, results, err := k.subaccountsKeeper.UpdateSubaccounts(ctx, updates, satypes.Match)
		if err != nil {
			return sizeSum, 0, err
		}
		takerResult := results[0]
		makerResult := results[1]
		if !takerResult.IsSuccess() {
			return sizeSum, 0, nil // TODO: should an error be returned?
		}
		if !makerResult.IsSuccess() {
			k.RemoveOrderById(ctx, makerUidBytes)
			continue
		}

		// Update resting order.
		// If order is fully-filled, remove it from state.
		// If order is partially-filled, update it in state.
		makerOrder.Base.Quantums -= fillSize
		if makerOrder.Base.Quantums == 0 {
			k.RemoveOrderById(ctx, makerUidBytes)
		} else {
			orderStore.Set(makerOrder.Uid.ToBytes(), k.cdc.MustMarshal(&makerOrder))
		}

		// Cancel Maker OCO Order.
		if makerOrder.Base.HasOcoClientId() {
			ocoOrderId := makerOrder.Uid // Copy by value.
			ocoOrderId.Iid.ClientId = makerOrder.Base.MustGetOcoClientId()
			if ocoOrderId == makerOrder.Uid {
				panic(fmt.Errorf("oco order id is the same as the maker order id"))
			}
			k.RemoveOrderById(ctx, ocoOrderId.ToBytes())
		}

		// Cancel Taker OCO Order. Optimization: Only attempt to cancel on the first fill.
		if sizeSum == 0 && order.Base.HasOcoClientId() {
			ocoOrderId := order.Uid // Copy by value.
			ocoOrderId.Iid.ClientId = order.Base.MustGetOcoClientId()
			if ocoOrderId == order.Uid {
				panic(fmt.Errorf("oco order id is the same as the taker order id"))
			}
			k.RemoveOrderById(ctx, ocoOrderId.ToBytes())
		}

		// Update counters.
		sizeRem -= fillSize
		sizeSum += fillSize
	}

	// If the order is IOC, there is no remaining size.
	if order.Base.GetTif() == types.Order_TIME_IN_FORCE_IOC {
		sizeRem = 0
	}

	// If there is remaining size, attempt to place the order on the book.
	if sizeRem > 0 {
		order.Base.Quantums = sizeRem
		err := k.AddOrderToState(ctx, order)
		if err != nil {
			return sizeSum, 0, err
		}
	}

	return sizeSum, sizeRem, nil
}

func (k Keeper) RemoveOrderById(
	ctx sdk.Context,
	uidBytes []byte,
) (found bool) {
	// Remove order by id.
	orderStore := k.GetXOrderStore(ctx)
	orderBytes := orderStore.Get(uidBytes)
	if len(orderBytes) == 0 {
		return false
	}
	var order types.XOrder
	k.cdc.MustUnmarshal(orderBytes, &order)
	orderStore.Delete(uidBytes)

	// Precompute value which is used multiple times.
	priority := order.GetPriority()

	// Remove from Resting store.
	restingStore := k.GetXRestingStore(ctx)
	restingKey := order.ToRestingKey(priority)
	if restingStore.Has(restingKey) {
		restingStore.Delete(restingKey)
	}

	// Remove from Stop store if triggerable.
	if order.Base.IsStop() {
		stopStore := k.GetXStopStore(ctx)
		stopKey := order.ToStopKey(priority)
		if stopStore.Has(stopKey) {
			stopStore.Delete(stopKey)
		}
	}

	// Remove expiry.
	if order.Base.HasGoodTilTime() {
		expiryStore := k.GetXExpiryStore(ctx)
		expiryKey := order.ToExpiryKey(priority)
		if !expiryStore.Has(expiryKey) {
			panic(fmt.Errorf("expiry does not exist with key %s", expiryKey))
		}
		expiryStore.Delete(expiryKey)
	}

	return true
}

func (k Keeper) AddOrderToState(
	ctx sdk.Context,
	order types.XOrder,
) error {
	var store prefix.Store
	var key []byte
	// Precompute these values which may be used multiple times.
	priority := order.GetPriority()
	uidBytes := order.Uid.ToBytes()

	// Store order by id.
	store = k.GetXOrderStore(ctx)
	key = uidBytes
	if store.Has(key) {
		return fmt.Errorf("order already exists with key %s", key)
	}
	store.Set(key, k.cdc.MustMarshal(&order))

	// Store resting or stop.
	if order.Base.IsStop() {
		store = k.GetXStopStore(ctx)
		key = order.ToStopKey(priority)
		if store.Has(key) {
			return fmt.Errorf("order already exists with key %s", key)
		}
		store.Set(key, uidBytes)
	} else {
		store = k.GetXRestingStore(ctx)
		key = order.ToRestingKey(priority)
		if store.Has(key) {
			return fmt.Errorf("order already exists with key %s", key)
		}
		store.Set(key, uidBytes)
	}

	// Store expiry.
	if order.Base.HasGoodTilTime() {
		store = k.GetXExpiryStore(ctx)
		key = order.ToExpiryKey(priority)
		if store.Has(key) {
			return fmt.Errorf("order already exists with key %s", key)
		}
		store.Set(key, uidBytes)
	}

	return nil
}

func (k Keeper) ExpireXOrders(ctx sdk.Context) {
	// Iterate over the expiry store til the next second, exclusive.
	store := k.GetXExpiryStore(ctx)
	iterEnd := lib.Uint32ToBytes(uint32(ctx.BlockTime().Unix() + 1))
	it := store.Iterator(nil, iterEnd)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		k.RemoveOrderById(ctx, it.Value())
	}
}

func (k Keeper) TriggerXOrders(ctx sdk.Context) error {
	// Iterate over the stop store for each clob pair.
	store := k.GetXStopStore(ctx)
	for _, clobPair := range k.GetAllClobPairs(ctx) {
		// Get the last traded prices in the block.
		minPrice, maxPrice, found := k.GetTradePricesForPerpetual(ctx, clobPair.MustGetPerpetualId())
		if !found {
			continue
		}

		// Iterate over the traded prices upwards.
		itaEnd := types.ToStopKey(clobPair.Id, false, uint64(maxPrice)+1, 0)
		ita := store.Iterator(nil, itaEnd)
		defer ita.Close()
		for ; ita.Valid(); ita.Next() {
			_, _, err := k.TriggerOrder(ctx, ita.Value())
			if err != nil {
				return err
			}
		}

		// Iterate over the traded prices downwards.
		itbEnd := types.ToStopKey(clobPair.Id, true, uint64(minPrice)-1, 0)
		itb := store.Iterator(nil, itbEnd)
		defer itb.Close()
		for ; itb.Valid(); itb.Next() {
			_, _, err := k.TriggerOrder(ctx, itb.Value())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Stateful Helper Functions

func (k Keeper) GetSubaccountIdFromSID(
	ctx sdk.Context,
	sid uint64,
) (
	satypes.SubaccountId,
	error,
) {
	accountNumber, subaccountNumber := types.SplitSID(sid)
	address, err := k.accountKeeper.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		return satypes.SubaccountId{}, err
	}
	return satypes.SubaccountId{
		Owner:  address.String(),
		Number: subaccountNumber,
	}, nil
}

func (k Keeper) GetSubaccountUpdatesForMatch(
	ctx sdk.Context,
	takerOrder types.XOrder,
	makerOrder types.XOrder,
	fillAmount uint64,
) (
	[]satypes.Update,
	error,
) {
	clobPair, ok := k.GetClobPair(ctx, types.ClobPairId(takerOrder.Uid.Iid.ClobId))
	if !ok {
		return nil, fmt.Errorf("clob pair not found")
	}
	perpetualId := clobPair.MustGetPerpetualId()

	// Get the subaccount IDs.
	makerSubaccountId, err := k.GetSubaccountIdFromSID(ctx, makerOrder.Uid.Sid)
	if err != nil {
		return nil, err
	}
	takerSubaccountId, err := k.GetSubaccountIdFromSID(ctx, takerOrder.Uid.Sid)
	if err != nil {
		return nil, err
	}

	// Get the fill quote quantums.
	fillQuoteQuantums := types.FillAmountToQuoteQuantums(
		types.Subticks(makerOrder.Base.Subticks),
		satypes.BaseQuantums(fillAmount),
		clobPair.QuantumConversionExponent,
	)

	// Taker fees and maker fees/rebates are rounded towards positive infinity.
	makerFeePpm := k.feeTiersKeeper.GetPerpetualFeePpm(ctx, makerSubaccountId.Owner, false)
	takerFeePpm := k.feeTiersKeeper.GetPerpetualFeePpm(ctx, takerSubaccountId.Owner, true)
	takerFee := lib.BigMulPpm(fillQuoteQuantums, lib.BigI(takerFeePpm), true)
	makerFee := lib.BigMulPpm(fillQuoteQuantums, lib.BigI(makerFeePpm), true)

	// Determine the balance deltas.
	takerQuoteDelta := new(big.Int).Set(fillQuoteQuantums)
	makerQuoteDelta := new(big.Int).Set(fillQuoteQuantums)
	takerPerpDelta := lib.BigU(fillAmount)
	makerPerpDelta := lib.BigU(fillAmount)
	if takerOrder.Base.Side() == types.Order_SIDE_BUY {
		takerQuoteDelta.Neg(takerQuoteDelta)
		makerPerpDelta.Neg(makerPerpDelta)
	} else {
		makerQuoteDelta.Neg(makerQuoteDelta)
		takerPerpDelta.Neg(takerPerpDelta)
	}
	takerQuoteDelta.Sub(takerQuoteDelta, takerFee)
	makerQuoteDelta.Sub(makerQuoteDelta, makerFee)

	// Create the subaccount update.
	updates := []satypes.Update{
		{
			SubaccountId: takerSubaccountId,
			AssetUpdates: []satypes.AssetUpdate{{
				AssetId:          assettypes.AssetUsdc.Id,
				BigQuantumsDelta: takerQuoteDelta,
			}},
			PerpetualUpdates: []satypes.PerpetualUpdate{{
				PerpetualId:      perpetualId,
				BigQuantumsDelta: takerPerpDelta,
			}},
		},
		{
			SubaccountId: makerSubaccountId,
			AssetUpdates: []satypes.AssetUpdate{{
				AssetId:          assettypes.AssetUsdc.Id,
				BigQuantumsDelta: makerQuoteDelta,
			}},
			PerpetualUpdates: []satypes.PerpetualUpdate{{
				PerpetualId:      perpetualId,
				BigQuantumsDelta: makerPerpDelta,
			}},
		},
	}

	return updates, nil
}

// Helper functions

func getStartSubticks(isBidBook bool) uint64 {
	if isBidBook {
		return math.MaxUint64
	}
	return 0
}
