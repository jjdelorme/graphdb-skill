const source = `
CREATE PROCEDURE UpdateInventory
    @ProductID INT
AS
`;

const procRegex = /create\s+(?:procedure|proc)\s+(?:\[?(\w+)\]?\.\[?(\w+)\]?|\[?(\w+)\]?)/gi;
let match = procRegex.exec(source);
console.log('Match:', match);

const tableRegex = /(?:from|join|update|insert\s+into)\s+(?:\[?(\w+)\]?\.\[?(\w+)\]?|\[?(\w+)\]?)/gi;
const tableSource = "FROM Products";
console.log('Table Match:', tableRegex.exec(tableSource));
