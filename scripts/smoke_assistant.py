"""第五课详细讲义版本的集成验收已迁移至 apitest；保留此入口。"""
import runpy
from pathlib import Path
if __name__ == '__main__':
    runpy.run_path(str(Path(__file__).resolve().parents[1]/'apitest'/'run_all.py'),run_name='__main__')
