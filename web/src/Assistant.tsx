import { useEffect, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { apiFetch } from './auth'
import { readSSE } from './stream'
import MCPAdmin from './MCPAdmin'
import TutorAdmin, { Agent, tutorRequest } from './TutorAdmin'

type Citation={chunk_id:number;material_id:number;title:string;chunk_index:number;start_char:number;end_char:number;excerpt:string}
type Settings={prompt:string;guidance_enabled:boolean}
type Conversation={id:number;title:string;updated_at:string;busy:boolean;agent_id:number}
type ToolUse={tool_ref:string;status:string;round:number;detail:string;result?:string}
type Message={id:number;role:'user'|'assistant';content:string;citations:Citation[];guidance_enabled:boolean;state:string;prompt_snapshot:string;tool_calls?:ToolUse[]}
async function jsonRequest<T>(path:string,init?:RequestInit):Promise<T>{
 const response=await apiFetch(path,init)
 if(!response.ok){const error=await response.json().catch(()=>({}));throw new Error(response.status===401?'请退出并重新登录':response.status===409?'会话正在生成或已达上限，请稍后刷新或新建会话':error.error||'请求失败')}
 return response.json()
}
export default function Assistant({role,open}:{role:'teacher'|'student';open:(id:number)=>void}){
 const [agents,setAgents]=useState<Agent[]>([]),[chosenAgent,setChosenAgent]=useState<number>(0)
 const [requestEvidence,setRequestEvidence]=useState<unknown>(null)
 const [savedSettings,setSavedSettings]=useState<Settings|null>(null)
 const [conversations,setConversations]=useState<Conversation[]>([]),[current,setCurrent]=useState<number|null>(null)
 const [messages,setMessages]=useState<Message[]>([]),[question,setQuestion]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[saving,setSaving]=useState(false)
 const controller=useRef<AbortController|null>(null),bottom=useRef<HTMLDivElement|null>(null)
 useEffect(()=>{
  let active=true
  tutorRequest<Agent[]>('/api/agents').then(a=>{if(active){setAgents(a.filter(v=>v.enabled));setChosenAgent(a.find(v=>v.enabled)?.id||0)}}).catch(e=>{if(active)setNotice(e.message)})
  jsonRequest<Conversation[]>('/api/assistant/conversations').then(c=>{if(active)setConversations(c)}).catch(e=>{if(active)setNotice(e.message)})
  return()=>{active=false;controller.current?.abort()}
 },[])
 useEffect(()=>{bottom.current?.scrollIntoView({block:'nearest',behavior:'smooth'})},[messages])
 async function refreshList(){setConversations(await jsonRequest<Conversation[]>('/api/assistant/conversations'))}
 async function load(id:number){
  try{const data=await jsonRequest<{messages:Message[];conversation:Conversation}>('/api/assistant/conversations/'+id);setRequestEvidence(null);setCurrent(id);setChosenAgent(data.conversation.agent_id);setMessages(data.messages);setNotice('')}
  catch(e){setNotice((e as Error).message)}
 }
 async function removeConversation(c:Conversation){
  if(busy||!window.confirm('删除“'+c.title+'”及全部问答历史？此操作无法撤销。'))return
  setSaving(true)
  try{await jsonRequest('/api/assistant/conversations/'+c.id,{method:'DELETE'});setConversations(prev=>prev.filter(v=>v.id!==c.id));if(current===c.id){setCurrent(null);setMessages([]);setQuestion('')};setNotice('会话已删除')}
  catch(e){setNotice((e as Error).message)}finally{setSaving(false)}
 }
 async function send(){
  const text=question.trim();if(text.length<2||busy||saving||current===null&&!chosenAgent)return
  setBusy(true);setNotice('');setRequestEvidence(null);let id=current;let finished=false;const tempUser=-Date.now()*2,tempAssistant=tempUser-1;let replyID=tempAssistant
  const abort=new AbortController();controller.current=abort
  try{
   if(id===null){const created=await jsonRequest<{id:number}>('/api/assistant/conversations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({agent_id:chosenAgent})});id=created.id;setCurrent(id)}
   const activeSettings={prompt:'',guidance_enabled:false};setSavedSettings(activeSettings)
   setMessages(prev=>[...prev,{id:tempUser,role:'user',content:text,citations:[],state:'complete',guidance_enabled:activeSettings.guidance_enabled,prompt_snapshot:activeSettings.prompt},{id:tempAssistant,role:'assistant',content:'',citations:[],state:'streaming',guidance_enabled:activeSettings.guidance_enabled,prompt_snapshot:activeSettings.prompt}]);setQuestion('')
   const response=await apiFetch('/api/assistant/conversations/'+id+'/messages',{method:'POST',headers:{'Content-Type':'application/json','Accept':'text/event-stream'},body:JSON.stringify({question:text}),signal:abort.signal})
   if(!response.ok)throw new Error(response.status===401?'请退出并重新登录':response.status===409?'会话正在生成或已达上限，请刷新或新建会话':response.status===503?'知识库或模型服务暂不可用':'发送失败')
   if(!response.headers.get('content-type')?.startsWith('text/event-stream'))throw new Error('服务器未返回流式回答')
   await readSSE(response,event=>{
    const data=JSON.parse(event.data)
    if(event.event==='meta'){
     replyID=data.assistant_message_id
     setSavedSettings({prompt:data.prompt_snapshot,guidance_enabled:data.guidance_enabled})
     setMessages(prev=>prev.map(m=>m.id===tempUser?{...m,id:data.user_message_id}:m.id===tempAssistant?{...m,id:replyID,citations:data.citations,tool_calls:data.tool_calls||[],guidance_enabled:data.guidance_enabled,prompt_snapshot:data.prompt_snapshot}:m))
    }else if(event.event==='tool_call'){
     setMessages(prev=>prev.map(m=>m.id===replyID?{...m,tool_calls:[...(m.tool_calls||[]).filter(c=>!(c.tool_ref===data.tool_ref&&c.round===data.round&&c.status==='pending')),data]}:m))
    }else if(event.event==='delta'){
     setMessages(prev=>prev.map(m=>m.id===replyID?{...m,content:m.content+data.text}:m))
    }else if(event.event==='done'){
     finished=true;setMessages(prev=>prev.map(m=>m.id===data.message_id?{...m,content:data.content,citations:data.citations,state:'complete',tool_calls:data.tool_calls||m.tool_calls}:m))
    }else if(event.event==='error')throw new Error(data.message)
   })
   if(!finished)throw new Error('连接提前结束，已保存部分回答')
  }catch(e){
   setNotice(abort.signal.aborted?'已停止生成，稍后可刷新会话查看保存的部分内容':(e as Error).message)
   setMessages(prev=>prev.map(m=>m.state==='streaming'?{...m,state:'interrupted'}:m))
   if(!finished)setQuestion(text)
  }finally{
   setBusy(false);controller.current=null
   try{await refreshList()}catch{/* Keep current conversation visible during a network interruption. */}
  }
 }
 return <section className="assistant-area">
  <div className="assistant-heading"><div><div className="eyebrow">AI ASSISTANT</div><h1>本班解题助手</h1><p>选定智能体后连续追问，只检索它绑定的本班材料。</p></div><span className="skill-badge">最近一轮：{savedSettings?.guidance_enabled?'解题引导':'直接回答'}</span></div>
  {role==='teacher'&&<TutorAdmin changed={()=>{tutorRequest<Agent[]>('/api/agents').then(a=>{setAgents(a.filter(v=>v.enabled));if(!a.some(v=>v.id===chosenAgent&&v.enabled))setChosenAgent(a.find(v=>v.enabled)?.id||0)}).catch(e=>setNotice(e.message))}}/>}
  {role==='teacher'&&<MCPAdmin changed={()=>tutorRequest<Agent[]>('/api/agents').then(a=>{setAgents(a.filter(v=>v.enabled));if(!chosenAgent)setChosenAgent(a.find(v=>v.enabled)?.id||0)}).catch(e=>setNotice(e.message))}/>}
  {notice&&<p className="notice" role="status">{notice}</p>}
  {current===null&&<label>选择本班已启用的智能体<select value={chosenAgent} onChange={e=>setChosenAgent(Number(e.target.value))} disabled={busy}><option value={0}>请选择</option>{agents.map(a=><option key={a.id} value={a.id}>{a.name}</option>)}</select>{agents.length===0&&<p>请本班教师先创建并启用智能体。</p>}</label>}
  <div className="assistant-layout"><aside className="conversation-list card"><button disabled={busy} onClick={()=>{setCurrent(null);setMessages([]);setRequestEvidence(null);setNotice('')}}>＋ 新会话</button><small>仅显示你自己的会话</small>{conversations.map(c=><div className="conversation-row" key={c.id}><button disabled={busy||saving} className={current===c.id?'selected':''} onClick={()=>load(c.id)}>{c.title}{c.busy?' · 生成中':''}</button><button className="danger" aria-label={'删除会话：'+c.title} disabled={busy||saving||c.busy} onClick={()=>removeConversation(c)}>删除</button></div>)}</aside>
   <div className="chat-panel card"><div className="chat-top"><strong>{current?'会话 #'+current:'新会话'}</strong>{current&&<button className="quiet" disabled={busy} onClick={()=>load(current)}>刷新历史</button>}</div>{current&&<div className="trace-view"><button className="quiet" disabled={busy} onClick={()=>jsonRequest('/api/assistant/conversations/'+current+'/traces').then(setRequestEvidence).catch(e=>setNotice(e.message))}>查看模型请求参数与返回</button>{requestEvidence!==null&&<details open><summary>仅本人可见 · 不含认证密钥<button className="quiet" onClick={()=>setRequestEvidence(null)}>关闭</button></summary><pre>{JSON.stringify(requestEvidence,null,2)}</pre></details>}</div>}<div className="chat-messages" aria-live="polite">
    {messages.length===0&&<div className="empty">输入题目或学习问题。助手会读取智能体配置、选择绑定技能并检索绑定资料。<br/>引导开启时，你会收到思路和下一步提示。</div>}
    {messages.map(m=><article className={'chat-message '+m.role} key={m.id}><div className="message-label">{m.role==='user'?'你':'学习助手'}{m.role==='assistant'&&<small> · {m.guidance_enabled?'解题引导':'直接回答'}{m.state==='streaming'?' · 生成中':m.state==='complete'?'':' · 未完成'}</small>}</div>{m.role==='assistant'&&(m.tool_calls||[]).length>0&&<div className="tool-uses">{m.tool_calls?.map((c,i)=><details key={i}><summary>工具：{c.tool_ref} · {c.status==='pending'?'调用中':c.status==='ok'?'成功':c.status==='denied'?'被拒':'出错'} · 第{c.round}轮</summary><p>{c.detail}</p>{c.result&&<p>{c.result}</p>}</details>)}</div>}<ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{m.content||(m.role==='assistant'?'正在检索本班资料…':'')}</ReactMarkdown>
     {m.role==='assistant'&&m.citations.length>0&&<div className="chat-citations"><strong>本轮材料依据</strong>{m.citations.map((h,i)=><details key={h.chunk_id}><summary>[{i+1}] {h.title} · 切片 {h.chunk_index+1} · 字符 {h.start_char}–{h.end_char}</summary><p>{h.excerpt}</p><button className="quiet" onClick={()=>open(h.material_id)}>打开原材料 ↗</button></details>)}</div>}
    </article>)}<div ref={bottom}/></div>
    <form className="chat-composer" onSubmit={e=>{e.preventDefault();send()}}><textarea aria-label="向助手提问" placeholder="输入题目，也可以接着上一轮追问…" maxLength={500} value={question} onChange={e=>setQuestion(e.target.value)} disabled={busy}/><div><small>{question.length}/500 · 仅检索本班资料</small>{busy?<button type="button" onClick={()=>controller.current?.abort()}>停止生成</button>:<button disabled={question.trim().length<2||current===null&&!chosenAgent} type="submit">发送问题 ↑</button>}</div></form>
   </div></div>
 </section>
}
