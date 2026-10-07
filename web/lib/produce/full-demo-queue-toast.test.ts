import assert from 'node:assert/strict';
import test from 'node:test';
import { fullDemoQueueToast } from './full-demo-queue-toast.ts';

const NO_PACKETS = 'Esta demo no tiene datos de voz. El vídeo se hace sin las voces del equipo.';

test('a queued long video shows plan warnings long enough to dismiss', () => {
  const plain = fullDemoQueueToast(false, []);
  assert.equal(plain.description, 'Sigue el progreso en Demos y vídeos.');
  assert.equal(plain.duration, undefined);
  assert.equal(plain.closeButton, undefined);

  const warned = fullDemoQueueToast(false, [{ message: NO_PACKETS }]);
  assert.equal(warned.description, `${NO_PACKETS} Sigue el progreso en Demos y vídeos.`);
  assert.equal(warned.duration, 12_000);
  assert.equal(warned.closeButton, true);

  const busy = fullDemoQueueToast(true, [{ message: NO_PACKETS }, { message: '  ' }]);
  assert.equal(busy.description, `${NO_PACKETS} Empezará cuando quede libre CS2.`);
  assert.equal(busy.closeButton, true);
});
