import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();code,rows=c.request('GET','/api/mcp/servers')
    assert code==200 and any(x['id']==s['id'] for x in rows)
    assert c.env['MCP_AUTH_TOKEN'] not in str(rows)
    assert c.request('GET','/api/mcp/servers',user='student_a')[0]==403
    code,other=c.request('GET','/api/mcp/servers',user='teacher_b');assert code==200 and all(x['id']!=s['id'] for x in other)

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
