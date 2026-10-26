import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    a=c.agent();s=c.skill()
    try:
        events=c.stream('三角形面积该怎么分析？');meta=events[0][1];done=events[-1][1];a=c.agent()
        assert meta['agent_id']==a['id'] and meta['selected_skill']['id']==c.skill()['id']
        assert meta['citations'] and all(h['material_id'] in a['material_ids'] for h in meta['citations'])
        assert sum(n=='delta' for n,_ in events)>1
        assert done['content'] and done['state']=='complete'
        assert c.scalar(f"SELECT state FROM assistant_messages WHERE id={done['message_id']}")=='complete'
        # HEX avoids mysql CLI escaping of newlines and tabs.
        assert bytes.fromhex(c.scalar(f"SELECT HEX(content) FROM assistant_messages WHERE id={done['message_id']}" )).decode()==done['content']
        s=c.skill();assert c.request('PUT',f"/api/skills/{s['id']}",{'source':s['source'].replace('启用：是','启用：否')})[0]==200
        second=c.stream('三角形面积该怎么分析？');assert second[0][1]['selected_skill'] is None
        assert c.request('PUT',f"/api/skills/{s['id']}",{'source':s['source']})[0]==200
        # Agent disabled after conversation creation: existing conversation remains usable.
        assert c.request('PUT',f"/api/agents/{a['id']}",dict(a,enabled=False))[0]==200
        third=c.stream('语文古诗应怎么理解？');assert third[0][1]['citations']==[] and third[0][1]['selected_skill'] is None
        assert not re.search(r'\[\d+\]',third[-1][1]['content'])
        assert third[-1][1]['content']!='资料中未找到相关内容'
        assert c.request('PUT',f"/api/agents/{a['id']}",a)[0]==200
    finally:
        code,result=c.request('PUT',f"/api/skills/{s['id']}",{'source':s['source']})
        assert code==200,('restore skill failed',code,result)
        code,result=c.request('PUT',f"/api/agents/{a['id']}",a)
        assert code==200,('restore agent failed',code,result)

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
