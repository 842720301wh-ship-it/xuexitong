"""Bounded chapter scheduling which can always unwind when the user stops."""
from concurrent.futures import ThreadPoolExecutor
import time
import superstar


class ClosedChapter(Exception):
    pass


def run_course(client, course, config, report, check):
    check()
    overview = client.get_course_point(course['courseId'], course['clazzId'], course['cpi'])
    points = overview['points']
    if not points:
        raise RuntimeError('课程未读取到章节，无法确认完成')
    skipped = set()

    def process(point):
        for attempt in range(5):
            check()
            current = next((p for p in client.get_course_point(course['courseId'], course['clazzId'], course['cpi'])['points'] if p['id'] == point['id']), None)
            if current is None:
                raise RuntimeError('平台章节列表缺少当前章节')
            point.update(current)
            point['verify_finished'] = True
            report(message='正在处理章节', course_name=course['title'][:200], chapter_name=point['title'][:200], position_seconds=0, duration_seconds=0)
            result = superstar.process_chapter(client, course, point, config['speed'])
            if result == superstar.ChapterResult.PENDING:
                skipped.add(point['id'])
                return
            if result == superstar.ChapterResult.SUCCESS and superstar.verify_chapter_finished(client, course, point):
                return
            if result == superstar.ChapterResult.INCOMPLETE:
                raise RuntimeError('章节任务尚未完成；请检查答题是否仅保存、缺少答案或存在未处理任务')
            if attempt < 4:
                time.sleep(2)
        raise RuntimeError('章节处理或平台核验未通过：' + point['title'])

    if config['sequential'] or overview.get('hasLocked'):
        for point in points:
            process(point)
    else:
        # No Queue.join(): cancelled worker futures always release the scheduler.
        with ThreadPoolExecutor(max_workers=config['jobs']) as pool:
            futures = [pool.submit(process, point) for point in points]
            for future in futures:
                future.result()
    check()
    latest = client.get_course_point(course['courseId'], course['clazzId'], course['cpi'])['points']
    completed = {p['id'] for p in latest if p.get('has_finished')}
    missing = [p for p in points if p['id'] not in completed and p['id'] not in skipped]
    if missing:
        raise RuntimeError('平台仍有未完成章节：' + '、'.join(p['title'] for p in missing))
    return len(skipped)
