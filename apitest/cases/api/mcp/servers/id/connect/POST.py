import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server()
    for _ in range(2):
        code,r=c.request('POST',f"/api/mcp/servers/{s['id']}/connect");assert code==200 and r['connected']
        assert c.scalar(f"SELECT COUNT(*) FROM mcp_tools WHERE server_id={s['id']}")=='2'
    assert c.request('POST',f"/api/mcp/servers/{s['id']}/connect",user='student_a')[0]==403

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
