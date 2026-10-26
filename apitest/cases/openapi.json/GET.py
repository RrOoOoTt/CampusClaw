import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    code,spec=c.request('GET','/openapi.json',user=None);assert code==200 and spec['openapi']=='3.0.3'
    assert '/api/agents' in spec['paths'] and spec['components']['securitySchemes']['BearerAuth']['scheme']=='bearer'
    code,ui=c.request('GET','/swagger/',user=None);assert code==200 and 'SwaggerUIBundle' in ui

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
