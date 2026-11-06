import test from 'node:test';
import assert from 'node:assert/strict';
import { formatUploadBytes, uploadForm, uploadProgress, uploadSent, type UploadProgress } from './upload.ts';

const MB = 1024 * 1024;

test('upload progress floors the percent and clamps the bytes', () => {
  assert.deepEqual(uploadProgress(120 * MB, 480 * MB), { loaded: 120 * MB, total: 480 * MB, percent: 25 });
  assert.equal(uploadProgress(479.9 * MB, 480 * MB)?.percent, 99);
  assert.deepEqual(uploadProgress(500, 400), { loaded: 400, total: 400, percent: 100 });
  assert.equal(uploadProgress(-5, 400)?.loaded, 0);
  assert.equal(uploadProgress(10, 0), null);
  assert.equal(uploadProgress(10, Number.NaN), null);
});

test('upload bytes read like Windows Explorer sizes', () => {
  const progress = uploadProgress(120 * MB, 480 * MB);
  assert.ok(progress);
  assert.equal(formatUploadBytes(progress), '120 MB de 480 MB');
  const big = uploadProgress(1.5 * 1024 * MB, 3.2 * 1024 * MB);
  assert.ok(big);
  assert.equal(formatUploadBytes(big), '1,5 GB de 3,2 GB');
});

test('upload is sent only once every byte left', () => {
  assert.equal(uploadSent(null), false);
  assert.equal(uploadSent(uploadProgress(99, 100)), false);
  assert.equal(uploadSent(uploadProgress(100, 100)), true);
});

class FakeXHR {
  static last: FakeXHR | undefined;
  upload: { onprogress: ((event: { lengthComputable: boolean; loaded: number; total: number }) => void) | null } = { onprogress: null };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  status = 0;
  statusText = '';
  responseText = '';
  method = '';
  url = '';
  sent: unknown;
  headers: Record<string, string> = {};
  constructor() {
    FakeXHR.last = this;
  }
  open(method: string, url: string): void {
    this.method = method;
    this.url = url;
  }
  setRequestHeader(name: string, value: string): void {
    this.headers[name] = value;
  }
  getAllResponseHeaders(): string {
    return 'content-type: application/json\r\nx-request-id: abc\r\n';
  }
  send(body: unknown): void {
    this.sent = body;
  }
  abort(): void {
    this.onabort?.();
  }
}

async function withFakeXHR(run: () => Promise<void>): Promise<void> {
  const original = Object.getOwnPropertyDescriptor(globalThis, 'XMLHttpRequest');
  Object.defineProperty(globalThis, 'XMLHttpRequest', { value: FakeXHR, configurable: true, writable: true });
  try {
    await run();
  } finally {
    if (original) Object.defineProperty(globalThis, 'XMLHttpRequest', original);
    else Reflect.deleteProperty(globalThis, 'XMLHttpRequest');
  }
}

test('uploadForm reports progress and hands back the server response', () =>
  withFakeXHR(async () => {
    const seen: UploadProgress[] = [];
    const form = new FormData();
    const pending = uploadForm({ url: '/api/demos/scan', body: form, headers: { 'x-a': '1' }, onProgress: (p) => seen.push(p) });
    const xhr = FakeXHR.last;
    assert.ok(xhr);
    assert.equal(xhr.method, 'POST');
    assert.equal(xhr.url, '/api/demos/scan');
    assert.equal(xhr.sent, form);
    assert.deepEqual(xhr.headers, { 'x-a': '1' });
    xhr.upload.onprogress?.({ lengthComputable: false, loaded: 5, total: 0 });
    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 50, total: 200 });
    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 200, total: 200 });
    xhr.status = 413;
    xhr.responseText = '{"error":"too big"}';
    xhr.onload?.();
    const res = await pending;
    assert.deepEqual(seen.map((p) => p.percent), [25, 100]);
    assert.equal(res.status, 413);
    assert.equal(res.headers.get('x-request-id'), 'abc');
    assert.deepEqual(await res.json(), { error: 'too big' });
  }));

test('uploadForm rejects like fetch on a network error and on abort', () =>
  withFakeXHR(async () => {
    const failed = uploadForm({ url: '/api/streams', body: new FormData() });
    FakeXHR.last?.onerror?.();
    await assert.rejects(failed, TypeError);

    const controller = new AbortController();
    const aborted = uploadForm({ url: '/api/streams', body: new FormData(), signal: controller.signal });
    controller.abort();
    await assert.rejects(aborted, { name: 'AbortError' });
  }));
