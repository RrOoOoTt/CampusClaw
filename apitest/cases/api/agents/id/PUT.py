import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    a=c.agent();body=dict(a,prompt='',enabled=False)
    try:
        assert c.request('PUT',f"/api/agents/{a['id']}",body)[0]==200
        assert c.scalar(f"SELECT system_prompt FROM tutor_agents WHERE id={a['id']}")==''
        assert c.request('GET',f"/api/agents/{a['id']}",user='student_a')[0]==404
        assert c.request('POST','/api/assistant/conversations',{'agent_id':a['id']},user='student_a')[0]==404
        assert c.request('PUT',f"/api/agents/{a['id']}",a)[0]==200
        bad=dict(a,skill_ids=[999999999]);assert c.request('PUT',f"/api/agents/{a['id']}",bad)[0]==404
    finally:
        code,result=c.request('PUT',f"/api/agents/{a['id']}",a)
        assert code==200,('restore agent failed',code,result)

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
