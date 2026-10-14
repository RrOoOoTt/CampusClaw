const tokenKey = 'campusclaw_access_token'
export function saveToken(token:string){sessionStorage.setItem(tokenKey,token)}
export function clearToken(){sessionStorage.removeItem(tokenKey)}
export async function apiFetch(path:string,init:RequestInit={}):Promise<Response>{
 const headers=new Headers(init.headers)
 const token=sessionStorage.getItem(tokenKey)
 if(token&&path!=='/api/login')headers.set('Authorization','Bearer '+token)
 const response=await fetch(path,{...init,headers,credentials:'omit'})
 if(response.status===401)clearToken()
 return response
}
