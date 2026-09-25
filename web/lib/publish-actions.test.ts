import assert from 'node:assert/strict';
import test from 'node:test';
import { YOUTUBE_STUDIO_URL, parsePublishAssistant } from './api/publish-assistant.ts';
import {
  PUBLISH_ASSISTANT_FAILED_COPY,
  PUBLISH_ASSISTANT_WAITING_COPY,
  downloadPublishMP4,
  initialPublishDraft,
  openYouTubeStudio,
  publishAssistantAvailability,
  publishTagsText,
  recommendedPublishDraft,
} from './publish-actions.ts';

function assistant() {
  const recommendation = {
    title: '4K de Alex en Mirage',
    description: 'Cuatro bajas de Alex en Mirage.',
    keywords: ['Alex', 'Mirage', '4K'],
    tags: ['CS2', 'Mirage', '4K'],
    score: 92,
    rationale: 'Solo usa hechos del render.',
  };
  return parsePublishAssistant({
    schema_version: '1.0',
    metadata: { title: 'Alex en Mirage', description: 'POV completo.', tags: ['CS2', 'Mirage'] },
    recommendations: [
      recommendation,
      { ...recommendation, title: 'Mirage 4K de Alex' },
      { ...recommendation, title: 'POV: Alex consigue 4 bajas' },
    ],
    keywords: ['Alex', 'Mirage', '4K'],
    tags: ['CS2', 'Mirage', '4K'],
    schedule: {
      time_zone: 'Europe/Madrid',
      generated_at: '2026-07-12T10:00:00Z',
      days: [{
        date: '2026-07-13',
        weekday: 'lunes',
        slots: [{
          publish_at: '2026-07-13T18:00:00Z',
          local_time: '20:00',
          source: 'baseline',
          confidence: 0.62,
          score: 0.7,
          rationale: 'Referencia general.',
        }],
      }],
      sources: [],
      caveat: 'Referencia orientativa.',
    },
    trends: { available: false, terms: [], sources: [], reason: 'Sin Firecrawl.' },
    studio_url: YOUTUBE_STUDIO_URL,
  });
}

test('failed videos are not waiting for a YouTube draft', () => {
  assert.equal(publishAssistantAvailability('ready'), 'ready');
  assert.equal(publishAssistantAvailability('failed'), 'failed');
  for (const status of ['queued', 'recording', 'composing', 'review_required'] as const) {
    assert.equal(publishAssistantAvailability(status), 'waiting', status);
  }
  assert.match(PUBLISH_ASSISTANT_WAITING_COPY, /esté listo/);
  assert.match(PUBLISH_ASSISTANT_FAILED_COPY, /falló/);
  assert.notEqual(PUBLISH_ASSISTANT_FAILED_COPY, PUBLISH_ASSISTANT_WAITING_COPY);
});

test('selecting a recommendation replaces editable title, description, and tags', () => {
  const value = assistant();
  assert.deepEqual(initialPublishDraft(value), {
    title: 'Alex en Mirage',
    description: 'POV completo.',
    tags: ['CS2', 'Mirage'],
  });
  assert.deepEqual(recommendedPublishDraft(value.recommendations[0]), {
    title: '4K de Alex en Mirage',
    description: 'Cuatro bajas de Alex en Mirage.',
    tags: ['CS2', 'Mirage', '4K'],
  });
});

test('formats tags as a comma-separated list for the clipboard', () => {
  assert.equal(publishTagsText(['CS2', 'Mirage', '4K']), 'CS2, Mirage, 4K');
});

test('downloads the MP4 with a safe filename and clicks the temporary anchor', () => {
  const events: string[] = [];
  const anchor = {
    href: '',
    download: '',
    rel: '',
    click: () => events.push('click'),
    remove: () => events.push('remove'),
  };
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'document');
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: {
      createElement: (tag: string) => {
        assert.equal(tag, 'a');
        return anchor;
      },
      body: { appendChild: (value: unknown) => { assert.equal(value, anchor); events.push('append'); } },
    },
  });
  try {
    downloadPublishMP4('/api/reel.mp4', 'Mirage: 4K');
  } finally {
    if (previous) Object.defineProperty(globalThis, 'document', previous);
    else Reflect.deleteProperty(globalThis, 'document');
  }
  assert.equal(anchor.href, '/api/reel.mp4');
  assert.equal(anchor.download, 'Mirage- 4K.mp4');
  assert.equal(anchor.rel, 'noopener');
  assert.deepEqual(events, ['append', 'click', 'remove']);
});

test('opens only the stable YouTube Studio URL', () => {
  const calls: unknown[][] = [];
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { open: (...args: unknown[]) => { calls.push(args); } },
  });
  try {
    openYouTubeStudio();
  } finally {
    if (previous) Object.defineProperty(globalThis, 'window', previous);
    else Reflect.deleteProperty(globalThis, 'window');
  }
  assert.deepEqual(calls, [[YOUTUBE_STUDIO_URL, '_blank', 'noopener,noreferrer']]);
});
