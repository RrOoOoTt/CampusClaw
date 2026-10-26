import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    a=c.agent();code,r=c.request('GET',f"/api/search?q=%E4%B8%89%E8%A7%92%E5%BD%A2%E9%9D%A2%E7%A7%AF&agent_id={a['id']}",user='student_a')
    assert code==200 and r['hits'],(code,r)
    for h in r['hits']:
        assert h['material_id'] in a['material_ids']
        assert c.scalar(f"SELECT material_id FROM knowledge_chunks WHERE id={h['chunk_id']}")==str(h['material_id'])
    assert c.request('GET',f"/api/search?q=test&agent_id={a['id']}",user='teacher_b')[0]==404

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
