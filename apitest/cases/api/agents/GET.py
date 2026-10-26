import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/'support.py').exists():sys.path.insert(0,str(parent));break

def run(c):
    a=c.agent();code,agents=c.request('GET','/api/agents',user='student_a')
    assert code==200 and any(v['id']==a['id'] for v in agents)
    for item in agents:
        assert item['enabled']
        assert c.scalar(f"SELECT COUNT(*) FROM tutor_agents a JOIN users u ON a.class_id=u.class_id WHERE a.id={item['id']} AND u.username='student_a'")=='1'
    code,foreign=c.request('GET','/api/agents',user='teacher_b')
    assert code==200 and all(v['id']!=a['id'] for v in foreign)
if __name__=='__main__':
    from run_all import run_single
    sys.exit(run_single(__file__))
