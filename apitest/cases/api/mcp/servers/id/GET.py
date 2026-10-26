import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server()
    assert c.request('GET',f"/api/mcp/servers/{s['id']}",user='teacher_b')==c.request('GET','/api/mcp/servers/999999999',user='teacher_b')
    assert c.request('GET',f"/api/mcp/servers/{s['id']}")[1]['has_token']

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
