import { useEffect, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { apiFetch } from './auth'
import { readSSE } from './stream'

type Citation={chunk_id:number;material_id:number;title:string;chunk_index:number;start_char:number;end_char:number;excerpt:string}
type Settings={prompt:string;guidance_enabled:boolean}
type Conversation={id:number;title:string;updated_at:string;busy:boolean}
type Message={id:number;role:'user'|'assistant';content:string;citations:Citation[];guidance_enabled:boolean;state:string;prompt_snapshot:string}
async function jsonRequest<T>(path:string,init?:RequestInit):Promise<T>{
 const response=await apiFetch(path,init)
 if(!response.ok)throw new Error(response.status===401?'请退出并重新登录':response.status===403?'只有本班教师可以修改配置':response.status===503?'知识库或模型服务暂不可用':response.status===409?'会话正在生成或已达上限，请稍后刷新或新建会话':'请求失败')
 return response.json()
}
export default function Assistant({role,open}:{role:'teacher'|'student';open:(id:number)=>void}){
 const [settings,setSettings]=useState<Settings>({prompt:'',guidance_enabled:true})
 const [savedSettings,setSavedSettings]=useState<Settings|null>(null)
 const [conversations,setConversations]=useState<Conversation[]>([]),[current,setCurrent]=useState<number|null>(null)
 const [messages,setMessages]=useState<Message[]>([]),[question,setQuestion]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[saving,setSaving]=useState(false)
 const controller=useRef<AbortController|null>(null),bottom=useRef<HTMLDivElement|null>(null)
 useEffect(()=>{
  let active=true
  jsonRequest<Settings>('/api/assistant/settings').then(s=>{if(active){setSettings(s);setSavedSettings(s)}}).catch(e=>{if(active)setNotice(e.message)})
  jsonRequest<Conversation[]>('/api/assistant/conversations').then(c=>{if(active)setConversations(c)}).catch(e=>{if(active)setNotice(e.message)})
  return()=>{active=false;controller.current?.abort()}
 },[])
 useEffect(()=>{bottom.current?.scrollIntoView({block:'nearest',behavior:'smooth'})},[messages])
 async function refreshList(){setConversations(await jsonRequest<Conversation[]>('/api/assistant/conversations'))}
 async function load(id:number){
  try{const data=await jsonRequest<{messages:Message[]}>('/api/assistant/conversations/'+id);setCurrent(id);setMessages(data.messages);setNotice('')}
  catch(e){setNotice((e as Error).message)}
 }
 async function removeConversation(c:Conversation){
  if(busy||!window.confirm('删除“'+c.title+'”及全部问答历史？此操作无法撤销。'))return
  setSaving(true)
  try{await jsonRequest('/api/assistant/conversations/'+c.id,{method:'DELETE'});setConversations(prev=>prev.filter(v=>v.id!==c.id));if(current===c.id){setCurrent(null);setMessages([]);setQuestion('')};setNotice('会话已删除')}
  catch(e){setNotice((e as Error).message)}finally{setSaving(false)}
 }
 async function saveSettings(){
  setSaving(true);setNotice('')
  try{const s=await jsonRequest<Settings>('/api/assistant/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(settings)});setSettings(s);setSavedSettings(s);setNotice('已保存，从下一轮回答生效')}
  catch(e){setNotice((e as Error).message)}finally{setSaving(false)}
 }
 async function send(){
  const text=question.trim();if(!text||busy||saving)return
  setBusy(true);setNotice('');let id=current;let finished=false;const tempUser=-Date.now()*2,tempAssistant=tempUser-1;let replyID=tempAssistant
  const abort=new AbortController();controller.current=abort
  try{
   if(id===null){const created=await jsonRequest<{id:number}>('/api/assistant/conversations',{method:'POST'});id=created.id;setCurrent(id)}
   const activeSettings=await jsonRequest<Settings>('/api/assistant/settings');setSavedSettings(activeSettings)
   setMessages(prev=>[...prev,{id:tempUser,role:'user',content:text,citations:[],state:'complete',guidance_enabled:activeSettings.guidance_enabled,prompt_snapshot:activeSettings.prompt},{id:tempAssistant,role:'assistant',content:'',citations:[],state:'streaming',guidance_enabled:activeSettings.guidance_enabled,prompt_snapshot:activeSettings.prompt}]);setQuestion('')
   const response=await apiFetch('/api/assistant/conversations/'+id+'/messages',{method:'POST',headers:{'Content-Type':'application/json','Accept':'text/event-stream'},body:JSON.stringify({question:text}),signal:abort.signal})
   if(!response.ok)throw new Error(response.status===401?'请退出并重新登录':response.status===409?'会话正在生成或已达上限，请刷新或新建会话':response.status===503?'知识库或模型服务暂不可用':'发送失败')
   if(!response.headers.get('content-type')?.startsWith('text/event-stream'))throw new Error('服务器未返回流式回答')
   await readSSE(response,event=>{
    const data=JSON.parse(event.data)
    if(event.event==='meta'){
     replyID=data.assistant_message_id
     setMessages(prev=>prev.map(m=>m.id===tempUser?{...m,id:data.user_message_id}:m.id===tempAssistant?{...m,id:replyID,citations:data.citations,guidance_enabled:data.guidance_enabled,prompt_snapshot:data.prompt_snapshot}:m))
    }else if(event.event==='delta'){
     setMessages(prev=>prev.map(m=>m.id===replyID?{...m,content:m.content+data.text}:m))
    }else if(event.event==='done'){
     finished=true;setMessages(prev=>prev.map(m=>m.id===data.message_id?{...m,content:data.content,citations:data.citations,state:'complete'}:m))
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
  <div className="assistant-heading"><div><div className="eyebrow">AI ASSISTANT</div><h1>本班解题助手</h1><p>连续追问、逐步理解，并回到本班材料核对依据。</p></div><span className="skill-badge">解题引导：{savedSettings?.guidance_enabled?'开启':'关闭'}</span></div>
  {role==='teacher'&&<details className="assistant-settings card"><summary>本班助手配置 · 仅教师可修改</summary><label>助手角色提示词<textarea maxLength={2000} value={settings.prompt} onChange={e=>setSettings({...settings,prompt:e.target.value})}/></label><label className="skill-switch"><input type="checkbox" checked={settings.guidance_enabled} onChange={e=>setSettings({...settings,guidance_enabled:e.target.checked})}/>启用解题引导：给思路和提示，不直接给最终答案</label><button disabled={saving||!settings.prompt.trim()} onClick={saveSettings}>{saving?'保存中…':'保存，下轮生效'}</button></details>}
  {notice&&<p className="notice" role="status">{notice}</p>}
  <div className="assistant-layout"><aside className="conversation-list card"><button disabled={busy} onClick={()=>{setCurrent(null);setMessages([]);setNotice('')}}>＋ 新会话</button><small>仅显示你自己的会话</small>{conversations.map(c=><div className="conversation-row" key={c.id}><button disabled={busy||saving} className={current===c.id?'selected':''} onClick={()=>load(c.id)}>{c.title}{c.busy?' · 生成中':''}</button><button className="danger" aria-label={'删除会话：'+c.title} disabled={busy||saving||c.busy} onClick={()=>removeConversation(c)}>删除</button></div>)}</aside>
   <div className="chat-panel card"><div className="chat-top"><strong>{current?'会话 #'+current:'新会话'}</strong>{current&&<button className="quiet" disabled={busy} onClick={()=>load(current)}>刷新历史</button>}</div><div className="chat-messages" aria-live="polite">
    {messages.length===0&&<div className="empty">输入题目或学习问题。助手会先读取教师配置，再查找本班材料。<br/>引导开启时，你会收到思路和下一步提示。</div>}
    {messages.map(m=><article className={'chat-message '+m.role} key={m.id}><div className="message-label">{m.role==='user'?'你':'学习助手'}{m.role==='assistant'&&<small> · {m.guidance_enabled?'解题引导':'直接回答'}{m.state==='streaming'?' · 生成中':m.state==='complete'?'':' · 未完成'}</small>}</div><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{m.content||(m.role==='assistant'?'正在检索本班资料…':'')}</ReactMarkdown>
     {m.role==='assistant'&&m.citations.length>0&&<div className="chat-citations"><strong>本轮材料依据</strong>{m.citations.map((h,i)=><details key={h.chunk_id}><summary>[{i+1}] {h.title} · 切片 {h.chunk_index+1} · 字符 {h.start_char}–{h.end_char}</summary><p>{h.excerpt}</p><button className="quiet" onClick={()=>open(h.material_id)}>打开原材料 ↗</button></details>)}</div>}
    </article>)}<div ref={bottom}/></div>
    <form className="chat-composer" onSubmit={e=>{e.preventDefault();send()}}><textarea aria-label="向助手提问" placeholder="输入题目，也可以接着上一轮追问…" maxLength={500} value={question} onChange={e=>setQuestion(e.target.value)} disabled={busy}/><div><small>{question.length}/500 · 仅检索本班资料</small>{busy?<button type="button" onClick={()=>controller.current?.abort()}>停止生成</button>:<button disabled={!question.trim()} type="submit">发送问题 ↑</button>}</div></form>
   </div></div>
 </section>
}
