"""Phone-local adapter. One active task, cancellable waits, no credential files."""
import json
import os
import re
import threading
import time
from pathlib import Path


class Stopped(BaseException):
    pass


def run(raw, directory, callback):
    config = json.loads(raw)
    task, account, model = config['task'], config['account'], config['model']
    os.chdir(directory)
    import requests
    import superstar
    import api.base as base
    import api.cookies as cookies
    import api.cxsecret_font as fonts
    import model_client
    import mobile_course
    from api.answer import AI
    from api.logger import logger
    from tqdm import tqdm

    secrets = [str(account.get('username', '')), str(account.get('password', '')), str(model.get('api_key', ''))]
    state = {'course_name': '', 'chapter_name': '', 'position_seconds': 0, 'duration_seconds': 0}
    lock = threading.RLock()
    last_progress = [0.0]
    sleep, request = time.sleep, requests.sessions.Session.request

    def check():
        if callback.isStopped():
            raise Stopped()

    def safe(text):
        text = str(text)
        for secret in sorted(filter(None, secrets), key=len, reverse=True):
            text = text.replace(secret, '[已隐藏]')
        text = re.sub(r'https?://\S+', '[接口地址]', text)
        text = re.sub(r'(?i)(bearer\s+|(?:password|token|api[_-]?key|authorization)["\x27]?\s*[:=]\s*["\x27]?)[^\s"\x27,}]+', r'\1[已隐藏]', text)
        return text.replace('\x00', '')[-1500:]

    def report(status='running', message='', **values):
        with lock:
            state.update(values)
            event = dict(state, state=status, message=safe(message))
            callback.report(json.dumps(event, ensure_ascii=False))

    def cancellable_sleep(seconds):
        until = time.monotonic() + max(0, seconds)
        while time.monotonic() < until:
            check()
            sleep(min(0.25, until - time.monotonic()))
        check()

    def cancellable_request(session, method, url, **kwargs):
        check()
        kwargs['timeout'] = min(float(kwargs.get('timeout') or 30), 40)
        result = request(session, method, url, **kwargs)
        check()
        if result.status_code in (401, 403) and 'chaoxing.com' in url:
            raise RuntimeError('学习平台需要重新登录或安全验证，请在学习通完成验证后重试')
        return result

    def log(message):
        record = message.record
        if record['level'].no >= 20:
            report(message=record['message'])

    class ProgressBar(tqdm):
        def __init__(self, *args, **kwargs):
            self.desc = kwargs.get('desc', '')
            kwargs['disable'] = True
            super().__init__(*args, **kwargs)

        def refresh(self, *args, **kwargs):
            check()
            now = time.monotonic()
            if now - last_progress[0] >= 2:
                last_progress[0] = now
                report(message=self.desc or '正在学习', position_seconds=int(self.n or 0), duration_seconds=int(self.total or 0))

    original_chapter = superstar.process_chapter
    original_bar = base.tqdm

    def chapter(client, course, point, speed):
        check()
        report(message='正在读取章节任务', course_name=course['title'][:200], chapter_name=point['title'][:200], position_seconds=0, duration_seconds=0)
        jobs, info = client.get_job_list(course, point, mark_empty=False)
        if info.get('notOpen') and task['notopen_action'] == 'stop':
            raise mobile_course.ClosedChapter('章节尚未开放，已按设置停止任务')
        if info.get('notOpen') and task['notopen_action'] == 'skip':
            report(message='章节尚未开放，已按设置跳过')
            return superstar.ChapterResult.PENDING
        return original_chapter(client, course, point, speed)

    try:
        # All upstream requests/waits, including chapter worker threads, honor stop.
        time.sleep = cancellable_sleep
        requests.sessions.Session.request = cancellable_request
        base.tqdm = ProgressBar
        superstar.process_chapter = chapter
        fonts.resource_path = lambda relative: str(Path(__file__).parent / relative)
        model_client.configuration = model
        logger.remove()
        logger.add(log, level='INFO', catch=False, enqueue=False, format='{message}')
        retries = int(task['retries'])
        for attempt in range(retries + 1):
            check()
            try:
                base.SessionManager._instance = None
                cookies._cookies = {}
                tiku = AI()
                tiku.config_set({
                    'endpoint': model['endpoint'], 'key': model['api_key'], 'model': model['primary'],
                    'fallback_model': model.get('fallback', ''), 'http_proxy': '',
                    'min_interval_seconds': '3', 'ai_retry_times': str(task['ai_retries']),
                    'submit': str(task['auto_submit']).lower(), 'auto_submit': str(task['auto_submit']).lower(),
                    'cover_rate': '1', 'delay': '1', 'true_list': '正确,对,√,是', 'false_list': '错误,错,×,否,不对,不正确',
                })
                tiku.init_tiku()
                client = base.Chaoxing(account=base.Account(account['username'], account['password']), tiku=tiku, query_delay=1)
                report(message='正在登录所选学习账号', chapter_name='', position_seconds=0, duration_seconds=0)
                login = client.login()
                if not login.get('status'):
                    raise RuntimeError('学习平台登录失败，请检查账号密码或在学习通完成安全验证')
                all_courses = client.get_course_list()
                selected = {str(item['id']) for item in task['courses']}
                courses = [item for item in all_courses if str(item['courseId']) in selected]
                if selected - {str(item['courseId']) for item in courses}:
                    raise RuntimeError('未找到所选课程，请重新获取该账号课程列表')
                skipped = 0
                for course in courses:
                    check()
                    report(message='读取课程章节', course_name=course['title'][:200], chapter_name='', position_seconds=0, duration_seconds=0)
                    skipped += mobile_course.run_course(client, course, task, report, check) or 0
                report('paused' if skipped else 'completed', f'已执行可用章节，跳过 {skipped} 个未开放章节，等待后续继续' if skipped else '平台完成状态核验通过', position_seconds=0, duration_seconds=0)
                break
            except Exception as error:
                reason = safe(str(error)) or type(error).__name__
                permanent = re.search(r'登录失败|验证码|安全验证|密码|未找到|尚未开放，已按设置停止', reason)
                if attempt >= retries or permanent:
                    report('failed', reason, position_seconds=0, duration_seconds=0)
                    break
                wait = int(task['retry_delay']) * (1 if attempt == 0 else 3)
                report('retrying', f'{reason}；{wait} 秒后重试（{attempt + 1}/{retries}）', position_seconds=0, duration_seconds=0)
                cancellable_sleep(wait)
    except Stopped:
        report('cancelled', '已停止任务，进度以平台记录为准', position_seconds=0, duration_seconds=0)
    except model_client.ModelAccessError as error:
        report('failed', str(error), position_seconds=0, duration_seconds=0)
    except Exception:
        report('failed', '本机执行异常，请检查运行配置后重试', position_seconds=0, duration_seconds=0)
    finally:
        time.sleep = sleep
        requests.sessions.Session.request = request
        base.tqdm = original_bar
        superstar.process_chapter = original_chapter
        logger.remove()
        cookies._cookies = {}
        base.SessionManager._instance = None
        model_client.configuration = {}
        secrets.clear()
