"""Regression tests do not need Docker, a model, .env or network."""
import unittest
from unittest.mock import patch
import subprocess
import sys
import runpy
from pathlib import Path
from support import Context

class SQLResultTests(unittest.TestCase):
    def query(self,text):
        c=Context.__new__(Context)
        with patch('support.subprocess.run',return_value=subprocess.CompletedProcess([],0,stdout=text,stderr='')):
            return c.sql('SELECT fixture')
    def test_empty_value_is_one_row(self):
        self.assertEqual(self.query('\n'),[['']])
        self.assertEqual(self.query(''),[])
    def test_tabs_and_spaces_are_preserved(self):
        self.assertEqual(self.query('\t\n'),[['','']])
        self.assertEqual(self.query('  value  \n'),[['  value  ']])
    def test_unicode_and_multiple_rows(self):
        self.assertEqual(self.query('中文\n第二条\n'),[['中文'],['第二条']])
    def test_explicit_utf8_capture(self):
        c=Context.__new__(Context)
        actual_run=subprocess.run
        def child(*args,**kwargs):
            self.assertEqual(kwargs.get('encoding'),'utf-8')
            self.assertEqual(kwargs.get('errors'),'strict')
            return actual_run([sys.executable,'-c',"import sys;sys.stdout.buffer.write('你是耐心的数学老师\\n'.encode('utf-8'))"],**kwargs)
        with patch('support.subprocess.run',side_effect=child):
            self.assertEqual(c.scalar('SELECT chinese'),'你是耐心的数学老师')
    def test_missing_capture_has_readable_error(self):
        c=Context.__new__(Context)
        with patch('support.subprocess.run',return_value=subprocess.CompletedProcess([],0,stdout=None,stderr='')):
            with self.assertRaisesRegex(RuntimeError,'数据库输出读取失败'):c.sql('SELECT fixture')
    def test_scalar_empty(self):
        c=Context.__new__(Context)
        with patch('support.subprocess.run',return_value=subprocess.CompletedProcess([],0,stdout='\n',stderr='')):
            self.assertEqual(c.scalar('SELECT empty'),'')
class FixtureRestoreTests(unittest.TestCase):
    def test_agent_restored_after_assertion_failure(self):
        original={'id':7,'prompt':'原角色','enabled':True,'skill_ids':[],'material_ids':[]}
        class Fake:
            def __init__(self):self.calls=[]
            def agent(self):return original
            def request(self,method,path,body=None,**kwargs):
                self.calls.append((method,path,body));return 200,body
            def scalar(self,query):raise AssertionError('simulated SQL check failure')
        c=Fake()
        case=Path(__file__).parent/'cases/api/agents/id/PUT.py'
        with self.assertRaises(AssertionError):runpy.run_path(str(case))['run'](c)
        self.assertEqual(c.calls[-1][2],original)
        self.assertTrue(c.calls[-1][2]['enabled'])
if __name__=='__main__':unittest.main()
