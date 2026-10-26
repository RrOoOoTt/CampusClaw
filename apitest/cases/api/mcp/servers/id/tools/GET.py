import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();code,tools=c.request('GET',f"/api/mcp/servers/{s['id']}/tools")
    assert code==200 and {t['name'] for t in tools}=={'formula_hint','unit_convert'}
    assert all(t['description'] and t['inputSchema']['type']=='object' for t in tools)
    assert c.request('GET',f"/api/mcp/servers/{s['id']}/tools",user='teacher_b')[0]==404

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
