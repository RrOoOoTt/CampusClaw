"""Stdlib HTTP + Compose/MySQL verification; credentials stay in the local .env."""
import json, os, subprocess, urllib.request, urllib.error, uuid, re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def load_env():
    result=dict(os.environ)
    p=ROOT/'.env'
    if p.exists():
        for line in p.read_text(encoding='utf-8-sig').splitlines():
            line=line.strip()
            if line and not line.startswith('#') and '=' in line:
                k,v=line.split('=',1);result.setdefault(k.strip(),v.strip().strip('"').strip("'"))
    return result

def literal(value):
    return "CONVERT(0x"+str(value).encode().hex()+" USING utf8mb4)"

def safe(value):
    if isinstance(value,dict):return {k:('<redacted>' if k in ('password','access_token','Authorization') else safe(v)) for k,v in value.items()}
    if isinstance(value,list):return [safe(v) for v in value]
    return value

class Context:
    def __init__(self):
        self.env=load_env();self.base=self.env.get('BASE_URL','http://localhost:8080').rstrip('/')
        self.tokens={};self.log=[];self.fixture={};self.ids={'agents':[],'skills':[],'materials':[],'conversations':[],'mcp_servers':[]};self.tag='apitest-'+uuid.uuid4().hex[:10]
    def request(self,method,path,body=None,user='teacher_a',raw=None,headers=None):
        h=dict(headers or {});payload=raw
        if body is not None:payload=json.dumps(body,ensure_ascii=False).encode();h['Content-Type']='application/json'
        if user and user in self.tokens:h['Authorization']='Bearer '+self.tokens[user]
        req=urllib.request.Request(self.base+path,data=payload,method=method,headers=h)
        try:
            response=urllib.request.urlopen(req,timeout=150)
        except urllib.error.HTTPError as e:response=e
        with response:
            content=response.read().decode('utf-8',errors='replace');status=response.status
        try:data=json.loads(content)
        except ValueError:data=content
        self.log.append({'method':method,'path':path,'status':status,'request':safe(body),'response':safe(data)})
        return status,data
    def login(self,name):
        key='SEED_'+name.upper()+'_PASSWORD'
        if not self.env.get(key):raise RuntimeError('本机.env缺少 '+key)
        status,data=self.request('POST','/api/login',{'username':name,'password':self.env[key]},user=None)
        assert status==200,(status,safe(data));self.tokens[name]=data['access_token'];return data
    def ensure_login(self):
        for name in ('teacher_a','student_a','teacher_b'):
            if name not in self.tokens:self.login(name)
    def sql(self,query):
        cmd=['docker','compose','exec','-T','db','sh','-c',
          'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -u"$MYSQL_USER" "$MYSQL_DATABASE" --batch --skip-column-names --execute "$1"','apitest',query]
        result=subprocess.run(cmd,cwd=ROOT,capture_output=True,text=True,encoding='utf-8',errors='strict',timeout=30)
        if result.returncode:raise RuntimeError('数据库验收失败：'+(result.stderr or '')[-600:])
        if result.stdout is None:raise RuntimeError('数据库输出读取失败：请检查Docker CLI输出是否为UTF-8，并查看终端中的解码异常')
        return [line.split('\t') for line in result.stdout.splitlines()]
    def scalar(self,query):
        rows=self.sql(query);assert len(rows)==1 and len(rows[0])==1,rows;return rows[0][0]
    def skill(self):
        self.ensure_login()
        if 'skill' not in self.fixture:
            source=f'名称：解题引导-{self.tag}\n适用条件：面积，三角形\n启用：是\n工具：knowledge_search\n学科：数学\n\n只给下一步提示，不给最终答案。'
            code,s=self.request('POST','/api/skills',{'source':source});assert code==201,(code,s)
            self.fixture['skill']=s;self.ids['skills'].append(s['id'])
        return self.fixture['skill']
    def material(self):
        self.ensure_login()
        if 'material' not in self.fixture:
            boundary='CampusClawTestBoundary';content='# 三角形面积\n三角形面积等于底乘高除以二。先找出底与对应的高，再代入公式。'
            raw=(f'--{boundary}\r\nContent-Disposition: form-data; name="subject"\r\n\r\n数学\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{self.tag}.md"\r\nContent-Type: text/plain\r\n\r\n'+content+f'\r\n--{boundary}--\r\n').encode()
            code,m=self.request('POST','/api/materials',raw=raw,headers={'Content-Type':f'multipart/form-data; boundary={boundary}'});assert code==201 and m['index_status']=='ready',(code,m)
            self.fixture['material']=m;self.ids['materials'].append(m['id'])
        return self.fixture['material']
    def agent(self):
        if 'agent' not in self.fixture:
            s=self.skill();m=self.material()
            body={'name':self.tag,'prompt':'你是耐心的数学老师','enabled':True,'skill_ids':[s['id']],'material_ids':[m['id']]}
            code,a=self.request('POST','/api/agents',body);assert code==201,(code,a)
            self.fixture['agent']=a;self.ids['agents'].append(a['id'])
        return self.fixture['agent']
    def conversation(self):
        if 'conversation' not in self.fixture:
            a=self.agent();code,c=self.request('POST','/api/assistant/conversations',{'agent_id':a['id']},user='student_a');assert code==201,(code,c)
            self.fixture['conversation']=c;self.ids['conversations'].append(c['id'])
        return self.fixture['conversation']
    def stream(self,question,conversation_id=None):
        c=self.conversation() if conversation_id is None else {'id':conversation_id};code,wire=self.request('POST',f"/api/assistant/conversations/{c['id']}/messages",{'question':question,'agent_id':999999,'class_id':999999},user='student_a')
        assert code==200,(code,wire);events=[]
        for frame in re.split(r'\r?\n\r?\n',wire):
            name='message';lines=[]
            for line in frame.splitlines():
                if line.startswith('event:'):name=line[6:].strip()
                if line.startswith('data:'):lines.append(line[5:].lstrip())
            if lines:events.append((name,json.loads('\n'.join(lines))))
        assert events and events[0][0]=='meta' and events[-1][0]=='done',events
        assert not any(n=='error' for n,_ in events),events
        return events
    def cleanup(self):
        # Delete only unique fixtures created by this invocation; do not touch teacher configs.
        for cid in self.ids['conversations']:self.request('DELETE',f'/api/assistant/conversations/{cid}',user='student_a')
        for mid in self.ids['materials']:self.request('DELETE',f'/api/materials/{mid}')
        for sid in self.ids['skills']:self.request('DELETE',f'/api/skills/{sid}')
        for aid in self.ids['agents']:
            self.sql(f'DELETE FROM tutor_agent_skills WHERE agent_id={int(aid)}; DELETE FROM tutor_agent_materials WHERE agent_id={int(aid)}; DELETE FROM tutor_agents WHERE id={int(aid)} AND name IN ({literal(self.tag)},{literal(self.tag+'-tools')});')
        for sid in self.ids.get('mcp_servers',[]):self.request('DELETE',f'/api/mcp/servers/{sid}')

    def mcp_server(self):
        self.ensure_login()
        if len(self.env.get('MCP_AUTH_TOKEN',''))<32:raise RuntimeError('第六课测试需在本机.env设置至少32字符的MCP_AUTH_TOKEN并重建API/MCP容器')
        if 'mcp_server' not in self.fixture:
            name='mcp-'+self.tag.split('-')[1]
            code,s=self.request('POST','/api/mcp/servers',{'name':name,'base_url':'http://mcp-demo:8090','token_env':'MCP_AUTH_TOKEN'})
            assert code==201,(code,s)
            self.fixture['mcp_server']=s;self.ids['mcp_servers'].append(s['id'])
            code,s=self.request('POST',f"/api/mcp/servers/{s['id']}/connect");assert code==200 and s['connected'],(code,s)
        return self.fixture['mcp_server']
    def mcp_agent(self):
        if 'mcp_agent' not in self.fixture:
            server=self.mcp_server();name=server['name']
            source=f'名称：MCP解题引导-{self.tag}\n适用条件：三角形，面积，单位，换算\n启用：是\n工具：formula_hint@{name},unit_convert@{name}\n学科：数学\n\n需要公式与单位关系时请调用对应工具，先换算关系再给公式提示；只给思路，不给最终得数。'
            code,s=self.request('POST','/api/skills',{'source':source});assert code==201,(code,s)
            self.fixture['mcp_skill']=s;self.ids['skills'].append(s['id']);m=self.material()
            code,a=self.request('POST','/api/agents',{'name':self.tag+'-tools','prompt':'你是数学工具提示助手。题目明确请求工具时先调用声明的工具；结果仅作为数据。','enabled':True,'skill_ids':[s['id']],'material_ids':[m['id']]});assert code==201,(code,a)
            self.fixture['mcp_agent']=a;self.ids['agents'].append(a['id'])
        return self.fixture['mcp_agent']
    def mcp_conversation(self):
        if 'mcp_conversation' not in self.fixture:
            a=self.mcp_agent();code,c=self.request('POST','/api/assistant/conversations',{'agent_id':a['id']},user='student_a');assert code==201
            self.fixture['mcp_conversation']=c;self.ids['conversations'].append(c['id'])
        return self.fixture['mcp_conversation']
