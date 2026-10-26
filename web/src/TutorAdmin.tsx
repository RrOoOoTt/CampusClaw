import { useEffect, useState } from 'react'
import { apiFetch } from './auth'
export type Agent={id:number;name:string;prompt:string;enabled:boolean;skill_ids:number[];material_ids:number[]}
type Skill={id:number;name:string;source:string;enabled:boolean}
type Material={id:number;title:string;subject:string}
export async function tutorRequest<T>(path:string,init?:RequestInit):Promise<T>{const r=await apiFetch(path,init);const body=await r.json();if(!r.ok)throw new Error(body.error||'请求失败');return body}
const template='名称：数学解题引导\n适用条件：数学，三角形，面积，方程，几何，计算\n启用：是\n工具：knowledge_search\n学科：数学\n\n核对已知条件，给出下一步提示与检查问题。引用资料，不给最终答案、选项、得数或完整解题过程。'
const empty={id:0,name:'',prompt:'',enabled:true,skill_ids:[],material_ids:[]} as Agent
export default function TutorAdmin({changed}:{changed:()=>void}){
 const [agents,setAgents]=useState<Agent[]>([]),[skills,setSkills]=useState<Skill[]>([]),[materials,setMaterials]=useState<Material[]>([])
 const [agent,setAgent]=useState<Agent>(empty),[source,setSource]=useState(template),[skillID,setSkillID]=useState(0),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false)
 async function reload(){const [a,s,m]=await Promise.all([tutorRequest<Agent[]>('/api/agents'),tutorRequest<Skill[]>('/api/skills'),tutorRequest<Material[]>('/api/materials')]);setAgents(a);setSkills(s);setMaterials(m)}
 useEffect(()=>{reload().catch(e=>setNotice(e.message))},[])
 async function action(run:()=>Promise<unknown>){setBusy(true);try{await run();await reload();changed();setNotice('已保存，下一轮提问读取最新配置')}catch(e){setNotice((e as Error).message)}finally{setBusy(false)}}
 function checked(key:'skill_ids'|'material_ids',id:number,on:boolean){setAgent({...agent,[key]:on?[...agent[key],id]:agent[key].filter(v=>v!==id)})}
 return <details className="assistant-settings card"><summary>教师管理：智能体、技能与材料绑定</summary>{notice&&<p role="status">{notice}</p>}
  <h3>智能体</h3><div className="admin-list"><button disabled={busy} onClick={()=>setAgent({...empty})}>＋ 新建智能体</button>{agents.map(a=><button disabled={busy} key={a.id} onClick={()=>setAgent(a)}>{a.name}{a.enabled?'':'（已停用）'}</button>)}</div>
  <label>名称<input maxLength={64} value={agent.name} onChange={e=>setAgent({...agent,name:e.target.value})}/></label>
  <label>角色提示词（空白使用默认角色）<textarea maxLength={2000} value={agent.prompt} onChange={e=>setAgent({...agent,prompt:e.target.value})}/></label>
  <label><input type="checkbox" checked={agent.enabled} onChange={e=>setAgent({...agent,enabled:e.target.checked})}/>启用：停用后不能新建会话，已有会话仍可提问</label>
  <fieldset><legend>绑定技能（仅候选集，至多路由一个）</legend>{skills.map(s=><label key={s.id}><input type="checkbox" checked={agent.skill_ids.includes(s.id)} onChange={e=>checked('skill_ids',s.id,e.target.checked)}/>{s.name}{s.enabled?'':'（已停用）'}</label>)}</fieldset>
  <fieldset><legend>绑定资料（新上传资料需在这里绑定）</legend>{materials.map(m=><label key={m.id}><input type="checkbox" checked={agent.material_ids.includes(m.id)} onChange={e=>checked('material_ids',m.id,e.target.checked)}/>{m.title} · {m.subject}</label>)}</fieldset>
  <button disabled={busy||!agent.name.trim()} onClick={()=>action(async()=>{const a=await tutorRequest<Agent>('/api/agents'+(agent.id?'/'+agent.id:''),{method:agent.id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(agent)});setAgent(a)})}>保存智能体及绑定</button>
  <h3>技能文件</h3><p>字段、空行、正文。适用条件用逗号分隔关键词；启用填“是”或“否”。</p>
  <div className="admin-list"><button disabled={busy} onClick={()=>{setSkillID(0);setSource(template)}}>＋ 新建技能</button>{skills.map(s=><button disabled={busy} key={s.id} onClick={()=>{setSkillID(s.id);setSource(s.source)}}>{s.name}{s.enabled?'':'（已停用）'}</button>)}</div>
  <label>读取 UTF-8 .md/.txt 文件<input type="file" accept=".txt,.md" onChange={async e=>{const file=e.target.files?.[0];if(file){if(file.size>40000){setNotice('技能文件过大');return}setSource(await file.text())}e.target.value=''}}/></label>
  <textarea aria-label="技能文件内容" rows={10} maxLength={8000} value={source} onChange={e=>setSource(e.target.value)}/>
  <button disabled={busy} onClick={()=>action(async()=>{const s=await tutorRequest<Skill>('/api/skills'+(skillID?'/'+skillID:''),{method:skillID?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({source})});setSkillID(s.id)})}>解析并保存技能</button>
  {skillID>0&&<button className="danger" disabled={busy} onClick={()=>{if(window.confirm('删除技能及其绑定？'))action(async()=>{await tutorRequest('/api/skills/'+skillID,{method:'DELETE'});setSkillID(0);setSource(template)})}}>删除技能</button>}
 </details>
}
