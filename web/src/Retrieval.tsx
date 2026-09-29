import { useState } from 'react'

type Hit={chunk_id:number;material_id:number;title:string;chunk_index:number;start_char:number;end_char:number;excerpt:string;link:string;score:number}
type SearchResult={hits:Hit[];message:string}
type Answer={answer:string;citations:Hit[]}
export default function Retrieval({open}:{open:(id:number)=>void}){
 const [q,setQ]=useState(''),[mode,setMode]=useState<'keyword'|'vector'|'hybrid'>('hybrid')
 const [hits,setHits]=useState<Hit[]>([]),[answer,setAnswer]=useState<Answer|null>(null),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false)
 async function submit(kind:'search'|'ask'){
  if(!q.trim())return;setBusy(true);setNotice('');setAnswer(null)
  try{
   const url=kind==='search'?'/api/search?q='+encodeURIComponent(q.trim())+'&mode='+mode:'/api/ask'
   const res=await fetch(url,kind==='ask'?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q.trim()})}:undefined)
   if(!res.ok)throw new Error(res.status===503?'检索依赖暂时不可用':res.status===401?'请重新登录':'请求失败')
   if(kind==='search'){const data=await res.json() as SearchResult;setHits(data.hits);setNotice(data.message)}
   else{const data=await res.json() as Answer;setAnswer(data);setHits(data.citations);if(!data.citations.length)setNotice(data.answer)}
  }catch(e){setNotice((e as Error).message);setHits([])}finally{setBusy(false)}
 }
 return <section className="retrieval card"><div className="eyebrow">RETRIEVAL / 02</div><h2>检索本班知识</h2><p>从已入库材料中定位片段，或根据片段生成带出处的简短回答。</p>
  <div className="retrieval-form"><input aria-label="检索问题" placeholder="输入教材内容或问题…" value={q} onChange={e=>setQ(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')submit('search')}}/>
  <select aria-label="检索模式" value={mode} onChange={e=>setMode(e.target.value as typeof mode)}><option value="hybrid">混合检索</option><option value="keyword">关键词</option><option value="vector">向量</option></select>
  <button disabled={busy||!q.trim()} onClick={()=>submit('search')}>检索</button><button disabled={busy||!q.trim()} onClick={()=>submit('ask')}>提问</button></div>
  {notice&&<p role="status" className="notice">{notice}</p>}
  {answer&&answer.citations.length>0&&<div className="answer"><strong>简短回答</strong><p>{answer.answer}</p></div>}
  {hits.length>0&&<div className="hit-list"><div className="count">{answer?'引用出处':'命中切片'} <strong>{hits.length}</strong></div>{hits.map((h,i)=><article className="hit" key={h.chunk_id}><div className="hit-title"><span>{answer?'['+(i+1)+'] ':''}{h.title} · 切片 {h.chunk_index+1}</span><button className="quiet" onClick={()=>open(h.material_id)}>打开材料 ↗</button></div><small>字符区间 {h.start_char}–{h.end_char} · 相对分数 {h.score.toFixed(3)}</small><p>{h.excerpt}</p></article>)}</div>}
 </section>
}
