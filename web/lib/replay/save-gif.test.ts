import test from 'node:test';
import assert from 'node:assert/strict';
import { pickGifTarget, pickVideoTarget, writeGifTarget, writeVideoTarget } from './save-gif.ts';

function bridge(pickAnswer: unknown, writeAnswer: unknown, calls: string[]): object {
  return {
    cliphubReplay: {
      pickGif: async (fileName: string) => {
        calls.push(`pick ${fileName}`);
        return pickAnswer;
      },
      writeGif: async (token: string, bytes: Uint8Array) => {
        calls.push(`write ${token} ${bytes.length}`);
        return writeAnswer;
      },
    },
  };
}

test('the desktop picks the file first, then gets the bytes under its token', async () => {
  const calls: string[] = [];
  const scope = bridge({ ok: true, token: 't1' }, { ok: true, name: 'a.gif' }, calls);
  assert.deepEqual(await pickGifTarget('a.gif', scope), { kind: 'file', token: 't1' });
  assert.deepEqual(await writeGifTarget('t1', new Uint8Array(6), scope), { kind: 'saved', name: 'a.gif' });
  assert.deepEqual(calls, ['pick a.gif', 'write t1 6']);
});

test('a closed dialog, a refusal and a bad answer are told apart', async () => {
  assert.deepEqual(await pickGifTarget('a.gif', bridge({ ok: false, canceled: true }, null, [])), { kind: 'canceled' });
  assert.deepEqual(await pickGifTarget('a.gif', bridge({ ok: false, error: 'El GIF se guarda desde un clic.' }, null, [])), { kind: 'failed', message: 'El GIF se guarda desde un clic.' });
  assert.deepEqual(await pickGifTarget('a.gif', bridge({ ok: true }, null, [])), { kind: 'failed', message: 'No se pudo guardar el GIF.' });
  assert.deepEqual(await writeGifTarget('t', new Uint8Array(1), bridge(null, { ok: false, error: 'Disco lleno.' }, [])), { kind: 'failed', message: 'Disco lleno.' });
});

test('without the bridge the GIF downloads', async () => {
  assert.deepEqual(await pickGifTarget('a.gif', {}), { kind: 'download' });
  assert.deepEqual(await pickGifTarget('a.gif', { cliphubReplay: { pickGif: 'no' } }), { kind: 'download' });
});

test('the MP4 goes through its own bridge calls, and a desktop without them downloads it', async () => {
  const calls: string[] = [];
  const scope = {
    cliphubReplay: {
      pickVideo: async (fileName: string) => {
        calls.push(`pick ${fileName}`);
        return { ok: true, token: 'v1' };
      },
      writeVideo: async (token: string, bytes: Uint8Array) => {
        calls.push(`write ${token} ${bytes.length}`);
        return { ok: true, name: 'a.mp4' };
      },
    },
  };
  assert.deepEqual(await pickVideoTarget('a.mp4', scope), { kind: 'file', token: 'v1' });
  assert.deepEqual(await writeVideoTarget({ token: 'v1', bytes: new Uint8Array(8), scope }), { kind: 'saved', name: 'a.mp4' });
  assert.deepEqual(calls, ['pick a.mp4', 'write v1 8']);
  assert.deepEqual(await pickVideoTarget('a.mp4', bridge({ ok: true, token: 't' }, null, [])), { kind: 'download' });
});
