import test from 'node:test';
import assert from 'node:assert/strict';
import type { StorageFile } from '../api/storage.ts';
import { renderVideoFile } from './video-file.ts';

const REV = '0d5b8a5e-1c2f-4b7a-9e3d-2a1b3c4d5e6f';

function file(path: string, jobId = 'job'): StorageFile {
  return { path, sizeBytes: 1, modifiedAt: '', kind: 'full_demo', jobId };
}

test('renderVideoFile picks the render revision on disk, not another job or variant', () => {
  const files = [
    file('C:\\data\\jobs\\other\\renders\\gameplay-pov-60\\videos\\full.mp4', 'other'),
    file('C:\\data\\jobs\\job\\renders\\viral-60-clean\\videos\\full.mp4'),
    file('C:\\data\\jobs\\job\\renders\\gameplay-pov-60\\revisions\\old\\videos\\full.mp4'),
    file(`C:\\data\\jobs\\job\\renders\\gameplay-pov-60\\revisions\\${REV}\\videos\\full.mp4`),
  ];
  const ref = { jobId: 'job', variant: 'gameplay-pov-60', artifactName: 'full.mp4', revision: REV };
  assert.equal(renderVideoFile(files, ref)?.path, files[3].path);
  assert.equal(renderVideoFile(files, { ...ref, revision: 'legacy:1' })?.path, files[2].path);
  assert.equal(renderVideoFile(files, { ...ref, artifactName: 'other.mp4' }), undefined);
});
