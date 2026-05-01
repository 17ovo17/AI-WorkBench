import json
d = json.load(open("/tmp/biz_topo.json"))
biz = d[0]
g = biz.get("graph", {})
nodes = g.get("nodes", [])
edges = g.get("edges", [])
svc_nodes = [n for n in nodes if n.get("type") != "host"]
svc_ids = set(n["id"] for n in svc_nodes)
host_nodes = [n for n in nodes if n.get("type") == "host"]
print(f"Total nodes: {len(nodes)}, non-host: {len(svc_nodes)}, host: {len(host_nodes)}")
for n in svc_nodes:
    print(f"  SVC {n['id']} type={n.get('type','')} name={n.get('name','')}")
matched = [e for e in edges if e.get("source_id") in svc_ids and e.get("target_id") in svc_ids]
print(f"\nEdges both non-host: {len(matched)}/{len(edges)}")
for e in matched[:5]:
    print(f"  {e['source_id']} -> {e['target_id']} {e.get('protocol','')}")
unmatched = [e for e in edges if e.get("source_id") not in svc_ids or e.get("target_id") not in svc_ids]
print(f"\nEdges with host (LOST): {len(unmatched)}")
for e in unmatched[:5]:
    print(f"  {e['source_id']} -> {e['target_id']} {e.get('protocol','')}")
