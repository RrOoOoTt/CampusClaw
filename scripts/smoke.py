"""Run after docker compose up; stdlib only."""
import http.cookiejar
import json
import os
import urllib.error
import urllib.request

BASE = os.getenv('BASE_URL', 'http://localhost:8080')

def client():
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

def request(opener, path, method='GET', body=None, headers=None):
    request_headers = dict(headers or {})
    token = getattr(opener, 'access_token', None)
    if token and path != '/api/login':
        request_headers.setdefault('Authorization', 'Bearer ' + token)
    req = urllib.request.Request(BASE + path, data=body, method=method, headers=request_headers)
    try:
        with opener.open(req) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as err:
        return err.code, err.read()

def login(opener, name, password):
    status, data = request(opener, '/api/login', 'POST', json.dumps({'username': name, 'password': password}).encode(), {'Content-Type': 'application/json'})
    assert status == 200, (name, status)
    session = json.loads(data)
    assert session['token_type'] == 'Bearer' and session['expires_in'] == 43200
    opener.access_token = session['access_token']

def multipart(filename, content):
    boundary = 'CampusClawBoundary'
    head = f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\nContent-Type: application/octet-stream\r\n\r\n'.encode()
    return head + content + f'\r\n--{boundary}--\r\n'.encode(), {'Content-Type': f'multipart/form-data; boundary={boundary}'}

if __name__ == '__main__':
    anonymous = client()
    assert request(anonymous, '/health')[0] == 200
    for path in ('/api/me', '/api/materials'):
        assert request(anonymous, path)[0] == 401
    users = {}
    for name in ('teacher_a', 'student_a', 'teacher_b', 'student_b'):
        env = 'SEED_' + name.upper() + '_PASSWORD'
        password = os.environ[env]
        op = client(); login(op, name, password)
        assert request(op, '/api/me')[0] == 200
        users[name] = op
    # A legacy cookie alone must not authenticate; malformed and unknown Bearer tokens fail.
    assert request(client(), '/api/me', headers={'Cookie': 'cc_session=' + users['teacher_a'].access_token})[0] == 401
    for value in ('Basic abc', 'Bearer invalid', 'Bearer ' + '0' * 64):
        assert request(client(), '/api/me', headers={'Authorization': value})[0] == 401
    body, headers = multipart('lesson.md', b'# CampusClaw smoke test\nOnly class A.')
    status, data = request(users['teacher_a'], '/api/materials', 'POST', body, headers)
    assert status == 201, (status, data)
    mid = json.loads(data)['id']
    for name in ('teacher_a', 'student_a'):
        status, data = request(users[name], '/api/materials/' + str(mid))
        assert status == 200 and b'Only class A.' in data
        assert request(users[name], '/api/materials/' + str(mid) + '/file')[0] == 200
        assert any(x['id'] == mid for x in json.loads(request(users[name], '/api/materials')[1]))
    for name in ('teacher_b', 'student_b'):
        assert request(users[name], '/api/materials/' + str(mid))[0] == 404
        assert request(users[name], '/api/materials/' + str(mid) + '/file')[0] == 404
        assert all(x['id'] != mid for x in json.loads(request(users[name], '/api/materials')[1]))
    assert request(users['student_a'], '/api/materials', 'POST', body, headers)[0] == 403
    before = [x['id'] for x in json.loads(request(users['teacher_a'], '/api/materials')[1])]
    for filename, contents, expected in [('bad.exe', b'text', 415), ('bad.txt', b'\xff', 422), ('too-big.txt', b'x' * (2 * 1024 * 1024 + 1), 413)]:
        b, h = multipart(filename, contents)
        assert request(users['teacher_a'], '/api/materials', 'POST', b, h)[0] == expected
    assert before == [x['id'] for x in json.loads(request(users['teacher_a'], '/api/materials')[1])]
    assert request(users['teacher_a'], '/api/logout', 'POST')[0] == 200
    assert request(users['teacher_a'], '/api/me')[0] == 401
    print('PASS: authentication, RBAC, isolation, ingestion, invalid files, logout, liveness')
