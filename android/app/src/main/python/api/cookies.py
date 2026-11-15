# Mobile runtime keeps platform cookies in memory only.
_cookies = {}
def save_cookies(session):
    global _cookies
    _cookies = dict(session.cookies.items())
def use_cookies():
    return dict(_cookies)
