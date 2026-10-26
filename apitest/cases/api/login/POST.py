import sys, re
from pathlib import Path
for parent in Path(__file__).resolve().parents:
    if (parent/"support.py").exists():sys.path.insert(0,str(parent));break
from support import literal

def run(c):
    c.ensure_login()
    r=c.login('student_a');assert r['token_type']=='Bearer' and r['expires_in']==43200 and len(r['access_token'])==64
    assert int(c.scalar("SELECT COUNT(*) FROM sessions s JOIN users u ON u.id=s.user_id WHERE u.username='student_a' AND s.expires_at>NOW()"))>0
    assert c.request('GET','/api/me',user=None)[0]==401

if __name__=="__main__":
    from run_all import run_single
    sys.exit(run_single(__file__))
