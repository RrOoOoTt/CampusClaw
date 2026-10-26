import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/'support.py').exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.skill()
    assert bytes.fromhex(c.scalar(f"SELECT HEX(source_text) FROM tutor_skills WHERE id={s['id']}" )).decode()==s['source']
    assert c.request('POST','/api/skills',{'source':s['source']})[0]==400
    assert c.request('POST','/api/skills',{'source':s['source']},user='student_a')[0]==403
    for source in [s['source'].replace('knowledge_search','shell_exec'),'名称：重复\n'+s['source'],s['source'].replace('启用：是\n','')]:
        code,error=c.request('POST','/api/skills',{'source':source})
        assert code==400 and error['error']
if __name__=='__main__':
    from run_all import run_single
    sys.exit(run_single(__file__))
