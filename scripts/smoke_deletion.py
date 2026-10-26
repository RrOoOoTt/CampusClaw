"""Run against a running Compose project. Deletes only fixtures created by this script."""
import json
import os
import uuid
from smoke import client, login, request, multipart

def data(op,path,method='GET',body=None):
    payload=None if body is None else json.dumps(body).encode()
    status,raw=request(op,path,method,payload,{'Content-Type':'application/json'})
    return status,json.loads(raw)

if __name__=='__main__':
    a,b,s,other=client(),client(),client(),client()
    for op,name in [(a,'teacher_a'),(b,'teacher_b'),(s,'student_a'),(other,'student_a')]:
        login(op,name,os.environ['SEED_'+name.upper()+'_PASSWORD'])
    body,headers=multipart('delete-test.md',('# 删除功能测试\n删除测试专用资料 '+uuid.uuid4().hex).encode())
    status,raw=request(a,'/api/materials','POST',body,headers)
    assert status==201,(status,raw)
    mid=json.loads(raw)['id'];path='/api/materials/'+str(mid)
    assert request(client(),path,'DELETE')[0]==401
    assert request(s,path,'DELETE')[0]==403
    assert request(b,path,'DELETE')[0]==404
    assert request(a,path)[0]==200
    status,result=data(a,path,'DELETE');assert status==200 and result['deleted'],(status,result)
    assert request(a,path)[0]==404
    assert request(a,path+'/file')[0]==404
    assert request(a,path+'/reindex','POST',b'{}',{'Content-Type':'application/json'})[0]==404
    assert all(m['id']!=mid for m in data(a,'/api/materials')[1])
    status,hits=data(a,'/api/search?q='+str(mid)+'&mode=hybrid')
    assert status==200 and all(h['material_id']!=mid for h in hits['hits'])
    assert request(a,path,'DELETE')[0]==404
    agents=data(s,'/api/agents')[1];assert agents, 'need enabled agent'
    status,result=data(s,'/api/assistant/conversations','POST',{'agent_id':agents[0]['id']});assert status==201
    cid=result['id'];path='/api/assistant/conversations/'+str(cid)
    assert request(client(),path,'DELETE')[0]==401
    assert request(a,path,'DELETE')[0]==404
    # Another login for the same user owns the same history; deletion is per user, not per token.
    assert request(other,path)[0]==200
    status,result=data(s,path,'DELETE');assert status==200 and result['deleted']
    assert request(s,path)[0]==404 and request(other,path)[0]==404
    assert all(c['id']!=cid for c in data(s,'/api/assistant/conversations')[1])
    assert request(s,path,'DELETE')[0]==404
    print('PASS: delete permissions, class isolation, material access/index removal, conversation ownership')
