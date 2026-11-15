"""Offline verification of the phone adapter and protocol payloads."""
import json
from pathlib import Path
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

root = Path(__file__).resolve().parents[2]
sys.path[:0] = [str(root / 'android/app/src/main/python'), str(root / '.cache/mobile-python-tests')]
import model_client
import mobile_runner
import superstar
import mobile_course
import api.base as base


class Callback:
    def __init__(self):
        self.events, self.stop = [], False
    def isStopped(self):
        return self.stop
    def report(self, raw):
        self.events.append(json.loads(raw))


def config():
    return {'task': {'name': '任务', 'courses': [{'id': '123'}], 'speed': 1, 'jobs': 1, 'sequential': True, 'notopen_action': 'retry', 'retries': 0, 'retry_delay': 1, 'auto_submit': True, 'ai_retries': 1},
            'account': {'username': '13811112222', 'password': 'private-password'}, 'model': {'endpoint': 'https://example.com/v1', 'api_key': 'private-key', 'primary': 'model', 'protocol': 'responses'}}


class MobileTests(unittest.TestCase):
    def execute(self, callback, process):
        previous = Path.cwd()
        try:
            with tempfile.TemporaryDirectory(dir=root / '.cache') as directory:
                with patch.object(base.Chaoxing, 'login', return_value={'status': True}), patch.object(base.Chaoxing, 'get_course_list', return_value=[{'courseId': '123', 'title': '高等数学'}]), patch.object(mobile_course, 'run_course', side_effect=process):
                    mobile_runner.run(json.dumps(config()), directory, callback)
                assert not (Path(directory) / 'cookies.txt').exists()
                assert not (Path(directory) / 'chaoxing.log').exists()
                import os
                os.chdir(previous)
        finally:
            import os
            os.chdir(previous)

    def test_progress_and_verified_completion(self):
        callback = Callback()
        def process(*args):
            bar = base.tqdm(total=20, initial=0, desc='第一节')
            bar.n = 5
            bar.refresh()
        self.execute(callback, process)
        self.assertEqual(callback.events[-1]['state'], 'completed')
        self.assertTrue(any(e['position_seconds'] == 5 and e['duration_seconds'] == 20 for e in callback.events))

    def test_cancellation_interrupts_waits(self):
        callback = Callback()
        def process(*args):
            callback.stop = True
            import time
            time.sleep(60)
        self.execute(callback, process)
        self.assertEqual(callback.events[-1]['state'], 'cancelled')

    def test_failed_course_is_not_reported_complete_and_logs_redacted(self):
        callback = Callback()
        self.execute(callback, lambda *args: (_ for _ in ()).throw(RuntimeError('private-password https://example.com?token=abc private-key')))
        self.assertEqual(callback.events[-1]['state'], 'failed')
        result = json.dumps(callback.events)
        self.assertNotIn('private-password', result)
        self.assertNotIn('private-key', result)
        self.assertNotIn('token=abc', result)

    def test_protocols(self):
        for protocol in ['chat', 'responses', 'anthropic']:
            model_client.configuration = {'protocol': protocol}
            def respond(endpoint, **kwargs):
                body, headers = kwargs['json'], kwargs['headers']
                self.assertFalse(kwargs['allow_redirects'])
                self.assertEqual(body['model'], 'test-model')
                self.assertTrue(endpoint.endswith({'chat':'/chat/completions', 'responses':'/responses', 'anthropic':'/messages'}[protocol]))
                if protocol == 'responses':
                    self.assertIn('input', body); self.assertNotIn('messages', body)
                    data = {'status': 'completed', 'output':[{'type':'message', 'content':[{'type':'output_text', 'text':'OK'}]}]}
                elif protocol == 'anthropic':
                    self.assertEqual(headers['x-api-key'], 'private-key')
                    self.assertEqual(body['system'], 'system')
                    self.assertEqual(len(body['messages']), 1)
                    data = {'content':[{'type':'text', 'text':'OK'}]}
                else:
                    self.assertEqual(headers['Authorization'], 'Bearer private-key')
                    data = {'choices':[{'message':{'content':'OK'}}]}
                return type('Response', (), {'status_code':200, 'content':b'{}', 'json':lambda self: data})()
            with patch.object(model_client.requests, 'post', side_effect=respond):
                result = model_client.OpenAI(base_url='https://example.com/v1', api_key='private-key').chat.completions.create(model='test-model', messages=[{'role':'system', 'content':'system'}, {'role':'user', 'content':'user'}])
                self.assertEqual(result.choices[0].message.content, 'OK')


    def test_quota_failure_stops_without_course_retry(self):
        callback = Callback()
        calls = []
        def process(*args):
            calls.append(True)
            raise model_client.ModelAccessError('AI 调用额度已达上限')
        self.execute(callback, process)
        self.assertEqual(len(calls), 1)
        self.assertEqual(callback.events[-1]['state'], 'failed')
        self.assertIn('额度', callback.events[-1]['message'])

    def test_parallel_scheduler_unwinds_on_stop(self):
        callback = Callback()
        client = type('Client', (), {'get_course_point':lambda *args: {'points':[{'id':'1','title':'章节一','has_finished':False},{'id':'2','title':'章节二','has_finished':False}]}})()
        def check():
            if callback.stop:
                raise mobile_runner.Stopped()
        def chapter(*args):
            callback.stop = True
            check()
        with patch.object(superstar, 'process_chapter', side_effect=chapter):
            with self.assertRaises(mobile_runner.Stopped):
                mobile_course.run_course(client, {'courseId':'123','clazzId':'456','cpi':'789','title':'课程'}, {'speed':1,'jobs':2,'sequential':False}, lambda **kwargs: None, check)


if __name__ == '__main__':
    unittest.main(verbosity=2)
