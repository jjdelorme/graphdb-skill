import { nodes, links, nodesMap, linksMap, state, seamState, visibilitySettings } from './state.js';
import { isNodeVisible, getColor, getCommunityColor, getNodeCommunityId, isSharedBoundary, isQuarantinedHub } from './ui.js';

let svg, g, gHulls, gLinks, gNodes, zoom, simulation;
let width, height;
let registeredHandlers = {};

const hullLine = d3.line()
    .x(d => d[0])
    .y(d => d[1])
    .curve(d3.curveCatmullRomClosed.alpha(0.5));

export function initGraph(handleNodeClick, handleNodeMouseOver, handleNodeMouseOut, handleNodeDoubleClick, handleNodeContextMenu) {
    registeredHandlers = {
        handleNodeClick,
        handleNodeMouseOver,
        handleNodeMouseOut,
        handleNodeDoubleClick,
        handleNodeContextMenu
    };

    width = document.getElementById('graph-container').clientWidth || 1200;
    height = document.getElementById('graph-container').clientHeight || 800;

    zoom = d3.zoom().on("zoom", function (event) {
        const transform = event.transform;
        g.attr("transform", transform);
        
        const shouldBeVisible = transform.k >= 0.6;
        if (shouldBeVisible !== state.labelsVisible) {
            state.labelsVisible = shouldBeVisible;
            g.selectAll('.node-label').style('opacity', shouldBeVisible ? 1 : 0);
        }
    });

    svg = d3.select('#graph-container')
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .style('cursor', 'grab')
        .call(zoom)
        .on("dblclick.zoom", null); 

    g = svg.append('g');
    // Feature F18: Add hulls-layer BEFORE links-layer so hulls sit behind edges and nodes
    gHulls = g.append('g').attr('class', 'hulls-layer');
    gLinks = g.append('g').attr('class', 'links-layer');
    gNodes = g.append('g').attr('class', 'nodes-layer');

    svg.append("defs").append("marker")
        .attr("id", "arrowhead")
        .attr("viewBox", "0 -5 10 10")
        .attr("refX", 25)
        .attr("refY", 0)
        .attr("orient", "auto")
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("xoverflow", "visible")
        .append("svg:path")
        .attr("d", "M 0,-5 L 10 ,0 L 0,5")
        .attr("fill", "#999")
        .style("stroke", "none");

    simulation = d3.forceSimulation()
        .force("link", d3.forceLink().id(d => d.id).distance(250))
        .force("charge", d3.forceManyBody().strength(-1000))
        .force("x", d3.forceX(width / 2).strength(0.05))
        .force("y", d3.forceY(height / 2).strength(0.05))
        .force("collision", d3.forceCollide().radius(45));

    simulation.on("tick", () => {
        updateHullsTick();

        gLinks.selectAll(".link")
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        gNodes.selectAll(".node-group")
            .attr("transform", d => `translate(${d.x},${d.y})`);
    });

    return { svg, g, zoom, simulation };
}

export function getGraphComponents() {
    return { svg, g, zoom, simulation };
}

function isSemanticSeam(nodeId) {
    if (!seamState.semanticSeams) return false;
    for (let i = 0; i < seamState.semanticSeams.length; i++) {
        if (seamState.semanticSeams[i].method_a === nodeId || seamState.semanticSeams[i].method_b === nodeId) return true;
    }
    return false;
}

// Compute convex hull data for a community (Feature F18)
function computeCommunityHullData(commId, commNodes) {
    if (!commNodes || commNodes.length === 0) return null;

    let sumX = 0, sumY = 0;
    commNodes.forEach(n => {
        sumX += n.x;
        sumY += n.y;
    });
    const cx = sumX / commNodes.length;
    const cy = sumY / commNodes.length;

    let pathStr = '';
    const pad = 30; // 30px radial expansion padding

    if (commNodes.length >= 3) {
        const pts = commNodes.map(n => [n.x, n.y]);
        let hull = d3.polygonHull(pts);

        if (!hull || hull.length < 3) {
            // Collinear or degenerate fallback: expand points radially
            const expandedPts = [];
            pts.forEach(([px, py]) => {
                expandedPts.push([px + pad, py]);
                expandedPts.push([px - pad, py]);
                expandedPts.push([px, py + pad]);
                expandedPts.push([px, py - pad]);
            });
            hull = d3.polygonHull(expandedPts);
        }

        if (hull && hull.length >= 3) {
            // Radial expansion from centroid
            const expandedHull = hull.map(([px, py]) => {
                const dx = px - cx;
                const dy = py - cy;
                const dist = Math.sqrt(dx * dx + dy * dy);
                if (dist < 0.001) {
                    return [px + pad, py];
                }
                return [
                    px + (dx / dist) * pad,
                    py + (dy / dist) * pad
                ];
            });
            pathStr = hullLine(expandedHull);
        }
    } else if (commNodes.length === 2) {
        // Double node fallback: smooth capsule / pill path
        const n1 = commNodes[0];
        const n2 = commNodes[1];
        const dx = n2.x - n1.x;
        const dy = n2.y - n1.y;
        const dist = Math.sqrt(dx * dx + dy * dy);
        const r = 35;

        if (dist < 0.001) {
            pathStr = `M ${n1.x - r},${n1.y} a ${r},${r} 0 1,0 ${2 * r},0 a ${r},${r} 0 1,0 ${-2 * r},0`;
        } else {
            const nx = -dy / dist;
            const ny = dx / dist;
            const tx = dx / dist;
            const ty = dy / dist;

            const capsulePoints = [
                [n1.x - tx * r, n1.y - ty * r],
                [n1.x + nx * r, n1.y + ny * r],
                [n2.x + nx * r, n2.y + ny * r],
                [n2.x + tx * r, n2.y + ty * r],
                [n2.x - nx * r, n2.y - ny * r],
                [n1.x - nx * r, n1.y - ny * r]
            ];
            pathStr = hullLine(capsulePoints);
        }
    } else if (commNodes.length === 1) {
        // Single node fallback: circle
        const n = commNodes[0];
        const r = 35;
        pathStr = `M ${n.x - r},${n.y} a ${r},${r} 0 1,0 ${2 * r},0 a ${r},${r} 0 1,0 ${-2 * r},0`;
    }

    return {
        id: commId,
        nodes: commNodes,
        path: pathStr,
        cx: cx,
        cy: cy,
        count: commNodes.length
    };
}

function getCommunityGroups() {
    const showHulls = state.showHulls && visibilitySettings.showHulls;
    if (!showHulls) return [];

    const commMap = new Map();
    nodes.forEach(n => {
        if (!isNodeVisible(n)) return;
        // Quarantined hubs are excluded from hull bounds to avoid ballooning
        if (isQuarantinedHub(n)) return;

        const commId = getNodeCommunityId(n);
        if (commId !== null) {
            if (!commMap.has(commId)) {
                commMap.set(commId, []);
            }
            commMap.get(commId).push(n);
        }
    });

    const groups = [];
    commMap.forEach((commNodes, commId) => {
        const data = computeCommunityHullData(commId, commNodes);
        if (data) groups.push(data);
    });

    return groups;
}

function updateHullsTick() {
    if (!gHulls) return;

    const showHulls = state.showHulls && visibilitySettings.showHulls;
    if (!showHulls) {
        gHulls.selectAll(".community-hull-group").style("opacity", 0);
        return;
    }

    gHulls.selectAll(".community-hull-group")
        .each(function(d) {
            const data = computeCommunityHullData(d.id, d.nodes);
            if (!data) return;

            const group = d3.select(this);
            group.select(".community-hull")
                .attr("d", data.path);

            group.select(".community-hull-label")
                .attr("x", data.cx)
                .attr("y", data.cy - 35)
                .text(`Community #${data.id} (${data.count} nodes)`);
        });
}

export function updateGraph(newNodes, newLinks) {
    const newlyAddedIds = new Set();
    newNodes.forEach(n => {
        const id = n.id || n.Id;
        if (!id) return;
        
        if (!nodesMap.has(id)) {
            const props = n.properties || n.Properties || {};
            const normalizedNode = {
                id: id,
                name: n.name || props.name || id,
                properties: props,
                x: width / 2, // Default to center
                y: height / 2,
                ...n
            };
            nodesMap.set(id, normalizedNode);
            nodes.push(normalizedNode);
            newlyAddedIds.add(id);
        } else {
            // Update properties if they changed
            const existing = nodesMap.get(id);
            const props = n.properties || n.Properties || {};
            existing.properties = { ...existing.properties, ...props };
            if (n.name) existing.name = n.name;
        }
    });

    if (newLinks) {
        newLinks.forEach(l => {
            const sourceId = l.sourceId || l.source;
            const targetId = l.targetId || l.target;
            const linkId = `${sourceId}-${targetId}-${l.type}`;
            if (!linksMap.has(linkId)) {
                const link = { source: sourceId, target: targetId, type: l.type };
                linksMap.set(linkId, link);
                links.push(link);

                // If one of the endpoints was just added, seed coordinates
                if (newlyAddedIds.has(sourceId) && nodesMap.has(targetId) && !newlyAddedIds.has(targetId)) {
                    const sourceNode = nodesMap.get(sourceId);
                    const targetNode = nodesMap.get(targetId);
                    sourceNode.x = targetNode.x;
                    sourceNode.y = targetNode.y;
                } else if (newlyAddedIds.has(targetId) && nodesMap.has(sourceId) && !newlyAddedIds.has(sourceId)) {
                    const sourceNode = nodesMap.get(sourceId);
                    const targetNode = nodesMap.get(targetId);
                    targetNode.x = sourceNode.x;
                    targetNode.y = sourceNode.y;
                }
            }
        });
    }

    renderGraph();
}

export function renderGraph() {
    const {
        handleNodeClick,
        handleNodeMouseOver,
        handleNodeMouseOut,
        handleNodeDoubleClick,
        handleNodeContextMenu
    } = registeredHandlers;

    // Feature F18: Render Structural Community Convex Hulls
    const commGroups = getCommunityGroups();
    let hullSelection = gHulls.selectAll(".community-hull-group")
        .data(commGroups, d => d.id);

    hullSelection.exit().remove();

    const hullEnter = hullSelection.enter()
        .append("g")
        .attr("class", "community-hull-group");

    hullEnter.append("path")
        .attr("class", d => `community-hull ${state.selectedCommunity === d.id ? 'focused' : ''}`)
        .attr("fill", d => getCommunityColor(d.id))
        .attr("stroke", d => getCommunityColor(d.id))
        .attr("d", d => d.path);

    hullEnter.append("text")
        .attr("class", "community-hull-label")
        .attr("x", d => d.cx)
        .attr("y", d => d.cy - 35)
        .attr("text-anchor", "middle")
        .attr("fill", d => getCommunityColor(d.id))
        .text(d => `Community #${d.id} (${d.count} nodes)`);

    hullSelection = hullEnter.merge(hullSelection);

    const showHulls = state.showHulls && visibilitySettings.showHulls;
    hullSelection.transition().duration(400)
        .style("opacity", showHulls ? 1 : 0);

    hullSelection.select(".community-hull")
        .attr("class", d => `community-hull ${state.selectedCommunity === d.id ? 'focused' : ''}`)
        .attr("fill", d => getCommunityColor(d.id))
        .attr("stroke", d => getCommunityColor(d.id));

    hullSelection.select(".community-hull-label")
        .attr("fill", d => getCommunityColor(d.id));

    // Links
    let linkSelection = gLinks.selectAll(".link")
        .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}-${d.type}`);

    linkSelection.exit().remove();

    const linkEnter = linkSelection.enter().append("line")
        .attr("class", "link")
        .attr("stroke", "#94a3b8")
        .attr("stroke-width", 2)
        .attr("stroke-opacity", 0.6)
        .attr("marker-end", "url(#arrowhead)");

    linkSelection = linkEnter.merge(linkSelection);

    linkSelection.transition().duration(500)
        .style("opacity", d => {
            const source = d.source.id ? d.source : nodesMap.get(d.source);
            const target = d.target.id ? d.target : nodesMap.get(d.target);
            return (isNodeVisible(source) && isNodeVisible(target)) ? 1 : 0.05;
        });

    // Nodes
    let nodeSelection = gNodes.selectAll(".node-group")
        .data(nodes, d => d.id);

    nodeSelection.exit().remove();

    const nodeEnter = nodeSelection.enter()
        .append('g')
        .attr('class', 'node-group')
        .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended));

    if (handleNodeClick) nodeEnter.on("click", handleNodeClick);
    if (handleNodeMouseOver) nodeEnter.on("mouseover", handleNodeMouseOver);
    if (handleNodeMouseOut) nodeEnter.on("mouseout", handleNodeMouseOut);
    if (handleNodeDoubleClick) nodeEnter.on("dblclick", handleNodeDoubleClick);
    if (handleNodeContextMenu) nodeEnter.on("contextmenu", handleNodeContextMenu);

    nodeEnter.append('circle')
        .attr('class', 'node')
        .attr('r', 20)
        .attr('fill', d => getColor(d))
        .attr('stroke', '#fff')
        .attr('stroke-width', 2);

    nodeEnter.append('text')
        .attr('class', 'node-label')
        .attr('dy', 30)
        .attr('text-anchor', 'middle')
        .style('opacity', state.labelsVisible ? 1 : 0)
        .text(d => d.name || d.id);

    nodeSelection = nodeEnter.merge(nodeSelection);
    
    nodeSelection.transition().duration(500)
        .style("opacity", d => isNodeVisible(d) ? 1 : 0.1)
        .style("pointer-events", d => isNodeVisible(d) ? "auto" : "none");

    // Dynamic coloring & seam/topology badge classes (Features F18 & F19)
    nodeSelection.select('circle')
        .attr('fill', d => getColor(d))
        .attr('class', d => {
            let cls = 'node';
            if (seamState.showPinchPoints && seamState.pinchPoints.has(d.id)) cls += ' pinch-point';
            if (seamState.showSemanticSeams && isSemanticSeam(d.id)) cls += ' semantic-seam';
            if (isSharedBoundary(d)) cls += ' shared-boundary';
            if (isQuarantinedHub(d)) cls += ' cross-cutting-hub';
            return cls;
        });

    simulation.nodes(nodes);
    simulation.force("link").links(links);
    simulation.alpha(1).restart();
}

function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
}

function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
}

function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    // Keep the node fixed where it was dragged
    d.fx = d.x;
    d.fy = d.y;
}

export function zoomFit(transitionDuration = 750) {
    if (nodes.length === 0) return;

    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    nodes.forEach(n => {
        if (n.x < minX) minX = n.x;
        if (n.x > maxX) maxX = n.x;
        if (n.y < minY) minY = n.y;
        if (n.y > maxY) maxY = n.y;
    });

    if (minX === Infinity) return;

    const bWidth = maxX - minX;
    const bHeight = maxY - minY;
    
    const containerWidth = document.getElementById('graph-container').clientWidth || width;
    const containerHeight = document.getElementById('graph-container').clientHeight || height;
    
    // In case all nodes are at the exact same spot
    if (bWidth === 0 && bHeight === 0) {
        const transform = d3.zoomIdentity.translate(containerWidth / 2 - minX, containerHeight / 2 - minY).scale(1);
        svg.transition().duration(transitionDuration).call(zoom.transform, transform);
        return;
    }

    const midX = minX + bWidth / 2;
    const midY = minY + bHeight / 2;
    
    const padding = 0.15; // 15% padding
    const scaleX = (containerWidth * (1 - padding * 2)) / (bWidth || 1);
    const scaleY = (containerHeight * (1 - padding * 2)) / (bHeight || 1);
    
    // Min scale to prevent zooming out into oblivion, max scale to not zoom in too tight
    let scale = Math.min(scaleX, scaleY);
    scale = Math.max(0.1, Math.min(scale, 1.5));

    const transform = d3.zoomIdentity
        .translate(containerWidth / 2, containerHeight / 2)
        .scale(scale)
        .translate(-midX, -midY);

    svg.transition().duration(transitionDuration).call(zoom.transform, transform);
}
