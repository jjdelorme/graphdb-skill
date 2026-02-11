import sys
import json
import os

def normalize_js_node(obj):
    # JS: { id, label: name, type, ... }
    # print(f"DEBUG: {obj.get('label')} -> file: {obj.get('file')}")
    return {
        "label": obj.get("type"),
        "name": obj.get("label"),
        "file": obj.get("file"),
        "id": obj.get("id")
    }

def normalize_go_node(obj):
    # Go output is flattened: { "id": ..., "type": "Label", "name": "Name", "file": "File" }
    return {
        "label": obj.get("type"),
        "name": obj.get("name"),
        "file": obj.get("file"),
        "id": obj.get("id")
    }

def normalize_js_edge(obj, id_to_node):
    # JS: { source, target, type }
    src_node = id_to_node.get(obj.get("source"))
    tgt_node = id_to_node.get(obj.get("target"))
    
    src_name = src_node["name"] if src_node else obj.get("source")
    tgt_name = tgt_node["name"] if tgt_node else obj.get("target")
    
    return {
        "source": src_name,
        "target": tgt_name,
        "type": obj.get("type")
    }

def normalize_go_edge(obj, id_to_node):
    # Go: { source, target, type }
    src_id = obj.get("source")
    tgt_id = obj.get("target")
    
    src_node = id_to_node.get(src_id)
    tgt_node = id_to_node.get(tgt_id)
    
    # If node not found (maybe external?), use ID part?
    # Go IDs are "file:name". We can extract name.
    
    def extract_name(full_id, node):
        if node: 
             return node["name"]
        if full_id and ":" in full_id: 
             parts = full_id.split(":")
             return parts[-1]
        return full_id

    return {
        "source": extract_name(src_id, src_node),
        "target": extract_name(tgt_id, tgt_node),
        "type": obj.get("type") # lowercase 'type'
    }

def load_jsonl(filepath):
    data = []
    if not os.path.exists(filepath):
        print(f"Warning: File not found {filepath}")
        return data
        
    with open(filepath, 'r') as f:
        for line in f:
            if line.strip():
                try:
                    data.append(json.loads(line))
                except:
                    pass
    return data

def main():
    print("Starting verification...")
    if len(sys.argv) < 3:
        print("Usage: python3 verify_parity.py <legacy_dir_or_file> <go_file>")
        sys.exit(1)

    legacy_input = sys.argv[1] # Can be directory containing nodes.jsonl/edges.jsonl or just nodes.jsonl
    go_file = sys.argv[2]
    
    l_nodes_raw = []
    l_edges_raw = []
    
    if os.path.isdir(legacy_input):
        l_nodes_raw = load_jsonl(os.path.join(legacy_input, "nodes.jsonl"))
        l_edges_raw = load_jsonl(os.path.join(legacy_input, "edges.jsonl"))
    else:
        # Assume it's a combined file or just one
        raw = load_jsonl(legacy_input)
        for item in raw:
            if "source" in item and "target" in item:
                l_edges_raw.append(item)
            else:
                l_nodes_raw.append(item)

    g_raw = load_jsonl(go_file)
    g_nodes_raw = []
    g_edges_raw = []
    for item in g_raw:
        if "source" in item and "target" in item:
            g_edges_raw.append(item)
        else:
            g_nodes_raw.append(item)

    # Normalize
    l_nodes_map = {} # id -> norm_node
    l_nodes_set = set() # (label, name)
    
    for n in l_nodes_raw:
        norm = normalize_js_node(n)
        l_nodes_map[n.get("id")] = norm
        
        # We assume file path might differ (relative vs absolute).
        # We normalize file path to basename for comparison
        fpath = norm["file"]
        if fpath: fpath = os.path.basename(fpath) 
        
        # Using basename for file to be safe against path variations
        key = (norm["label"], norm["name"], fpath) 
        l_nodes_set.add(key)

    g_nodes_map = {}
    g_nodes_set = set()
    
    for n in g_nodes_raw:
        norm = normalize_go_node(n)
        g_nodes_map[n.get("id")] = norm
        
        fpath = norm["file"]
        if fpath: fpath = os.path.basename(fpath)

        key = (norm["label"], norm["name"], fpath)
        g_nodes_set.add(key)

    # Compare Nodes
    print(f"--- Node Comparison (Label, Name, FileBasename) ---")
    common = l_nodes_set.intersection(g_nodes_set)
    missing = l_nodes_set - g_nodes_set
    extra = g_nodes_set - l_nodes_set
    
    print(f"Common: {len(common)}")
    print(f"Missing in Go: {len(missing)}")
    if len(missing) > 0:
        for i in list(missing)[:5]: print(f" - {i}")
        
    print(f"Extra in Go: {len(extra)}")
    if len(extra) > 0:
        for i in list(extra)[:5]: print(f" + {i}")

    # Compare Edges
    # Key: (Source_Name, Target_Name, Type)
    l_edges_set = set()
    for e in l_edges_raw:
        norm = normalize_js_edge(e, l_nodes_map)
        key = (norm["source"], norm["target"], norm["type"])
        l_edges_set.add(key)

    g_edges_set = set()
    for e in g_edges_raw:
        norm = normalize_go_edge(e, g_nodes_map)
        key = (norm["source"], norm["target"], norm["type"])
        g_edges_set.add(key)
        
    print(f"\n--- Edge Comparison (SrcName, TgtName, Type) ---")
    common_e = l_edges_set.intersection(g_edges_set)
    missing_e = l_edges_set - g_edges_set
    extra_e = g_edges_set - l_edges_set
    
    print(f"Common: {len(common_e)}")
    if len(common_e) > 0:
        print("Sample common edges:")
        for i in list(common_e)[:5]: print(f" = {i}")

    print(f"Missing in Go: {len(missing_e)}")
    if len(missing_e) > 0:
        for i in list(missing_e)[:5]: print(f" - {i}")

    print(f"Extra in Go: {len(extra_e)}")
    if len(extra_e) > 0:
        for i in list(extra_e)[:5]: print(f" + {i}")

    if len(missing) == 0 and len(extra) == 0 and len(missing_e) == 0 and len(extra_e) == 0:
        print("\n✅ PERFECT MATCH")
    else:
        print("\n❌ MISMATCH")

if __name__ == "__main__":
    main()
