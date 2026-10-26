"""python apitest/run_all.py; each URL case can also run independently."""
import argparse, html, json, runpy, sys, time
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from support import Context

def run_single(path):return run([Path(path)])
def run(paths):
    paths=[p.resolve() for p in paths]
    c=Context();results=[]
    for path in paths:
        start=time.monotonic()
        try:runpy.run_path(str(path))['run'](c);results.append((str(path.relative_to(Path(__file__).resolve().parent)),True,'',time.monotonic()-start))
        except Exception as e:results.append((str(path.relative_to(Path(__file__).resolve().parent)),False,str(e),time.monotonic()-start))
    try:c.cleanup()
    except Exception as e:results.append(('fixture cleanup',False,str(e),0))
    out=Path(__file__).parent/'reports';out.mkdir(exist_ok=True)
    rows=''.join(f'<tr><td>{html.escape(p)}</td><td>{"PASS" if ok else "FAIL"}</td><td>{secs:.2f}s</td><td>{html.escape(msg)}</td></tr>' for p,ok,msg,secs in results)
    report='<!doctype html><meta charset="utf-8"><title>CampusClaw API测试</title><style>body{font:16px sans-serif;margin:30px}table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:10px}pre{white-space:pre-wrap}</style><h1>CampusClaw 第3–6课接口测试</h1><p>同时核对HTTP响应和Compose MySQL；Token与口令已脱敏。</p><table><tr><th>接口脚本</th><th>结果</th><th>耗时</th><th>详情</th></tr>'+rows+'</table><h2>请求响应记录</h2><pre>'+html.escape(json.dumps(c.log,ensure_ascii=False,indent=2))+'</pre>'
    (out/'report.html').write_text(report,encoding='utf-8')
    for p,ok,msg,_ in results:print(('PASS ' if ok else 'FAIL ')+p+(' '+msg if msg else ''))
    print('报告：',out/'report.html');return 0 if all(ok for _,ok,_,_ in results) else 1
if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--case');args=parser.parse_args()
    paths=[Path(__file__).parent/'cases'/args.case] if args.case else sorted((Path(__file__).parent/'cases').rglob('*.py'))
    sys.exit(run(paths))
