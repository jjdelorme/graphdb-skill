const assert = require('node:assert');
const { test, describe } = require('node:test');
const SqlAdapter = require('../adapters/SqlAdapter');

describe('SqlAdapter', () => {
  const adapter = new SqlAdapter();

  const sourceCode = `
CREATE PROCEDURE UpdateInventory
    @ProductID INT,
    @QuantityChange INT
AS
BEGIN
    -- Check if product exists
    IF NOT EXISTS(SELECT 1 FROM Products WHERE ProductID = @ProductID)
    BEGIN
        THROW 50000, 'Product does not exist.', 1;
    END;

    UPDATE p
    SET p.Quantity = p.Quantity + @QuantityChange
    FROM Products p
    WHERE p.ProductID = @ProductID;

    EXEC dbo.NotifyManager @ProductID;
END;

CREATE TRIGGER trg_InventoryUpdate
ON InventoryLog
AFTER INSERT
AS
BEGIN
    PRINT 'Inventory Updated';
END;
`;

  test('should parse source code', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    assert.ok(tree);
  });

  test('should scan definitions (Procedures)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const proc = definitions.find(d => d.name === 'UpdateInventory');
    assert.ok(proc, 'Should find UpdateInventory procedure');
    assert.strictEqual(proc.type, 'Function');
  });

  test('should scan definitions (Triggers)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const trigger = definitions.find(d => d.name === 'trg_InventoryUpdate');
    assert.ok(trigger, 'Should find trg_InventoryUpdate trigger');
    assert.strictEqual(trigger.type, 'Trigger');
    assert.strictEqual(trigger.watches, 'InventoryLog');
  });

  test('should scan references (Tables)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const refs = adapter.scanReferences(tree, []);

    const productRef = refs.find(r => r.target === 'Products');
    assert.ok(productRef, 'Should find reference to Products table');
    assert.strictEqual(productRef.type, 'Usage');
    assert.strictEqual(productRef.source, 'UpdateInventory');
  });

  test('should scan references (Calls)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const refs = adapter.scanReferences(tree, []);

    const callRef = refs.find(r => r.target === 'NotifyManager');
    assert.ok(callRef, 'Should find call to NotifyManager');
    assert.strictEqual(callRef.type, 'Call');
    assert.strictEqual(callRef.source, 'UpdateInventory');
  });
});
