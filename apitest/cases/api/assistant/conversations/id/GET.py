import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    conv=c.conversation()
    assert c.request('GET',f"/api/assistant/conversations/{conv['id']}",user='teacher_a')==c.request('GET','/api/assistant/conversations/999999999',user='teacher_a')
    assert c.request('GET',f"/api/assistant/conversations/{conv['id']}",user='student_a')[0]==200

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
