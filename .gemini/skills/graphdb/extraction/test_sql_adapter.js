const SqlAdapter = require('./adapters/SqlAdapter');

const adapter = new SqlAdapter();

const sourceCode = `
CREATE PROCEDURE UpdateInventory
    @ProductID INT,
    @QuantityChange INT
AS
BEGIN
    BEGIN TRY
        BEGIN TRANSACTION;

        -- Check if product exists (Table Usage: Products)
        IF NOT EXISTS(SELECT 1 FROM Products WHERE ProductID = @ProductID)
        BEGIN
            THROW 50000, 'Product does not exist.', 1;
        END;

        -- Complex Update with Join (Table Usage: Products, Suppliers)
        UPDATE p
        SET p.Quantity = p.Quantity + @QuantityChange,
            p.LastUpdated = GETDATE()
        FROM Products p
        INNER JOIN Suppliers s ON p.SupplierID = s.ID
        WHERE p.ProductID = @ProductID AND s.Active = 1;

        -- Log (Table Usage: InventoryLog)
        INSERT INTO InventoryLog(ProductID, QuantityChange, ChangeDate)
        VALUES (@ProductID, @QuantityChange, GETDATE());

        -- Call another Proc (Call: NotifyManager)
        DECLARE @NewQty INT = (SELECT Quantity FROM Products WHERE ProductID = @ProductID);
        IF @NewQty < 10
        BEGIN
            EXEC dbo.NotifyManager @ProductID, 'Low Stock';
        END

        COMMIT TRANSACTION;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        -- Log Error (Call: LogError)
        EXEC LogError;
    END CATCH;
END;
`;

// Simulate "Parse" (virtual tree)
const tree = adapter.parse(sourceCode);

// Pass 1: Definitions
console.log("--- Definitions ---");
const defs = adapter.scanDefinitions(tree);
console.log(JSON.stringify(defs, null, 2));

// Pass 2: References
console.log("\n--- References ---");
// Mock globals
const globals = new Set();
const refs = adapter.scanReferences(tree, globals);
console.log(JSON.stringify(refs, null, 2));
