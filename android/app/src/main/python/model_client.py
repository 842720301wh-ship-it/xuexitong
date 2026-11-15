"""Small compatibility client for the upstream AI prompts; no SDK dependency."""
from types import SimpleNamespace
import requests

configuration = {}

class ModelAccessError(BaseException):
    """Quota/auth failures stop the task instead of falling back to random answers."""
    pass


class OpenAI:
    def __init__(self, *, base_url, api_key, **kwargs):
        self.endpoint, self.key = base_url.rstrip('/'), api_key
        self.chat = SimpleNamespace(completions=SimpleNamespace(create=self.create))

    def create(self, *, model, messages, **kwargs):
        protocol = configuration.get('protocol') or 'chat'
        headers = {'Content-Type': 'application/json'}
        body = {'model': model, 'stream': False}
        if protocol == 'responses':
            path = '/responses'
            body['input'] = messages
            headers['Authorization'] = 'Bearer ' + self.key
        elif protocol == 'anthropic':
            path = '/messages'
            body['system'] = '\n'.join(m['content'] for m in messages if m['role'] == 'system')
            body['messages'] = [m for m in messages if m['role'] != 'system']
            body['max_tokens'] = 4096
            headers.update({'x-api-key': self.key, 'anthropic-version': '2023-06-01'})
        else:
            path = '/chat/completions'
            body['messages'] = messages
            # Provider-specific flags are only supplied by upstream for that provider.
            body.update(kwargs.get('extra_body') or {})
            headers['Authorization'] = 'Bearer ' + self.key
        response = requests.post(self.endpoint + path, json=body, headers=headers, timeout=40, allow_redirects=False)
        if response.status_code == 402:
            raise ModelAccessError('AI 调用额度已达上限，请检查服务商用量或余额后重试')
        if response.status_code in (401, 403):
            raise ModelAccessError('AI 认证或访问权限不足，请检查 API Key 和模型权限')
        if response.status_code != 200:
            raise RuntimeError(f'{protocol} HTTP {response.status_code}')
        if len(response.content) > 4 * 1024 * 1024:
            raise RuntimeError('模型响应过大')
        data = response.json()
        if data.get('error'):
            raise RuntimeError('模型接口返回错误')
        if protocol == 'responses':
            if data.get('status') not in (None, 'completed'):
                raise RuntimeError('模型响应未完成')
            text = '\n'.join(p.get('text', '') for item in data.get('output', []) if item.get('type') == 'message' for p in item.get('content', []) if p.get('type') == 'output_text')
        elif protocol == 'anthropic':
            text = '\n'.join(p.get('text', '') for p in data.get('content', []) if p.get('type') == 'text')
        else:
            text = data.get('choices', [{}])[0].get('message', {}).get('content') or ''
            if isinstance(text, list):
                text = '\n'.join(p.get('text', '') for p in text if p.get('type') == 'text')
        if not text.strip():
            raise RuntimeError('模型没有返回有效答案')
        return SimpleNamespace(choices=[SimpleNamespace(message=SimpleNamespace(content=text))])
