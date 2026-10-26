import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server()
    assert c.request('POST','/api/mcp/servers',{'name':s['name'],'base_url':s['base_url']})[0]==400
    assert c.request('POST','/api/mcp/servers',{'name':'forbidden','base_url':s['base_url']},user='student_a')[0]==403
    assert 'token_env' not in s and 'token' not in s and s['has_token']
    assert c.env['MCP_AUTH_TOKEN'] not in str(s)
    assert c.scalar(f"SELECT COUNT(*) FROM mcp_servers WHERE id={s['id']}")=='1'

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
