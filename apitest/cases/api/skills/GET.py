import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/'support.py').exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.skill();code,skills=c.request('GET','/api/skills',user='student_a')
    assert code==200 and any(v['id']==s['id'] for v in skills)
    for item in skills:
        assert c.scalar(f"SELECT COUNT(*) FROM tutor_skills s JOIN users u ON s.class_id=u.class_id WHERE s.id={item['id']} AND u.username='student_a'")=='1'
if __name__=='__main__':
    from run_all import run_single
    sys.exit(run_single(__file__))
