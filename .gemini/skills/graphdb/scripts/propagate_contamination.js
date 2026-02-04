const neo4jService = require('./Neo4jService');

async function main() {
    const session = neo4jService.getSession();

    try {
        console.log('Linking functions to classes based on label (ClassName::MethodName)...');
        // This creates MEMBER_OF relationships
        await session.run(`
            MATCH (f:Function)
            WHERE f.label CONTAINS "::"
            WITH f, split(f.label, "::") as parts
            // Get the last part as method name, and everything before as class/namespace
            WITH f, reduce(s = "", x in parts[0..-1] | s + (CASE WHEN s = "" THEN "" ELSE "::" END) + x) as className
            MATCH (c:Class {label: className})
            MERGE (f)-[:MEMBER_OF]->(c)
        `);

        console.log('Resetting ui_contaminated flags...');
        await session.run('MATCH (f:Function) SET f.ui_contaminated = false');
        await session.run('MATCH (c:Class) SET c.ui_contaminated = false');

        console.log('Marking classes inheriting from MFC base classes...');
        // Default MFC base classes (Microsoft Foundation Classes)
        // This list can be customized for other frameworks (e.g., Qt, WinForms)
        const mfcBases = [
            'CDialog', 'CDialogEx', 'CWnd', 'CWinApp', 'CView', 'CDocument', 
            'CFrameWnd', 'CStatic', 'CEdit', 'CButton', 'CListBox', 'CComboBox', 
            'CPropertyPage', 'CPropertySheet', 'CToolBar', 'CStatusBar', 'CMenu',
            'CWinThread', 'CScrollView', 'CFormView', 'CListView', 'CTreeView',
            'CControlBar', 'CDialogBar', 'CReBar', 'CSplitterWnd', 'CDockablePane'
        ];

        await session.run(`
            MATCH (base:Class)
            WHERE base.label IN $mfcBases
            MATCH (c:Class)-[:INHERITS_FROM*0..5]->(base)
            SET c.ui_contaminated = true
        `, { mfcBases });

        console.log('Marking functions in contaminated classes...');
        await session.run(`
            MATCH (f:Function)-[:MEMBER_OF]->(c:Class)
            WHERE c.ui_contaminated = true
            SET f.ui_contaminated = true
        `);

        console.log('Marking functions directly calling MFC APIs...');
        await session.run(`
            MATCH (f:Function)-[:CALLS_MFC]->()
            SET f.ui_contaminated = true
        `);

        console.log('Propagating contamination UP the call graph (Transitive)...');
        // We iterate to propagate. Usually 5-10 iterations cover most of it.
        let changed = true;
        let iteration = 0;
        while (changed && iteration < 20) {
            iteration++;
            const result = await session.run(`
                MATCH (f:Function {ui_contaminated: false})-[:CALLS]->(target:Function {ui_contaminated: true})
                WITH f, count(target) as targets
                SET f.ui_contaminated = true
                RETURN count(f) as updated
            `);
            const updated = neo4jService.toNum(result.records[0].get('updated'));
            console.log(`  Iteration ${iteration}: updated ${updated} functions`);
            changed = updated > 0;
        }

        console.log('Marking pure business logic...');
        // Pure business logic = not contaminated AND doesn't access many globals (maybe?)
        // For now, let's just use the inverse of contaminated.
        await session.run(`
            MATCH (f:Function)
            SET f.pure_business_logic = NOT f.ui_contaminated
        `);

        console.log('Calculating contamination stats...');
        const stats = await session.run(`
            MATCH (f:Function)
            RETURN 
                count(f) as total,
                sum(CASE WHEN f.ui_contaminated THEN 1 ELSE 0 END) as contaminated,
                sum(CASE WHEN f.pure_business_logic THEN 1 ELSE 0 END) as pure
        `);
        const rec = stats.records[0];
        console.log(`Total Functions: ${neo4jService.toNum(rec.get('total'))}`);
        console.log(`Contaminated: ${neo4jService.toNum(rec.get('contaminated'))}`);
        console.log(`Pure: ${neo4jService.toNum(rec.get('pure'))}`);

    } catch (error) {
        console.error('Error:', error);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

main().catch(console.error);