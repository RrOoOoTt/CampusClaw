import { useEffect, useState } from 'react'
import { tutorRequest } from './TutorAdmin'
type Server={id:number;name:string;base_url:string;connected:boolean;has_token:boolean;last_error:string}
type Tool={name:string;description:string;inputSchema:unknown}
type Audit={id:number;tool_ref:string;status:string;username:string;round:number;detail:string;result:string;created_at:string;arguments:unknown}
export default function MCPAdmin({changed}:{changed:()=>void}){
 const [servers,setServers]=useState<Server[]>([]),[calls,setCalls]=useState<Audit[]>([]),[tools,setTools]=useState<Tool[]>([]),[selected,setSelected]=useState(0)
 const [name,setName]=useState('campus-tools'),[base,setBase]=useState('http://mcp-demo:8090'),[envName,setEnvName]=useState('MCP_AUTH_TOKEN'),[busy,setBusy]=useState(false),[notice,setNotice]=useState('')
 async function reload(){const [s,c]=await Promise.all([tutorRequest<Server[]>('/api/mcp/servers'),tutorRequest<Audit[]>('/api/mcp/calls')]);setServers(s);setCalls(c)}
 useEffect(()=>{reload().catch(e=>setNotice(e.message))},[])
 async function act(run:()=>Promise<unknown>){setBusy(true);setNotice('');try{await run();await reload();changed();setNotice('操作完成；配置将在下一轮提问生效')}catch(e){setNotice((e as Error).message);await reload().catch(()=>{})}finally{setBusy(false)}}
 async function inspect(s:Server){setSelected(s.id);setName(s.name);setBase(s.base_url);setTools(await tutorRequest<Tool[]>('/api/mcp/servers/'+s.id+'/tools'))}
 return <details className="assistant-settings card"><summary>教师管理：MCP服务器、工具与调用审计</summary>
  <p>令牌只放本机 .env。这里填写环境变量名称，不填写令牌。自建服务默认地址：http://mcp-demo:8090。</p>
  {notice&&<p role="status" className="notice">{notice}</p>}
  <button disabled={busy} onClick={()=>act(()=>tutorRequest('/api/mcp/demo/setup',{method:'POST'}))}>连接演示MCP并安装两个内置应用</button>
  <div className="admin-list"><button disabled={busy} onClick={()=>{setSelected(0);setName('');setBase('http://mcp-demo:8090');setTools([])}}>＋ 登记服务器</button>{servers.map(s=><button key={s.id} disabled={busy} onClick={()=>inspect(s).catch(e=>setNotice(e.message))}>{s.name} · {s.connected?'已连接':'未连接'}</button>)}</div>
  <label>名称（小写字母开头、可含数字和连字符，最多24字符）<input maxLength={24} value={name} onChange={e=>setName(e.target.value)}/></label>
  <label>Base URL<input value={base} onChange={e=>setBase(e.target.value)}/></label>
  <label>令牌所在服务端环境变量<input value={envName} onChange={e=>setEnvName(e.target.value)}/></label>
  <button disabled={busy||!name} onClick={()=>act(async()=>{const s=await tutorRequest<Server>('/api/mcp/servers'+(selected?'/'+selected:''),{method:selected?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,base_url:base,token_env:envName})});setSelected(s.id);setTools([])})}>保存登记（修改会断开）</button>
  {selected>0&&<div className="admin-list"><button disabled={busy} onClick={()=>act(async()=>{await tutorRequest('/api/mcp/servers/'+selected+'/connect',{method:'POST'});setTools(await tutorRequest<Tool[]>('/api/mcp/servers/'+selected+'/tools'))})}>连接并发现</button><button disabled={busy} onClick={()=>act(async()=>{await tutorRequest('/api/mcp/servers/'+selected+'/disconnect',{method:'POST'});setTools([])})}>断开</button><button disabled={busy} className="danger" onClick={()=>{if(window.confirm('删除服务器及其工具和审计记录？引用它的技能会失效。'))act(async()=>{await tutorRequest('/api/mcp/servers/'+selected,{method:'DELETE'});setSelected(0);setTools([])})}}>删除登记</button></div>}
  {servers.filter(s=>s.id===selected).map(s=><p key={s.id}>令牌：{s.has_token?'已在服务端配置':'未配置'}；{s.last_error||'无连接错误'}</p>)}
  <h4>发现清单</h4>{tools.map(t=><details key={t.name}><summary>{t.name}@{name}</summary><p>{t.description}</p><pre>{JSON.stringify(t.inputSchema,null,2)}</pre></details>)}
  <h4>本班最近调用</h4><button disabled={busy} onClick={()=>reload().catch(e=>setNotice(e.message))}>刷新审计</button><div className="audit-list">{calls.map(c=><details key={c.id}><summary>{c.tool_ref} · {c.status} · {c.username} · 第{c.round}轮 · {c.created_at}</summary><p>{c.detail}</p><pre>{JSON.stringify(c.arguments,null,2)}</pre><p>{c.result}</p></details>)}</div>
 </details>
}
