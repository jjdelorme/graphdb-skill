import { nodes, links, nodesMap, linksMap, state, visibilitySettings, seamState } from './state.js';
import { fetchOverview, fetchTraverse, fetchWhatIf, fetchSearch, fetchSeams, fetchSemanticTrace, fetchNeighbors } from './api.js';
import { updateGraph, renderGraph, getGraphComponents, zoomFit } from './graph.js';
import { showNodeDetails, resolveNodeName, isNodeVisible, isSemantic, isPhysical } from './ui.js';
import { CSS_CLASSES } from './config.js';

let contextNode = null;

export function initEventListeners() {
    const { svg, zoom, simulation } = getGraphComponents();

    // Zoom controls
    document.getElementById('zoom-in').addEventListener('click', () => {
        svg.transition().duration(300).call(zoom.scaleBy, 1.3);
    });

    document.getElementById('zoom-out').addEventListener('click', () => {
        svg.transition().duration(300).call(zoom.scaleBy, 0.7);
    });

    document.getElementById('reset-view').addEventListener('click', async () => {
        try {
            const data = await fetchOverview();
            if (data && data.nodes) {
                const fetchedNodes = data.nodes.map(n => ({
                    id: n.id || n.Id,
                    name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.id || n.Id,
                    properties: n.properties || n.Properties,
                    ...n
                }));

                nodesMap.clear();
                linksMap.clear();
                nodes.length = 0;
                links.length = 0;
                updateGraph(fetchedNodes, data.edges || []);
                
                // Clear selection
                state.lastSelectedNode = null;
                const panel = document.getElementById('impact-panel');
                if (panel) panel.style.display = 'none';

                // Reset zoom using zoomFit after simulation stabilizes
                const { simulation } = getGraphComponents();
                simulation.alpha(1).restart();
                simulation.on("end.fit", () => {
                    zoomFit(750);
                    simulation.on("end.fit", null);
                });
            }
        } catch (err) {
            console.error("Failed to fetch initial overview during reset:", err);
        }
    });

    document.getElementById('collapse-all').addEventListener('click', () => {
        // Find all nodes that were expanded and collapse them iteratively
        let didCollapse = false;
        nodes.forEach(n => {
            if (n._expanded) {
                collapseNode(n);
                n._expanded = false;
                didCollapse = true;
            }
        });
        
        if (didCollapse) {
            // Also refresh UI state if we had a selection
            if (state.lastSelectedNode) {
                showNodeDetails(state.lastSelectedNode);
            }
        }
    });

    // Layer toggles
    function updateLayerButtonState(btnId, isActive) {
        const btn = document.getElementById(btnId);
        if (!btn) return;
        const baseClasses = "h-9 px-3 text-xs font-medium rounded-lg flex items-center gap-2 transition-colors w-full justify-start";
        const activeClasses = "bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-600 shadow-sm text-slate-900 dark:text-slate-100 ring-1 ring-slate-200 dark:ring-slate-600";
        const inactiveClasses = "bg-transparent border border-transparent shadow-none text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 opacity-70";
        btn.className = `${baseClasses} ${isActive ? activeClasses : inactiveClasses}`;
    }

    // Initialize visual state
    updateLayerButtonState('btn-structural-layer', visibilitySettings.showHulls);
    updateLayerButtonState('btn-dual-lens-layer', visibilitySettings.showDualLens);
    updateLayerButtonState('btn-physical-layer', visibilitySettings.showPhysical);
    updateLayerButtonState('btn-semantic-layer', visibilitySettings.showSemantic);
    updateLayerButtonState('btn-test-layer', visibilitySettings.showTests);

    const btnStructural = document.getElementById('btn-structural-layer');
    if (btnStructural) {
        btnStructural.addEventListener('click', (e) => {
            visibilitySettings.showHulls = !visibilitySettings.showHulls;
            state.showHulls = visibilitySettings.showHulls;
            updateLayerButtonState('btn-structural-layer', visibilitySettings.showHulls);
            renderGraph();
        });
    }

    const btnDualLens = document.getElementById('btn-dual-lens-layer');
    if (btnDualLens) {
        btnDualLens.addEventListener('click', (e) => {
            visibilitySettings.showDualLens = !visibilitySettings.showDualLens;
            state.showDualLens = visibilitySettings.showDualLens;
            updateLayerButtonState('btn-dual-lens-layer', visibilitySettings.showDualLens);
            updateLegend();
            renderGraph();
        });
    }

    document.getElementById('btn-physical-layer').addEventListener('click', (e) => {
        visibilitySettings.showPhysical = !visibilitySettings.showPhysical;
        updateLayerButtonState('btn-physical-layer', visibilitySettings.showPhysical);
        if (visibilitySettings.showPhysical) {
            nodes.filter(n => isNodeVisible(n) && isSemantic(n)).forEach(n => fetchAndExpandNeighborhood(n, "IMPLEMENTS"));
        }
        renderGraph();
    });

    document.getElementById('btn-semantic-layer').addEventListener('click', (e) => {
        visibilitySettings.showSemantic = !visibilitySettings.showSemantic;
        updateLayerButtonState('btn-semantic-layer', visibilitySettings.showSemantic);
        if (visibilitySettings.showSemantic) {
            nodes.filter(n => isNodeVisible(n) && isPhysical(n)).forEach(n => fetchAndExpandNeighborhood(n, "IMPLEMENTS"));
        }
        renderGraph();
    });

    document.getElementById('btn-test-layer').addEventListener('click', (e) => {
        visibilitySettings.showTests = !visibilitySettings.showTests;
        updateLayerButtonState('btn-test-layer', visibilitySettings.showTests);
        renderGraph();
    });

    // Search
    document.getElementById('search-button').addEventListener('click', handleSearch);
    document.getElementById('search-input').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') handleSearch();
    });

    // Blast Radius Button (Footer)
    document.getElementById('blast-radius-button').addEventListener('click', () => {
        if (state.lastSelectedNode) {
            runSimulation(state.lastSelectedNode);
        } else {
            alert("Please select a node first by clicking on it.");
        }
    });

    // Seam buttons
    document.getElementById('pinch-points-button').addEventListener('click', async (e) => {
        seamState.showPinchPoints = !seamState.showPinchPoints;
        const btn = e.currentTarget;
        btn.classList.toggle('bg-primary/20', seamState.showPinchPoints);
        btn.classList.toggle('border-primary', seamState.showPinchPoints);
        
        if (seamState.showPinchPoints && seamState.pinchPoints.size === 0) {
            try {
                const data = await fetchSeams('seams');
                if (Array.isArray(data)) {
                    data.forEach(s => seamState.pinchPoints.add(s.seam || s.Seam));
                }
            } catch (err) { console.error(err); }
        }
        renderGraph();
    });

    document.getElementById('semantic-seams-button').addEventListener('click', async (e) => {
        seamState.showSemanticSeams = !seamState.showSemanticSeams;
        const btn = e.currentTarget;
        btn.classList.toggle('bg-primary/20', seamState.showSemanticSeams);
        btn.classList.toggle('border-primary', seamState.showSemanticSeams);
        
        if (seamState.showSemanticSeams && seamState.semanticSeams.length === 0) {
            try {
                const data = await fetchSeams('semantic-seams');
                if (Array.isArray(data)) {
                    seamState.semanticSeams = data;
                }
            } catch (err) { console.error(err); }
        }
        renderGraph();
    });

    // Context menu
    document.addEventListener('click', () => {
        const contextMenu = document.getElementById('context-menu');
        if (contextMenu) contextMenu.style.display = 'none';
    });

    document.getElementById('menu-simulate-extraction').addEventListener('click', () => {
        if (contextNode) runSimulation(contextNode);
    });

    document.getElementById('menu-unpin-node').addEventListener('click', () => {
        if (contextNode) {
            contextNode.fx = null;
            contextNode.fy = null;
            simulation.alpha(0.3).restart();
        }
    });

    initSidePanelButtons();
}

export function getHandlers() {
    return {
        handleNodeClick,
        handleNodeMouseOver,
        handleNodeMouseOut,
        handleNodeDoubleClick,
        handleNodeContextMenu
    };
}

async function handleNodeClick(event, d) {
    console.log("Node clicked:", d.id);
    state.lastSelectedNode = d;
    showNodeDetails(d);
}

function handleNodeMouseOver(event, d) {
    d3.select(this).select('circle')
        .transition().duration(200)
        .attr('r', 25)
        .attr('stroke-width', 4);
}

function handleNodeMouseOut(event, d) {
    d3.select(this).select('circle')
        .transition().duration(200)
        .attr('r', 20)
        .attr('stroke-width', 2);
}

async function handleNodeDoubleClick(event, d) {
    console.log("Node double-clicked:", d.id);
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }

    state.lastSelectedNode = d;
    showNodeDetails(d);

    const { svg, zoom } = getGraphComponents();
    const width = document.getElementById('graph-container').clientWidth;
    const height = document.getElementById('graph-container').clientHeight;

    // Pin the node if it has a position
    if (d.x !== undefined && d.y !== undefined) {
        d.fx = d.x;
        d.fy = d.y;
    }

    // Get current zoom transform to maintain or slightly widen the scale
    const currentTransform = d3.zoomTransform(svg.node());
    // Use the current scale, but if we're zoomed in very tight, zoom out a bit to see neighbors
    const scale = Math.min(currentTransform.k, 1.0); 
    
    const targetX = d.x !== undefined ? d.x : width / 2;
    const targetY = d.y !== undefined ? d.y : height / 2;

    const transform = d3.zoomIdentity
        .translate(width / 2, height / 2)
        .scale(scale)
        .translate(-targetX, -targetY);

    svg.transition()
        .duration(750)
        .call(zoom.transform, transform);

    if (d._expanded) {
        collapseNode(d);
        d._expanded = false;
    } else {
        await fetchAndExpandNeighborhood(d);
        d._expanded = true;
    }
    
    showNodeDetails(d);
}

function collapseNode(d) {
    console.log("Collapsing node:", d.id);
    const nodesToRemove = new Set();
    
    // Find neighbors of d
    const neighbors = new Set();
    links.forEach(l => {
        const sid = l.source.id || l.source;
        const tid = l.target.id || l.target;
        if (sid === d.id) neighbors.add(tid);
        if (tid === d.id) neighbors.add(sid);
    });

    neighbors.forEach(nid => {
        // Check if all links for nid are connected to d
        const isOnlyConnectedToD = links.every(l => {
            const sid = l.source.id || l.source;
            const tid = l.target.id || l.target;
            if (sid === nid || tid === nid) {
                return sid === d.id || tid === d.id;
            }
            return true;
        });

        if (isOnlyConnectedToD) {
            nodesToRemove.add(nid);
        }
    });

    if (nodesToRemove.size > 0) {
        // Remove nodes
        for (let i = nodes.length - 1; i >= 0; i--) {
            if (nodesToRemove.has(nodes[i].id)) {
                nodesMap.delete(nodes[i].id);
                nodes.splice(i, 1);
            }
        }
        // Remove links
        for (let i = links.length - 1; i >= 0; i--) {
            const sid = links[i].source.id || links[i].source;
            const tid = links[i].target.id || links[i].target;
            if (nodesToRemove.has(sid) || nodesToRemove.has(tid)) {
                const linkId = `${sid}-${tid}-${links[i].type}`;
                linksMap.delete(linkId);
                links.splice(i, 1);
            }
        }
        renderGraph();
    }
}

function handleNodeContextMenu(event, d) {
    event.preventDefault();
    contextNode = d;
    const contextMenu = document.getElementById('context-menu');
    contextMenu.style.display = 'block';
    contextMenu.style.left = `${event.pageX}px`;
    contextMenu.style.top = `${event.pageY}px`;
}

async function handleSearch() {
    const target = document.getElementById('search-input').value;
    if (!target) return;

    try {
        const data = await fetchSearch(target);
        
        if (data && Array.isArray(data)) {
            let fetchedNodes = data.map(item => {
                const node = item.node || item;
                return {
                    id: node.id || node.Id,
                    label: node.label || node.Label || 'Node',
                    name: (node.properties && node.properties.name) || (node.Properties && node.Properties.name) || node.name || node.id || node.Id,
                    properties: node.properties || node.Properties,
                    ...item
                };
            });
            
            nodesMap.clear();
            linksMap.clear();
            nodes.length = 0;
            links.length = 0;
            
            updateGraph(fetchedNodes, []);
            
            const { simulation } = getGraphComponents();
            simulation.alpha(1).restart();
            simulation.on("end.fit", () => {
                zoomFit(750);
                simulation.on("end.fit", null);
            });
        }
    } catch (err) {
        console.error("Failed to fetch data:", err);
    }
}

async function runSimulation(node) {
    if (!node) return;
    const target = node.id;
    
    showNodeDetails(node);
    
    const riskLabelEl = document.getElementById('risk-label');
    if (riskLabelEl) riskLabelEl.textContent = 'ANALYZING...';
    
    try {
        const data = await fetchWhatIf(target);
        
        // Update the local graph with any newly discovered nodes
        if (data.orphaned_nodes) updateGraph(data.orphaned_nodes, []);
        if (data.shared_state) updateGraph(data.shared_state, []);
        if (data.affected_nodes) updateGraph(data.affected_nodes, []);

        const impactedNodesCount = (data.orphaned_nodes?.length || 0) + (data.severed_edges?.length || 0);
        const statNodes = document.getElementById('stat-nodes');
        if (statNodes) statNodes.textContent = impactedNodesCount;
        
        const statDepth = document.getElementById('stat-depth');
        if (statDepth) statDepth.textContent = impactedNodesCount > 0 ? '1+' : '0';
        
        const vizContainer = document.getElementById('impact-visualizations') || document.createElement('div');
        vizContainer.id = 'impact-visualizations';
        const impactDetails = document.getElementById('impact-details');
        if (impactDetails && !document.getElementById('impact-visualizations')) {
            impactDetails.appendChild(vizContainer);
        }
        
        vizContainer.innerHTML = '<h4 class="text-[10px] font-bold uppercase tracking-widest text-slate-500 mt-4 mb-2">Affected Pathways</h4>';

        if (data && (data.severed_edges?.length > 0 || data.orphaned_nodes?.length > 0)) {
            (data.orphaned_nodes || []).forEach(orphanedNode => {
                const item = document.createElement('div');
                const displayName = resolveNodeName(orphanedNode.id);
                item.className = 'flex items-center justify-between p-2 rounded-lg bg-slate-100 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 mb-2 cursor-pointer hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors group';
                item.onclick = () => focusNode(orphanedNode.id);
                item.innerHTML = `
                    <div class="flex items-center gap-3">
                        <span class="material-symbols-outlined text-impact-high text-lg group-hover:scale-110 transition-transform">dangerous</span>
                        <div class="flex flex-col">
                            <span class="text-xs font-bold">${displayName}</span>
                            <span class="text-[10px] text-slate-500">Orphaned</span>
                        </div>
                    </div>
                `;
                vizContainer.appendChild(item);
            });

            (data.severed_edges || []).forEach(edge => {
                const item = document.createElement('div');
                const targetName = resolveNodeName(edge.targetId);
                const sourceName = resolveNodeName(edge.sourceId);
                item.className = 'flex items-center justify-between p-2 rounded-lg bg-slate-100 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 mb-2 cursor-pointer hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors group';
                item.onclick = () => focusNode(edge.targetId);
                item.innerHTML = `
                    <div class="flex items-center gap-3">
                        <span class="material-symbols-outlined text-impact-medium text-lg group-hover:scale-110 transition-transform">link_off</span>
                        <div class="flex flex-col">
                            <span class="text-xs font-bold">${targetName}</span>
                            <span class="text-[10px] text-slate-500">Severed from ${sourceName}</span>
                        </div>
                    </div>
                `;
                vizContainer.appendChild(item);
            });
            
            if (riskLabelEl) {
                if (impactedNodesCount > 10) riskLabelEl.textContent = 'CRITICAL';
                else if (impactedNodesCount > 0) riskLabelEl.textContent = 'HIGH';
                else riskLabelEl.textContent = 'STABLE';
            }
        } else {
            vizContainer.innerHTML += `<p class="text-xs text-slate-500 italic">No significant impact found.</p>`;
            if (riskLabelEl) riskLabelEl.textContent = 'STABLE';
        }
    } catch (err) {
        console.error("Failed to simulate extraction:", err);
        if (riskLabelEl) riskLabelEl.textContent = 'ERROR';
    }
}

async function focusNode(nodeId) {
    console.log("Focusing on node:", nodeId);
    let node = nodesMap.get(nodeId);
    
    if (!node) {
        console.log(`Node ${nodeId} not in graph, fetching...`);
        try {
            const data = await fetchTraverse(nodeId, 1, 'both');
            if (data && Array.isArray(data) && data.length > 0) {
                data.forEach(path => updateGraph(path.nodes || [], path.edges || []));
                node = nodesMap.get(nodeId);
            } else {
                const neighborData = await fetchNeighbors(nodeId);
                if (neighborData && neighborData.node) {
                    updateGraph([neighborData.node], []);
                    node = nodesMap.get(nodeId);
                }
            }
        } catch (err) {
            console.error("Failed to fetch node for focusing:", err);
        }
    }

    if (node) {
        await handleNodeDoubleClick(null, node);
    } else {
        alert(`Node ${nodeId} could not be found in the graph.`);
    }
}

async function fetchAndExpandNeighborhood(d, edgeTypes = "") {
    try {
        const data = await fetchTraverse(d.id, 1, 'both', edgeTypes);
        if (data && Array.isArray(data)) {
            let fetchedNodes = [];
            let fetchedLinks = [];
            data.forEach(path => {
                if (path.nodes) fetchedNodes.push(...path.nodes.map(n => ({
                    id: n.id || n.Id,
                    label: n.label || n.Label || 'Node',
                    name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.name || n.id || n.Id,
                    properties: n.properties || n.Properties
                })));
                if (path.edges) fetchedLinks.push(...path.edges.map(e => ({
                    source: e.sourceId || e.SourceID,
                    target: e.targetId || e.TargetID,
                    type: e.type || e.Type
                })));
            });
            updateGraph(fetchedNodes, fetchedLinks);
        }
    } catch (err) { console.error(err); }
}

function initSidePanelButtons() {
    const btnExpand = document.getElementById('btn-expand-node');
    if (btnExpand) {
        btnExpand.addEventListener('click', () => {
            if (state.lastSelectedNode) handleNodeDoubleClick(null, state.lastSelectedNode);
        });
    }

    const btnTrace = document.getElementById('btn-trace-intent');
    if (btnTrace) {
        btnTrace.addEventListener('click', async () => {
            if (!state.lastSelectedNode) return;
            try {
                const data = await fetchSemanticTrace(state.lastSelectedNode.id);
                if (data && Array.isArray(data)) {
                    let fetchedNodes = [];
                    let fetchedLinks = [];
                    data.forEach(path => {
                        if (path.nodes) fetchedNodes.push(...path.nodes.map(n => ({
                            id: n.id || n.Id,
                            label: n.label || n.Label || 'Node',
                            name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.name || n.id || n.Id,
                            properties: n.properties || n.Properties
                        })));
                        if (path.edges) fetchedLinks.push(...path.edges.map(e => ({
                            source: e.sourceId || e.SourceID,
                            target: e.targetId || e.TargetID,
                            type: e.type || e.Type
                        })));
                    });
                    updateGraph(fetchedNodes, fetchedLinks);
                }
            } catch (err) { console.error(err); }
        });
    }

    const btnSimulate = document.getElementById('btn-simulate-extraction');
    if (btnSimulate) {
        btnSimulate.addEventListener('click', () => {
            if (state.lastSelectedNode) runSimulation(state.lastSelectedNode);
        });
    }
}
