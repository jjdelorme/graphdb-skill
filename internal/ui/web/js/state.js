export const nodes = [];
export const links = [];
export const nodesMap = new Map();
export const linksMap = new Map();

export const state = {
    lastSelectedNode: null,
    labelsVisible: true,
    showHulls: true,
    showDualLens: false,
    selectedCommunity: null
};

export const visibilitySettings = {
    showPhysical: true,
    showSemantic: true,
    showTests: true,
    showHulls: true,
    showDualLens: false
};

export const seamState = {
    showPinchPoints: false,
    showSemanticSeams: false,
    pinchPoints: new Set(),
    semanticSeams: []
};
