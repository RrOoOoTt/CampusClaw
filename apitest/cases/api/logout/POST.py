import sys, hmac, hashlib
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/'support.py').exists():sys.path.insert(0,str(parent));break

def run(c):
    c.ensure_login()
    token=c.tokens['student_a'];digest=hmac.new(c.env['SESSION_SECRET'].encode(),token.encode(),hashlib.sha256).hexdigest()
    assert c.scalar(f"SELECT COUNT(*) FROM sessions WHERE token_hash=UNHEX('{digest}')")=='1'
    assert c.request('POST','/api/logout',user='student_a')[0]==200
    assert c.request('GET','/api/me',user='student_a')[0]==401
    assert c.scalar(f"SELECT COUNT(*) FROM sessions WHERE token_hash=UNHEX('{digest}')")=='0'
    c.login('student_a')
if __name__=='__main__':
    from run_all import run_single
    sys.exit(run_single(__file__))
