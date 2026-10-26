import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    a=c.agent()
    assert c.scalar(f"SELECT system_prompt FROM tutor_agents WHERE id={a['id']}")==a['prompt']
    assert c.scalar(f"SELECT COUNT(*) FROM tutor_agent_skills WHERE agent_id={a['id']}")=='1'
    assert c.scalar(f"SELECT COUNT(*) FROM tutor_agent_materials WHERE agent_id={a['id']}")=='1'
    assert c.request('POST','/api/agents',a,user='student_a')[0]==403
    assert c.request('POST','/api/agents',a)[0]==400

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
