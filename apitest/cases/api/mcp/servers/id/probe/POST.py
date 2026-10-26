import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();path=f"/api/mcp/servers/{s['id']}/probe"
    code,r=c.request('POST',path,{'name':'unit_convert','arguments':{'from':'m','to':'cm'}});assert code==200 and r['status']=='ok' and '100' in r['result']
    code,r=c.request('POST',path,{'name':'unit_convert','arguments':{'from':'kg','to':'cm'}});assert code==200 and r['status']=='error'
    code,r=c.request('POST',path,{'name':'unit_convert','arguments':{'from':'m','to':'cm','extra':'x'}});assert code==200 and r['status']=='denied'
    assert c.request('POST',path,{'name':'unit_convert','arguments':{}},user='student_a')[0]==403
    for status in ('ok','error','denied'):
        assert int(c.scalar(f"SELECT COUNT(*) FROM mcp_calls WHERE server_id={s['id']} AND status='{status}'"))>=1

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
