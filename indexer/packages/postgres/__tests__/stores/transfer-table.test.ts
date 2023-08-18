import {
  Ordering,
  SubaccountAssetNetTransferMap,
  TransferColumns,
  TransferCreateObject,
  TransferFromDatabase,
} from '../../src/types';
import * as TransferTable from '../../src/stores/transfer-table';
import { AssetTransferMap } from '../../src/stores/transfer-table';
import * as SubaccountTable from '../../src/stores/subaccount-table';
import { clearData, migrate, teardown } from '../../src/helpers/db-helpers';
import { seedData } from '../helpers/mock-generators';
import {
  createdDateTime,
  createdHeight,
  defaultAsset,
  defaultAsset2,
  defaultSubaccount3,
  defaultSubaccountId,
  defaultSubaccountId2,
  defaultSubaccountId3,
  defaultTendermintEventId,
  defaultTendermintEventId2,
  defaultTransfer,
  defaultTransfer2,
  defaultTransfer3,
} from '../helpers/constants';
import Big from 'big.js';

describe('Transfer store', () => {
  beforeEach(async () => {
    await seedData();
  });

  beforeAll(async () => {
    await migrate();
  });

  afterEach(async () => {
    await clearData();
  });

  afterAll(async () => {
    await teardown();
  });

  it('Successfully creates a Transfer', async () => {
    await TransferTable.create(defaultTransfer);
  });

  it('Successfully creates multiple transfers', async () => {
    const transfer2: TransferCreateObject = {
      senderSubaccountId: defaultSubaccountId2,
      recipientSubaccountId: defaultSubaccountId,
      assetId: defaultAsset2.id,
      size: '5',
      eventId: defaultTendermintEventId2,
      transactionHash: '', // TODO: Add a real transaction Hash
      createdAt: createdDateTime.toISO(),
      createdAtHeight: createdHeight,
    };
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(transfer2),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAll({}, [], {
      orderBy: [[TransferColumns.id, Ordering.ASC]],
    });

    expect(transfers.length).toEqual(2);
    expect(transfers[0]).toEqual(expect.objectContaining(transfer2));
    expect(transfers[1]).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully finds all Transfers', async () => {
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create({
        ...defaultTransfer,
        eventId: defaultTendermintEventId2,
      }),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAll({}, [], {
      orderBy: [[TransferColumns.id, Ordering.ASC]],
    });

    expect(transfers.length).toEqual(2);
    expect(transfers[0]).toEqual(expect.objectContaining({
      ...defaultTransfer,
      eventId: defaultTendermintEventId2,
    }));
    expect(transfers[1]).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully finds all transfers to and from subaccount', async () => {
    const transfer2: TransferCreateObject = {
      senderSubaccountId: defaultSubaccountId2,
      recipientSubaccountId: defaultSubaccountId,
      assetId: defaultAsset2.id,
      size: '5',
      eventId: defaultTendermintEventId2,
      transactionHash: '', // TODO: Add a real transaction Hash
      createdAt: createdDateTime.toISO(),
      createdAtHeight: createdHeight,
    };
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(transfer2),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAllToOrFromSubaccountId(
      { subaccountId: [defaultSubaccountId] },
      [], {
        orderBy: [[TransferColumns.id, Ordering.ASC]],
      });

    expect(transfers.length).toEqual(2);
    expect(transfers[0]).toEqual(expect.objectContaining(transfer2));
    expect(transfers[1]).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully finds all transfers to and from subaccount w/ event id', async () => {
    const transfer2: TransferCreateObject = {
      senderSubaccountId: defaultSubaccountId2,
      recipientSubaccountId: defaultSubaccountId,
      assetId: defaultAsset2.id,
      size: '5',
      eventId: defaultTendermintEventId2,
      transactionHash: '', // TODO: Add a real transaction Hash
      createdAt: createdDateTime.toISO(),
      createdAtHeight: createdHeight,
    };
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(transfer2),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAllToOrFromSubaccountId(
      {
        subaccountId: [defaultSubaccountId],
        eventId: [defaultTendermintEventId],
      },
      [], {
        orderBy: [[TransferColumns.id, Ordering.ASC]],
      });

    expect(transfers.length).toEqual(1);
    expect(transfers[0]).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully finds Transfer with eventId', async () => {
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create({
        ...defaultTransfer,
        eventId: defaultTendermintEventId2,
      }),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAll(
      {
        eventId: [defaultTendermintEventId2],
      },
      [],
      { readReplica: true },
    );

    expect(transfers.length).toEqual(1);
    expect(transfers[0]).toEqual(expect.objectContaining({
      ...defaultTransfer,
      eventId: defaultTendermintEventId2,
    }));
  });

  it('Successfully finds all Transfers before or at the height', async () => {
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create({
        ...defaultTransfer,
        eventId: defaultTendermintEventId2,
        createdAtHeight: '5',
      }),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAll(
      { createdBeforeOrAtHeight: defaultTransfer.createdAtHeight }, [], {
        orderBy: [[TransferColumns.id, Ordering.ASC]],
      });

    expect(transfers.length).toEqual(1);
    expect(transfers[0]).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully finds all Transfers created after height', async () => {
    const transfer2: TransferCreateObject = {
      ...defaultTransfer,
      eventId: defaultTendermintEventId2,
      createdAtHeight: '5',
    };
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(transfer2),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAll(
      { createdAfterHeight: '4' }, [], {
        orderBy: [[TransferColumns.id, Ordering.ASC]],
      });

    expect(transfers.length).toEqual(1);
    expect(transfers[0]).toEqual(expect.objectContaining(transfer2));
  });

  it('Successfully finds all transfers to and from subaccount created before time', async () => {
    const transfer2: TransferCreateObject = {
      senderSubaccountId: defaultSubaccountId2,
      recipientSubaccountId: defaultSubaccountId,
      assetId: defaultAsset2.id,
      size: '5',
      eventId: defaultTendermintEventId2,
      transactionHash: '', // TODO: Add a real transaction Hash
      createdAt: '1982-05-25T00:00:00.000Z',
      createdAtHeight: '100',
    };
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(transfer2),
    ]);

    const transfers: TransferFromDatabase[] = await TransferTable.findAllToOrFromSubaccountId(
      {
        subaccountId: [defaultSubaccountId],
        createdBeforeOrAt: '2000-05-25T00:00:00.000Z',
      },
      [], {
        orderBy: [[TransferColumns.id, Ordering.ASC]],
      });

    expect(transfers.length).toEqual(1);
    expect(transfers[0]).toEqual(expect.objectContaining(transfer2));
  });

  it('Successfully finds a Transfer', async () => {
    await TransferTable.create(defaultTransfer);

    const transfer: TransferFromDatabase | undefined = await TransferTable.findById(
      TransferTable.uuid(
        defaultTransfer.senderSubaccountId,
        defaultTransfer.recipientSubaccountId,
        defaultTransfer.eventId,
        defaultTransfer.assetId),
    );

    expect(transfer).toEqual(expect.objectContaining(defaultTransfer));
  });

  it('Successfully gets total transfers per subaccount', async () => {
    await SubaccountTable.create(defaultSubaccount3);
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create(defaultTransfer2),
      TransferTable.create(defaultTransfer3),
    ]);

    const transferMap: SubaccountAssetNetTransferMap = await
    TransferTable.getNetTransfersPerSubaccount(createdHeight);

    expect(transferMap).toEqual(expect.objectContaining({
      [defaultSubaccountId]: {
        [defaultAsset.id]: '-10',
      },
      [defaultSubaccountId2]: {
        [defaultAsset.id]: '15',
        [defaultAsset2.id]: '5',
      },
      [defaultSubaccountId3]: {
        [defaultAsset.id]: '-5',
        [defaultAsset2.id]: '-5',
      },
    }));
  });

  it('Successfully gets total transfers per subaccount with duplicate transfer amounts', async () => {
    await SubaccountTable.create(defaultSubaccount3);
    await Promise.all([
      TransferTable.create(defaultTransfer),
      TransferTable.create({
        ...defaultTransfer,
        eventId: defaultTendermintEventId2,
      }),
      TransferTable.create(defaultTransfer2),
      TransferTable.create(defaultTransfer3),
    ]);

    const transferMap: SubaccountAssetNetTransferMap = await
    TransferTable.getNetTransfersPerSubaccount(createdHeight);

    expect(transferMap).toEqual(expect.objectContaining({
      [defaultSubaccountId]: {
        [defaultAsset.id]: '-20',
      },
      [defaultSubaccountId2]: {
        [defaultAsset.id]: '25',
        [defaultAsset2.id]: '5',
      },
      [defaultSubaccountId3]: {
        [defaultAsset.id]: '-5',
        [defaultAsset2.id]: '-5',
      },
    }));
  });

  it('Successfully gets total decimal value transfers per subaccount and respects createdBeforeOrAtHeight', async () => {
    await SubaccountTable.create(defaultSubaccount3);
    await Promise.all([
      TransferTable.create({
        ...defaultTransfer,
        size: '10.5',
      }),
      TransferTable.create({
        ...defaultTransfer2,
        size: '5.2',
      }),
      // this transfer is ignored because createdAtHeight > createdBeforeOrAtHeight
      TransferTable.create({
        ...defaultTransfer3,
        size: '5.3',
        createdAtHeight: '5',
      }),
    ]);

    const transferMap: SubaccountAssetNetTransferMap = await
    TransferTable.getNetTransfersPerSubaccount(createdHeight);

    expect(transferMap).toEqual(expect.objectContaining({
      [defaultSubaccountId]: {
        [defaultAsset.id]: '-10.5',
      },
      [defaultSubaccountId2]: {
        [defaultAsset.id]: '15.7',
      },
      [defaultSubaccountId3]: {
        [defaultAsset.id]: '-5.2',
      },
    }));
  });

  it('Successfully gets net transfers between block heights for a subaccount', async () => {
    await SubaccountTable.create(defaultSubaccount3);
    await Promise.all([
      TransferTable.create({
        ...defaultTransfer,
        createdAtHeight: '1',
      }),
      TransferTable.create({
        ...defaultTransfer,
        size: '10.5',
        createdAtHeight: '2',
        eventId: defaultTendermintEventId2,
      }),
      TransferTable.create({
        ...defaultTransfer2,
        size: '5.2',
        createdAtHeight: '3',
      }),
      TransferTable.create({
        ...defaultTransfer3,
        size: '5.3',
        createdAtHeight: '5',
      }),
    ]);

    const [
      transferMap,
      transferMap2,
      transferMap21,
      transferMap3,
    ]: [
      AssetTransferMap,
      AssetTransferMap,
      AssetTransferMap,
      AssetTransferMap,
    ] = await Promise.all([
      TransferTable.getNetTransfersBetweenBlockHeightsForSubaccount(
        defaultSubaccountId,
        '1',
        '3',
      ),
      TransferTable.getNetTransfersBetweenBlockHeightsForSubaccount(
        defaultSubaccountId2,
        '1',
        '3',
      ),
      TransferTable.getNetTransfersBetweenBlockHeightsForSubaccount(
        defaultSubaccountId2,
        '1',
        '5',
      ),
      TransferTable.getNetTransfersBetweenBlockHeightsForSubaccount(
        defaultSubaccountId3,
        '2',
        '5',
      ),
    ]);
    expect(transferMap).toEqual({
      [defaultAsset.id]: Big('-10.5'),
    });
    expect(transferMap2).toEqual({
      [defaultAsset.id]: Big('15.7'),
    });
    expect(transferMap21).toEqual({
      [defaultAsset.id]: Big('15.7'),
      [defaultAsset2.id]: Big('5.3'),
    });
    expect(transferMap3).toEqual({
      [defaultAsset.id]: Big('-5.2'),
      [defaultAsset2.id]: Big('-5.3'),
    });
  });
});
