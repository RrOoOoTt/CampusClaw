import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();conv=c.mcp_conversation();path=f"/api/mcp/servers/{s['id']}"
    try:
        assert c.request('POST',path+'/disconnect')[0]==200
        assert c.request('GET',path+'/tools')[1]==[]
        skill=c.fixture['mcp_skill'];assert c.request('POST','/api/skills',{'source':skill['source'].replace(skill['name'],skill['name']+'-disconnected')})[0]==400
        events=c.stream('请给三角形面积的公式提示',conv['id']);assert events[0][1]['tool_calls'] and events[0][1]['tool_calls'][0]['status']=='denied'
        assert not any(x.get('status')=='ok' for x in events[-1][1].get('tool_calls',[]))
        assert int(c.scalar(f"SELECT COUNT(*) FROM mcp_calls WHERE server_id={s['id']} AND status='denied'"))>=1
    finally:
        assert c.request('POST',path+'/connect')[0]==200

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
