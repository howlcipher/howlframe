"""Independent black-box contract. Server lifecycle is external, no app imports.
EVIDENCEDESK_URL must point to a fresh server. PROJECTOR_ARGV is JSON argv prefix.
Run: python3 -m unittest discover -s tests -p acceptance.py -v
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
import urllib.error
import urllib.request

BASE = os.environ.get('EVIDENCEDESK_URL', 'http://127.0.0.1:8093')
FIXTURES = Path(__file__).parent / 'fixtures'


def entry(eid, task='TASK-A', action='task_created', **extra):
    return dict(schema='ai.evidence_entry/v1', entry_id=eid, task_id=task,
                agent_id='independent-test', action=action, **extra)


def ledger(records):
    return '\n'.join(json.dumps(x, ensure_ascii=False) for x in records).encode()


def request(method, path, body=None, media='application/json'):
    if body is not None and not isinstance(body, bytes):
        body = json.dumps(body).encode()
    req = urllib.request.Request(BASE + path, data=body, method=method,
                                 headers={'Content-Type': media})
    try:
        response = urllib.request.urlopen(req, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, json.loads(response.read())


class API(unittest.TestCase):
    def upload(self, data):
        return request('POST', '/api/import', data, 'application/x-ndjson')

    def setUp(self):
        self.records = [entry('e1'), entry('e2', action='verification_completed',
                        result='claimed', verification_summary='agent says so'),
                        entry('e3', 'TASK-B', result='<img src=x onerror="window.pwned=1">')]
        self.assertEqual(self.upload(ledger(self.records))[0], 200)
        request('PATCH', '/api/tasks/TASK-A/note', {'note': ''})

    def test_import_detail_and_order(self):
        status, result = self.upload(ledger(self.records))
        self.assertEqual((status, result['entries'], result['tasks']), (200, 3, 2))
        status, result = request('GET', '/api/tasks')
        self.assertEqual([x['id'] for x in result['items']], ['TASK-A', 'TASK-B'])
        self.assertEqual([x['events'] for x in result['items']], [2, 1])
        status, detail = request('GET', '/api/tasks/TASK-A')
        self.assertEqual(detail['events'], self.records[:2])
        self.assertEqual(detail['events'][1]['result'], 'claimed')

    def test_invalid_import_preserves_state(self):
        request('PATCH', '/api/tasks/TASK-A/note', {'note': 'retain me'})
        bad = [b'{broken', b'[]', ledger([entry('x'), entry('x')]),
               ledger([dict(entry('x'), schema='unknown')]),
               ledger([dict(entry('x'), task_id=4)]), b'\xff', b' \n']
        for payload in bad:
            with self.subTest(payload=payload[:80]):
                status, result = self.upload(payload)
                self.assertEqual(status, 400)
                self.assertEqual(result['error']['code'], 'invalid_ledger')
                self.assertTrue(result['error']['message'])
                self.assertEqual(request('GET', '/api/tasks/TASK-A')[1]['note'], 'retain me')
        status, result = self.upload(ledger([entry('good')]) + b'\n{broken')
        self.assertEqual((status, result['error']['line']), (400, 2))

    def test_limits_media_and_routing(self):
        for payload in [b'x' * 262145, ledger([entry(str(i)) for i in range(1001)])]:
            status, result = self.upload(payload)
            self.assertEqual((status, result['error']['code']), (413, 'payload_too_large'))
        self.assertEqual(request('POST', '/api/import', b'{}')[0], 415)
        self.assertEqual(request('DELETE', '/api/tasks')[0], 405)
        self.assertEqual(request('GET', '/api/not-a-route')[0], 404)
        self.assertEqual(request('GET', '/api/health'), (200, {'status': 'ok'}))

    def test_notes_and_reimport(self):
        note = 'reviewer annotation café 😀'
        status, detail = request('PATCH', '/api/tasks/TASK-A/note', {'note': note})
        self.assertEqual((status, detail['note']), (200, note))
        for body in [{'note': 1}, {'note': 'x', 'extra': 1}, {}, {'note': 'x' * 2001}]:
            self.assertEqual(request('PATCH', '/api/tasks/TASK-A/note', body)[0], 400)
        self.assertEqual(self.upload(ledger([entry('new')]))[0], 200)
        self.assertEqual(request('GET', '/api/tasks/TASK-A')[1]['note'], note)
        self.assertEqual(request('GET', '/api/tasks/TASK-B')[0], 404)


class Projector(unittest.TestCase):
    def run_projector(self, raw, maximum='1000'):
        prefix = json.loads(os.environ['PROJECTOR_ARGV'])
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / 'input.jsonl'
            source.write_bytes(raw)
            output = root / 'out'
            output.mkdir()
            result = subprocess.run(prefix + [str(source), str(output), maximum],
                                    capture_output=True, timeout=30)
            files = {p.name: p.read_bytes() for p in output.iterdir()}
            return result, files

    def test_exact_limit_and_validate_before_write(self):
        valid = ledger([entry('1'), entry('2')])
        result, files = self.run_projector(valid, '1')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(files, {})
        for raw in [valid + b'\n{bad', ledger([entry('1'), entry('1')])]:
            result, files = self.run_projector(raw)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(files, {})
            self.assertTrue(result.stderr)
        for maximum in ['0', '-1', '1.5', '1001', 'no']:
            self.assertEqual(self.run_projector(valid, maximum)[0].returncode, 2)

    def test_native_outputs_deterministic(self):
        raw = (FIXTURES / 'review.jsonl').read_bytes()
        first, outputs = self.run_projector(raw)
        self.assertEqual(first.returncode, 0, first.stderr)
        self.assertEqual(set(outputs), {'howlboard_missions.json', 'howlboard_timeline.json'})
        second, again = self.run_projector(raw)
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual({k: json.loads(v) for k, v in outputs.items()},
                         {k: json.loads(v) for k, v in again.items()})


if __name__ == '__main__':
    unittest.main()
