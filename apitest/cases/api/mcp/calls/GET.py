import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    s=c.mcp_server();a=c.mcp_agent();conv=c.mcp_conversation()
    # A real model may choose tools autonomously; this question explicitly requests the tools.
    events=c.stream('请先调用单位换算工具得到米到厘米的关系，再调用公式提示工具，给三角形面积的分析提示，不要计算最终得数。',conv['id'])
    uses=events[-1][1].get('tool_calls',[]);assert any(x['status']=='ok' for x in uses),('model did not execute a tool',uses)
    first_delta=next(i for i,(name,_) in enumerate(events) if name=='delta')
    assert any(name=='tool_call' for name,_ in events[:first_delta])
    code,rows=c.request('GET','/api/mcp/calls');assert code==200
    ours=[x for x in rows if x['server_name']==s['name'] and x['username']=='student_a'];assert any(x['status']=='ok' for x in ours)
    for row in ours:assert c.scalar(f"SELECT status FROM mcp_calls WHERE id={row['id']}")==row['status']
    code,other=c.request('GET','/api/mcp/calls',user='teacher_b');assert code==200 and all(x['server_name']!=s['name'] for x in other)
    assert c.request('GET','/api/mcp/calls',user='student_a')[0]==403

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
