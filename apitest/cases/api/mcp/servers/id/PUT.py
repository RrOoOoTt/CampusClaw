import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();path=f"/api/mcp/servers/{s['id']}"
    try:
        assert c.request('PUT',path,dict(name=s['name'],base_url=s['base_url'],token_env='MCP_TOKEN_UNCONFIGURED'))[0]==200
        code,r=c.request('POST',path+'/connect');assert code==400 and r['error']
        assert c.scalar(f"SELECT connected FROM mcp_servers WHERE id={s['id']}")=='0'
        assert c.scalar(f"SELECT COUNT(*) FROM mcp_tools WHERE server_id={s['id']}")=='0'
    finally:
        assert c.request('PUT',path,dict(name=s['name'],base_url=s['base_url'],token_env='MCP_AUTH_TOKEN'))[0]==200
        assert c.request('POST',path+'/connect')[0]==200

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
