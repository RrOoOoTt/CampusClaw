import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    s=c.skill();source=s['source'].replace('启用：是','启用：否')
    try:
        assert c.request('PUT',f"/api/skills/{s['id']}",{'source':source})[0]==200
        assert c.scalar(f"SELECT enabled FROM tutor_skills WHERE id={s['id']}")=='0'
        assert c.request('PUT',f"/api/skills/{s['id']}",{'source':s['source']})[0]==200
    finally:
        code,result=c.request('PUT',f"/api/skills/{s['id']}",{'source':s['source']})
        assert code==200,('restore skill failed',code,result)

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
