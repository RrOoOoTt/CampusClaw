import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    a=c.agent()
    assert c.request('GET',f"/api/agents/{a['id']}",user='student_a')[1]['id']==a['id']
    foreign=c.request('GET',f"/api/agents/{a['id']}",user='teacher_b');missing=c.request('GET','/api/agents/999999999',user='teacher_b')
    assert foreign==missing and foreign[0]==404

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
