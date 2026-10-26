import sys
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break

def run(c):
    conv=c.mcp_conversation();events=c.stream('请调用公式提示工具分析三角形面积',conv['id'])
    path=f"/api/assistant/conversations/{conv['id']}/traces"
    code,rows=c.request('GET',path,user='student_a');assert code==200 and rows
    assert c.request('GET',path,user='teacher_a')[0]==404
    assert any('tools' in row['request'] for row in rows if row['phase']=='tool_decision')
    assert any(row['phase']=='final_stream' and row['request']['stream'] for row in rows)
    for secret in [c.env.get('MCP_AUTH_TOKEN'),c.env.get('CHAT_GATEWAY_API_KEY')]:
        if secret:assert secret not in str(rows)

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
