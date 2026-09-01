import { nodeColors, communityPalette, domainPalette } from './config.js';
import { nodes, nodesMap, visibilitySettings, state } from './state.js';

export function getCommunityColor(communityId) {
    if (communityId === null || communityId === undefined || communityId === '') return '#64748b';
    let hash = 0;
    const str = String(communityId);
    for (let i = 0; i < str.length; i++) {
        hash = ((hash << 5) - hash) + str.charCodeAt(i);
        hash |= 0;
    }
    const idx = Math.abs(hash) % communityPalette.length;
    return communityPalette[idx];
}

export function getDomainColor(domainName) {
    if (!domainName) return '#94a3b8';
    let hash = 0;
    const str = String(domainName);
    for (let i = 0; i < str.length; i++) {
        hash = ((hash << 5) - hash) + str.charCodeAt(i);
        hash |= 0;
    }
    const idx = Math.abs(hash) % domainPalette.length;
    return domainPalette[idx];
}

export function getNodeCommunityId(n) {
    if (!n) return null;
    const props = n.properties || n.Properties || {};
    if (props.community_id !== undefined && props.community_id !== null && props.community_id !== '') return String(props.community_id);
    if (props.structural_community_id !== undefined && props.structural_community_id !== null && props.structural_community_id !== '') return String(props.structural_community_id);
    if (props.community !== undefined && props.community !== null && props.community !== '') return String(props.community);
    if (n.community_id !== undefined && n.community_id !== null && n.community_id !== '') return String(n.community_id);
    if (n.community !== undefined && n.community !== null && n.community !== '') return String(n.community);
    return null;
}

export function getNodeDomain(n) {
    if (!n) return null;
    const props = n.properties || n.Properties || {};
    if (props.domain) return String(props.domain);
    if (props.domain_name) return String(props.domain_name);
    if (props.semantic_domain) return String(props.semantic_domain);
    const label = (n.label || props.node_label || props.label || '').toLowerCase();
    if (label === 'domain') {
        return n.name || props.name || n.id;
    }
    return null;
}

export function isSharedBoundary(n) {
    if (!n) return false;
    const props = n.properties || n.Properties || {};
    if (props.is_shared_boundary === true || props.is_boundary === true) return true;
    if (props.bpr !== undefined && typeof props.bpr === 'number' && props.bpr >= 0.25) return true;
    if (props.bpr_max !== undefined && typeof props.bpr_max === 'number' && props.bpr_max >= 0.25) return true;
    const label = n.label || props.node_label || props.label || '';
    return label === 'SharedBoundary' || (Array.isArray(n.labels) && n.labels.includes('SharedBoundary'));
}

export function isQuarantinedHub(n) {
    if (!n) return false;
    const props = n.properties || n.Properties || {};
    if (props.is_hub === true || props.is_quarantined === true || props.is_cross_cutting_hub === true) return true;
    const label = n.label || props.node_label || props.label || '';
    return label === 'CrossCuttingHub' || (Array.isArray(n.labels) && n.labels.includes('CrossCuttingHub'));
}

export function getColor(node) {
    if (!node) return nodeColors['Unknown'];

    // Dual-Lens Contrast Mode (Feature F19):
    // In Dual-Lens mode, node circles are colored by semantic :Domain
    if (state.showDualLens || visibilitySettings.showDualLens) {
        if (isQuarantinedHub(node)) {
            return nodeColors['CrossCuttingHub'] || '#f59e0b';
        }
        const domain = getNodeDomain(node);
        if (domain) {
            return getDomainColor(domain);
        }
    }

    let rawLabel = node.label || 'Unknown';
    if ((rawLabel === 'CodeElement' || rawLabel === 'Unknown') && node.properties && node.properties.node_label) {
        rawLabel = node.properties.node_label;
    }
    if (isQuarantinedHub(node)) {
        rawLabel = 'CrossCuttingHub';
    } else if (isSharedBoundary(node)) {
        rawLabel = 'SharedBoundary';
    }

    const match = Object.keys(nodeColors).find(k => k.toLowerCase() === rawLabel.toLowerCase());
    return match ? nodeColors[match] : nodeColors['Unknown'];
}

export function updateLegend() {
    const legendContainer = document.getElementById('dynamic-legend');
    if (!legendContainer) return;

    const isDualLens = state.showDualLens || visibilitySettings.showDualLens;

    let html = `<h4 class="text-xs font-bold mb-2 uppercase tracking-wider text-slate-500">
        ${isDualLens ? '👁️ Dual-Lens Legend' : 'Graph Legend'}
    </h4>`;
    html += `<div class="flex flex-col gap-2.5">`;

    if (isDualLens) {
        html += `
            <div class="flex items-center gap-2.5">
                <div class="size-3 rounded border-2 border-emerald-400 bg-emerald-400/20"></div>
                <span class="text-[10px] font-medium uppercase tracking-wide">Hulls: Structural Communities (Leiden)</span>
            </div>
            <div class="flex items-center gap-2.5">
                <div class="size-3 rounded-full bg-blue-500"></div>
                <span class="text-[10px] font-medium uppercase tracking-wide">Nodes: Semantic Domains (RPG)</span>
            </div>
            <div class="flex items-center gap-2.5">
                <div class="size-3 rounded-full border-2 border-[#06b6d4] border-dashed bg-[#06b6d4]/30"></div>
                <span class="text-[10px] font-medium uppercase tracking-wide">Shared Boundary (BPR ≥ 0.25)</span>
            </div>
            <div class="flex items-center gap-2.5">
                <div class="size-3 rounded-full bg-[#f59e0b] shadow-[0_0_6px_#f59e0b]"></div>
                <span class="text-[10px] font-medium uppercase tracking-wide">Quarantined Hub (Top 1% Degree)</span>
            </div>
        `;
    } else {
        for (const [label, color] of Object.entries(nodeColors)) {
            if (label === 'Unknown') continue;
            html += `
                <div class="flex items-center gap-2.5">
                    <div class="size-3 rounded-full" style="background-color: ${color}"></div>
                    <span class="text-[10px] font-medium uppercase tracking-wide">${label}</span>
                </div>`;
        }
    }

    html += `
        <div class="h-px bg-slate-200 dark:bg-slate-800 my-1"></div>
        <div class="flex items-center gap-2.5">
            <div class="size-3 rounded-full border-2 border-[#ff3366] border-dashed"></div>
            <span class="text-[10px] font-medium uppercase tracking-wide">Semantic Seam</span>
        </div>
        <div class="flex items-center gap-2.5">
            <div class="size-3 rounded-full border-2 border-yellow-400"></div>
            <span class="text-[10px] font-medium uppercase tracking-wide">Actionable Pinch Point (≤ 4 Cut-Edges)</span>
        </div>
    </div>`;

    legendContainer.innerHTML = html;
}

export function isSemantic(n) {
    if (!n) return false;
    let label = (n.label || 'Node').toLowerCase();
    if (n.properties && n.properties.node_label) {
        label = n.properties.node_label.toLowerCase();
    } else if (n.properties && n.properties.label) {
        label = n.properties.label.toLowerCase();
    }
    return label === 'domain' || label === 'feature';
}

export function isPhysical(n) {
    if (!n) return false;
    return !isSemantic(n);
}

export function isNodeVisible(n) {
    if (!n) return false;
    
    if (n.properties && n.properties.is_test === true && !visibilitySettings.showTests) {
        return false;
    }

    if (isSemantic(n)) return visibilitySettings.showSemantic;
    return visibilitySettings.showPhysical;
}

export function resolveNodeName(nodeId) {
    const node = nodesMap.get(nodeId);
    if (node) {
        return node.name || (node.properties && node.properties.name) || nodeId;
    }
    return nodeId;
}

export function togglePanel(id, show) {
    const panel = document.getElementById(id);
    if (!panel) return;
    if (show === undefined) {
        panel.style.display = panel.style.display === 'none' ? 'flex' : 'none';
    } else {
        panel.style.display = show ? 'flex' : 'none';
    }
}

export function showNodeDetails(d) {
    const panel = document.getElementById('impact-panel');
    if (!panel) return;
    panel.style.display = 'flex';
    
    const rawName = d.name || d.id;
    let displayName = rawName;
    if (rawName && rawName.length > 40) {
        if (rawName.includes('/') || rawName.includes('\\') || rawName.includes('.') || rawName.includes(':')) {
            displayName = '...' + rawName.substring(rawName.length - 37);
        }
    }
    const nameEl = document.getElementById('impact-node-name');
    nameEl.textContent = displayName;
    nameEl.title = rawName;
    
    const typeEl = document.getElementById('impact-node-type');
    if (typeEl) {
        typeEl.textContent = d.label || (d.properties && d.properties.label) || (d.properties && d.properties.node_label) || 'Node';
    }
    
    const placeholder = document.getElementById('impact-placeholder');
    if (placeholder) placeholder.classList.add('hidden');
    
    const details = document.getElementById('impact-details');
    if (details) details.classList.remove('hidden');
    
    const props = d.properties || d.Properties || {};
    const riskScore = props.volatility_score || 0;

    const maxRisk = nodes.reduce((max, node) => {
        const score = (node.properties && node.properties.volatility_score) || 0;
        return score > max ? score : max;
    }, 0.0001);

    const riskPercent = Math.min(100, Math.round((riskScore / maxRisk) * 100));
    
    const riskScoreEl = document.getElementById('risk-score');
    if (riskScoreEl) riskScoreEl.textContent = `${riskPercent}/100`;
    
    const riskBarEl = document.getElementById('risk-bar');
    if (riskBarEl) riskBarEl.style.width = `${riskPercent}%`;
    
    let riskLabel = 'LOW';
    if (riskPercent > 70) riskLabel = 'CRITICAL';
    else if (riskPercent > 40) riskLabel = 'MEDIUM';
    
    const riskLabelEl = document.getElementById('risk-label');
    if (riskLabelEl) riskLabelEl.textContent = riskLabel;
    
    const propsContainer = document.getElementById('impact-properties');
    if (propsContainer) {
        propsContainer.innerHTML = '';
        
        // Update risk description if it exists in props
        const riskDescEl = document.getElementById('risk-description');
        if (riskDescEl) {
            if (props.description) {
                riskDescEl.textContent = props.description;
                riskDescEl.classList.remove('italic');
            } else {
                riskDescEl.textContent = 'No description available for this component.';
                riskDescEl.classList.add('italic');
            }
        }

        // Structural Community & Dual-Lens Metadata Card (Feature F19)
        const commId = getNodeCommunityId(d);
        if (commId !== null || isSharedBoundary(d) || isQuarantinedHub(d)) {
            const commCard = document.createElement('div');
            commCard.className = 'p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 mb-3 flex flex-col gap-2';
            
            let commHtml = `
                <div class="flex items-center justify-between">
                    <span class="text-[10px] font-bold uppercase tracking-wider text-emerald-400">Structural Topology</span>
                    <span class="px-1.5 py-0.5 rounded text-[9px] font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">CPM Leiden</span>
                </div>
            `;
            if (commId !== null) {
                const commColor = getCommunityColor(commId);
                commHtml += `
                    <div class="flex items-center gap-2">
                        <span class="size-2.5 rounded-full" style="background-color: ${commColor}"></span>
                        <span class="text-xs font-bold text-slate-200">Community #${commId}</span>
                        ${props.community_name ? `<span class="text-xs text-slate-400 truncate">(${props.community_name})</span>` : ''}
                    </div>
                `;
            }
            if (props.community_size !== undefined) {
                commHtml += `<div class="text-[11px] text-slate-300"><span class="text-slate-500">Size:</span> ${props.community_size} nodes</div>`;
            }
            if (props.community_density !== undefined) {
                commHtml += `<div class="text-[11px] text-slate-300"><span class="text-slate-500">Density:</span> ${(props.community_density).toFixed(3)}</div>`;
            }
            if (isSharedBoundary(d)) {
                const bprVal = props.bpr || props.bpr_max || props.bpr_avg || 0.25;
                commHtml += `
                    <div class="p-1.5 rounded bg-cyan-500/20 border border-cyan-500/40 text-[10px] text-cyan-200">
                        <span class="font-bold">⚡ Shared Boundary:</span> BPR ${(Number(bprVal)).toFixed(2)}
                        ${props.bridged_communities ? `<div class="text-[9px] text-cyan-300 mt-0.5">Bridges: ${props.bridged_communities}</div>` : ''}
                    </div>
                `;
            }
            if (isQuarantinedHub(d)) {
                commHtml += `
                    <div class="p-1.5 rounded bg-amber-500/20 border border-amber-500/40 text-[10px] text-amber-200">
                        <span class="font-bold">⚠️ Quarantined Hub:</span> Top 1% Centrality (${props.degree || 'High'} degree)
                    </div>
                `;
            }
            if (props.dominant_domains && Array.isArray(props.dominant_domains) && props.dominant_domains.length > 0) {
                commHtml += `
                    <div class="text-[10px] text-slate-400 mt-1">
                        <span class="font-semibold text-slate-300">Dominant Domains:</span>
                        <div class="flex flex-wrap gap-1 mt-1">
                            ${props.dominant_domains.map(dom => `<span class="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-300 text-[9px] border border-blue-500/30">${dom}</span>`).join('')}
                        </div>
                    </div>
                `;
            }
            commCard.innerHTML = commHtml;
            propsContainer.appendChild(commCard);
        }

        // Semantic Domain details
        const domainName = getNodeDomain(d);
        if (domainName) {
            const domainColor = getDomainColor(domainName);
            const domRow = document.createElement('div');
            domRow.className = 'grid grid-cols-3 gap-2 border-b border-slate-700/50 pb-1 mb-1';
            domRow.innerHTML = `
                <span class="text-slate-500 font-medium capitalize truncate" title="Semantic Domain">Semantic Domain</span>
                <span class="col-span-2 truncate text-slate-200 flex items-center gap-1.5" title="${domainName}">
                    <span class="size-2 rounded-full" style="background-color: ${domainColor}"></span>
                    ${domainName}
                </span>
            `;
            propsContainer.appendChild(domRow);
        }

        for (const [key, value] of Object.entries(props)) {
            if (key === 'name' || key === 'id' || key === 'description' || key === 'atomic_features' || key.startsWith('community') || key === 'dominant_domains' || key === 'domain') continue;
            const row = document.createElement('div');
            row.className = 'grid grid-cols-3 gap-2 border-b border-slate-700/50 pb-1 mb-1';
            
            let displayValue = value;
            // Format numeric scores as percentages
            if (typeof value === 'number' && (key.includes('score') || key.includes('risk') || key.includes('volatility') || key.includes('bpr'))) {
                displayValue = (value * 100).toFixed(1) + '%';
            } else if (String(value).length > 30) {
                const lowerKey = key.toLowerCase();
                if (lowerKey === 'file' || lowerKey === 'fqn' || lowerKey === 'path' || lowerKey === 'module' || lowerKey === 'full_name') {
                    displayValue = '...' + String(value).substring(String(value).length - 27);
                } else {
                    displayValue = String(value).substring(0, 27) + '...';
                }
            }
            
            row.innerHTML = `<span class="text-slate-500 font-medium capitalize truncate" title="${key}">${key.replace(/_/g, ' ')}</span><span class="col-span-2 truncate text-slate-300" title="${value}">${displayValue}</span>`;
            propsContainer.appendChild(row);
        }

        // Add atomic features at the end if it exists, full width
        if (props.atomic_features && Array.isArray(props.atomic_features) && props.atomic_features.length > 0) {
            const afRow = document.createElement('div');
            afRow.className = 'flex flex-col gap-1 border-b border-slate-700/50 pb-2 mb-1 mt-2';
            afRow.innerHTML = `
                <span class="text-slate-500 font-medium capitalize text-[10px] uppercase tracking-wider">Atomic Features</span>
                <span class="text-slate-300 text-xs leading-relaxed whitespace-pre-wrap">${props.atomic_features.join(', ')}</span>
            `;
            propsContainer.appendChild(afRow);
        }

        // Add description at the end if it exists, full width
        if (props.description) {
            const descRow = document.createElement('div');
            descRow.className = 'flex flex-col gap-1 border-b border-slate-700/50 pb-2 mb-1 mt-2';
            descRow.innerHTML = `
                <span class="text-slate-500 font-medium capitalize text-[10px] uppercase tracking-wider">Description</span>
                <span class="text-slate-300 text-xs leading-relaxed whitespace-pre-wrap">${props.description}</span>
            `;
            propsContainer.appendChild(descRow);
        }
    }

    const btnExpand = document.getElementById('btn-expand-node');
    if (btnExpand) {
        if (d._expanded) {
            btnExpand.innerHTML = '<span class="material-symbols-outlined text-[14px]">close_fullscreen</span> Collapse Relationships';
        } else {
            btnExpand.innerHTML = '<span class="material-symbols-outlined text-[14px]">open_in_full</span> Expand Relationships';
        }
    }
}
