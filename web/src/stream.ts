export type SSEEvent={event:string;data:string}
export function parseSSEFrame(frame:string):SSEEvent|null{
 let event='message';const data:string[]=[]
 for(const line of frame.split(/\r?\n/)){
  if(line.startsWith('event:'))event=line.slice(6).replace(/^ /,'')
  else if(line.startsWith('data:'))data.push(line.slice(5).replace(/^ /,''))
 }
 return data.length?{event,data:data.join('\n')}:null
}
export async function readSSE(response:Response,onEvent:(event:SSEEvent)=>void){
 if(!response.body)throw new Error('浏览器不支持流式响应')
 const reader=response.body.getReader();const decoder=new TextDecoder();let buffer=''
 try{
  while(true){
   const {value,done}=await reader.read()
   buffer+=done?decoder.decode():decoder.decode(value,{stream:true})
   let boundary:RegExpExecArray|null
   while((boundary=/\r?\n\r?\n/.exec(buffer))!==null){
    const frame=buffer.slice(0,boundary.index);buffer=buffer.slice(boundary.index+boundary[0].length)
    const parsed=parseSSEFrame(frame);if(parsed)onEvent(parsed)
   }
   if(buffer.length>1_000_000)throw new Error('流式响应过大')
   if(done){if(buffer.trim()){const parsed=parseSSEFrame(buffer);if(parsed)onEvent(parsed)}return}
  }
 }finally{try{await reader.cancel()}finally{reader.releaseLock()}}
}
