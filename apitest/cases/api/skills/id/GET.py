import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    s=c.skill()
    assert c.request('GET',f"/api/skills/{s['id']}",user='student_a')[1]['name']==s['name']
    assert c.request('GET',f"/api/skills/{s['id']}",user='teacher_b')==c.request('GET','/api/skills/999999999',user='teacher_b')

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
