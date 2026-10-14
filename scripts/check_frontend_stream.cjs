const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {execFileSync}=require('node:child_process');
const web=path.resolve(__dirname,'../web');
const folder=fs.mkdtempSync(path.join(os.tmpdir(),'campusclaw-sse-'));
(async()=>{
 try{
  const tsc=require.resolve('typescript/bin/tsc',{paths:[web]});
  execFileSync(process.execPath,[tsc,path.join(web,'src/stream.ts'),'--target','es2020','--module','commonjs','--lib','es2020,dom','--skipLibCheck','--outDir',folder],{stdio:'inherit'});
  const {readSSE,parseSSEFrame}=require(path.join(folder,'stream.js'));
  assert.equal(parseSSEFrame(': heartbeat'),null);
  assert.deepEqual(parseSSEFrame('event: delta\r\ndata: first\r\ndata: second'),{event:'delta',data:'first\nsecond'});
  const wire=': heartbeat\r\n\r\nevent: meta\r\ndata: {"id":1}\r\n\r\nevent: delta\r\ndata: {"text":"你好同学"}\r\n\r\nevent: done\r\ndata: {"state":"complete"}\r\n\r\n';
  const bytes=new TextEncoder().encode(wire);let index=0;
  const body=new ReadableStream({pull(controller){if(index===bytes.length){controller.close();return}controller.enqueue(bytes.slice(index,index+1));index++}});
  const events=[];await readSSE(new Response(body),event=>events.push(event));
  assert.deepEqual(events.map(e=>e.event),['meta','delta','done']);
  assert.equal(JSON.parse(events[1].data).text,'你好同学');
  const end=[];await readSSE(new Response('event: done\ndata: {}'),e=>end.push(e));assert.equal(end.length,1);
  console.log('PASS: fragmented UTF-8/CRLF SSE, comments, multiline data, EOF');
 }finally{fs.rmSync(folder,{recursive:true,force:true})}
})().catch(error=>{console.error(error);process.exitCode=1});
