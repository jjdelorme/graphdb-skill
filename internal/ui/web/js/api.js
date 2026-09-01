export async function fetchOverview() {
    const response = await fetch('/api/query?type=overview');
    if (!response.ok) throw new Error('Failed to fetch overview');
    return await response.json();
}

export async function fetchTraverse(targetId, depth = 1, direction = 'both', edgeTypes = "") {
    let url = `/api/query?type=traverse&target=${encodeURIComponent(targetId)}&direction=${direction}&depth=${depth}`;
    if (edgeTypes) url += `&edge-types=${encodeURIComponent(edgeTypes)}`;
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to fetch traverse');
    return await response.json();
}

export async function fetchWhatIf(targetId) {
    const response = await fetch(`/api/query?type=what-if&target=${encodeURIComponent(targetId)}`);
    if (!response.ok) throw new Error('Failed to fetch what-if analysis');
    return await response.json();
}

export async function fetchSearch(target) {
    const response = await fetch(`/api/query?type=search-all&target=${encodeURIComponent(target)}&limit=20`);
    if (!response.ok) throw new Error('Failed to fetch search results');
    return await response.json();
}

export async function fetchSeams(type = 'seams') {
    let endpoint = '/api/query?type=seams';
    if (type === 'semantic-seams') {
        endpoint = '/api/query?type=semantic-seams&similarity=0.6';
    } else if (type === 'dual-lens' || type === 'dual-lens-seams') {
        endpoint = '/api/query?type=dual-lens-seams&limit=50';
    }
    const response = await fetch(endpoint);
    if (!response.ok) throw new Error(`Failed to fetch ${type}`);
    return await response.json();
}

export async function fetchCommunities(limit = 100) {
    const response = await fetch(`/api/query?type=communities&limit=${limit}`);
    if (!response.ok) throw new Error('Failed to fetch structural communities');
    return await response.json();
}

export async function fetchDivergence(domain = "") {
    let url = '/api/query?type=divergence';
    if (domain) url += `&target=${encodeURIComponent(domain)}`;
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to fetch domain divergence');
    return await response.json();
}

export async function fetchSemanticTrace(targetId) {
    const response = await fetch(`/api/query?type=semantic-trace&target=${encodeURIComponent(targetId)}`);
    if (!response.ok) throw new Error('Failed to fetch semantic trace');
    return await response.json();
}

export async function fetchNeighbors(targetId) {
    const response = await fetch(`/api/query?type=neighbors&target=${encodeURIComponent(targetId)}`);
    if (!response.ok) throw new Error('Failed to fetch neighbors');
    return await response.json();
}

export async function fetchStatus() {
    const response = await fetch('/api/query?type=status');
    if (!response.ok) throw new Error('Failed to fetch status');
    return await response.json();
}

export async function fetchHealth() {
    const response = await fetch('/api/health');
    if (!response.ok) throw new Error('Failed to fetch health');
    return await response.json();
}

export async function fetchConfig() {
    const response = await fetch('/api/config');
    if (!response.ok) throw new Error('Failed to fetch config');
    return await response.json();
}
