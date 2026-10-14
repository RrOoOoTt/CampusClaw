import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import Assistant from './Assistant'
import { apiFetch, saveToken, clearToken } from './auth'
import './style.css'

type User = { username:string; role:'teacher'|'student'; class_id:number }
type Material = { id:number; title:string; size:number; created_at:string; body?:string }
async function api<T>(path:string,init?:RequestInit):Promise<T> {
 const res=await apiFetch('/api'+path,init)
 if(!res.ok) {const e=new Error(res.status===401?'请重新登录':res.status===503?'服务暂时不可用':'操作失败');(e as Error & {status?:number}).status=res.status;throw e}
 return res.json()
}
function App(){
 const [user,setUser]=useState<User|null>(null),[ready,setReady]=useState(false)
 const [username,setUsername]=useState(''),[password,setPassword]=useState(''),[items,setItems]=useState<Material[]>([])
 const [selected,setSelected]=useState<Material|null>(null),[search,setSearch]=useState(''),[message,setMessage]=useState('')
 const [page,setPage]=useState<'assistant'|'materials'>('assistant')
 const [strategy,setStrategy]=useState<'auto'|'custom'|'hierarchy'>('auto')
 const [busy,setBusy]=useState(false),[theme,setTheme]=useState<'light'|'dark'>('light'),[view,setView]=useState<'list'|'grid'>('list')
 useEffect(()=>{api<User>('/me').then(setUser).catch(()=>{}).finally(()=>setReady(true))},[])
 useEffect(()=>{document.documentElement.dataset.theme=theme},[theme])
 useEffect(()=>{if(!user)return;let active=true;api<Material[]>('/materials?q='+encodeURIComponent(search)).then(v=>{if(active)setItems(v)}).catch(e=>{if(active)handle(e)});return()=>{active=false}},[user,search])
 function handle(e:unknown){const err=e as Error & {status?:number};if(err.status===401){setUser(null);setSelected(null)}setMessage(err.message||'操作失败')}
 async function login(e:React.FormEvent){e.preventDefault();setBusy(true);setMessage('');try{const session=await api<{access_token:string}>('/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username,password})});saveToken(session.access_token);setUser(await api<User>('/me'));setPassword('')}catch{setMessage('账号或密码错误')}finally{setBusy(false)}}
 async function logout(){try{await api('/logout',{method:'POST'});clearToken();setUser(null);setItems([]);setSelected(null)}catch(e){handle(e)}}
 async function open(id:number){try{setSelected(await api<Material>('/materials/'+id))}catch(e){handle(e)}}
 async function upload(e:React.ChangeEvent<HTMLInputElement>){const file=e.target.files?.[0];if(!file)return;setBusy(true);setMessage('上传中…');try{const fd=new FormData();fd.append('file',file);fd.append('strategy',strategy);if(strategy==='custom'){fd.append('max_chars','800');fd.append('overlap_ratio','0.1');fd.append('separator','newline')}const result=await api<{id:number}>('/materials',{method:'POST',body:fd});setItems(await api<Material[]>('/materials?q='+encodeURIComponent(search)));setMessage('已入库');await open(result.id)}catch(err){handle(err)}finally{setBusy(false);e.target.value=''}}
 async function reindex(m:Material){setBusy(true);try{const config=strategy==='custom'?{strategy,max_chars:800,overlap_ratio:0.1,separator:'newline'}:{strategy};const res=await apiFetch('/api/materials/'+m.id+'/reindex',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(config)});if(!res.ok)throw new Error(res.status===503?'索引服务暂不可用':'重建失败');setMessage('索引已重建')}catch(e){handle(e)}finally{setBusy(false)}}
 async function removeMaterial(m:Material){
  if(!window.confirm('删除“'+m.title+'”？文件和知识库索引将被删除，历史回答中的引用快照会保留。'))return
  setBusy(true)
  try{const result=await api<{cleanup_pending:boolean}>('/materials/'+m.id,{method:'DELETE'});setItems(prev=>prev.filter(v=>v.id!==m.id));setSelected(null);setMessage(result.cleanup_pending?'资料已删除，文件和向量清理将在后台重试':'资料已删除')}catch(e){handle(e)}finally{setBusy(false)}
 }
 async function download(m:Material){try{const res=await apiFetch('/api/materials/'+m.id+'/file');if(!res.ok)throw new Error(res.status===401?'请重新登录':'下载失败');const url=URL.createObjectURL(await res.blob());const a=document.createElement('a');a.href=url;a.download=m.title;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}catch(e){handle(e)}}
 if(!ready)return <main className="center">加载中…</main>
 if(!user)return <main className="center"><form className="login card" onSubmit={login}><div className="mark">C</div><h1>CampusClaw</h1><p>教研材料 · 知识库底座</p><label>账号<input value={username} onChange={e=>setUsername(e.target.value)} autoComplete="username" required/></label><label>密码<input type="password" value={password} onChange={e=>setPassword(e.target.value)} autoComplete="current-password" required/></label><button disabled={busy}>登录</button>{message&&<p role="alert" className="error">{message}</p>}</form></main>
 return <div className="shell"><header><div className="brand"><span className="mark">C</span><strong>CampusClaw</strong><span className="subtitle">教研知识库</span></div><div className="user"><span>{user.username} · {user.role==='teacher'?'教师':'学生'} · {user.class_id} 班</span><button className="quiet" onClick={()=>setTheme(theme==='light'?'dark':'light')}>{theme==='light'?'深色':'浅色'}</button><button className="quiet" onClick={logout}>退出</button></div></header><main className="workspace"><aside><div className="eyebrow">工作空间</div><button className={page==='assistant'?'nav-current':'quiet'} onClick={()=>setPage('assistant')}>✦ AI 解题助手</button><button className={page==='materials'?'nav-current':'quiet'} onClick={()=>setPage('materials')}>▣ 教研材料</button><div className="aside-note">本班材料仅对本班成员可见。知识库正文来自教师上传的文本。</div></aside><section className="content">{page==='assistant'?<Assistant role={user.role} open={open}/>:<><div className="heading"><div><div className="eyebrow">KNOWLEDGE BASE / 01</div><h1>教研材料</h1><p>浏览、检索与管理本班的教学资料</p></div>{user.role==='teacher'&&<div className="upload-controls"><select aria-label="切分策略" value={strategy} onChange={e=>setStrategy(e.target.value as typeof strategy)}><option value="auto">自动切分</option><option value="custom">自定义换行</option><option value="hierarchy">Markdown 标题</option></select><label className="upload">＋ 上传材料<input type="file" accept=".txt,.md" onChange={upload} disabled={busy} hidden/></label></div>}</div><div className="toolbar"><input aria-label="搜索本班材料" placeholder="搜索材料标题…" value={search} onChange={e=>setSearch(e.target.value)}/><div className="toggle"><button className={view==='list'?'active':''} onClick={()=>setView('list')}>列表</button><button className={view==='grid'?'active':''} onClick={()=>setView('grid')}>网格</button></div></div>{message&&<p role="status" className="notice" onClick={()=>setMessage('')}>{message} ×</p>}<div className="count">本班材料 <strong>{items.length}</strong></div><div className={'materials '+view}>{items.map(m=><button className="material card" key={m.id} onClick={()=>open(m.id)}><span className="file-icon">{m.title.toLowerCase().endsWith('.md')?'MD':'TXT'}</span><span className="material-main"><strong>{m.title}</strong><small>{new Date(m.created_at).toLocaleDateString('zh-CN')} · {(m.size/1024).toFixed(1)} KB</small></span><span className="arrow">↗</span></button>)}{items.length===0&&<div className="empty card">暂无材料。{user.role==='teacher'?'上传 .txt 或 .md 文件即可开始。':'等待本班教师上传。'}</div>}</div></>}</section></main>{selected&&<div className="overlay" onMouseDown={e=>{if(e.target===e.currentTarget)setSelected(null)}}><article className="drawer"><div className="drawer-top"><span className="eyebrow">MATERIAL / {selected.id}</span><button className="quiet" onClick={()=>setSelected(null)} aria-label="关闭">✕</button></div><h2>{selected.title}</h2><p className="meta">{new Date(selected.created_at).toLocaleString('zh-CN')} · {(selected.size/1024).toFixed(1)} KB</p><button className="download" onClick={()=>download(selected)}>下载原文件 ↓</button>{user.role==='teacher'&&<button className="quiet" disabled={busy} onClick={()=>reindex(selected)}>按当前切分策略重建索引</button>}{user.role==='teacher'&&<button className="danger" disabled={busy} onClick={()=>removeMaterial(selected)}>删除资料</button>}<div className="body"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{selected.body||''}</ReactMarkdown></div></article></div>}</div>
}
createRoot(document.getElementById('root')!).render(<App/>)
