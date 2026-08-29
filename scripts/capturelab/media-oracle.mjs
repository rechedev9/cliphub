#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { readFile, stat, writeFile, mkdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { spawn } from 'node:child_process';

function parseArgs(args) {
  const out = { ffmpeg: 'ffmpeg', ffprobe: 'ffprobe' };
  const names = new Map([
    ['--recording-result', 'recordingResult'],
    ['--instrumentation', 'instrumentation'],
    ['--shorts-result', 'shortsResult'],
    ['--ffmpeg', 'ffmpeg'],
    ['--ffprobe', 'ffprobe'],
    ['--out', 'output'],
  ]);
  for (let index = 0; index < args.length; index++) {
    const key = names.get(args[index]);
    if (!key) throw new Error(`unknown argument ${args[index]}`);
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${args[index - 1]} requires a value`);
    out[key] = value;
  }
  if (!out.recordingResult || !out.instrumentation) {
    throw new Error('--recording-result and --instrumentation are required');
  }
  return out;
}

async function run(executable, args, binary = false) {
  return await new Promise((accept, reject) => {
    const child = spawn(executable, args, { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
    const stdout = [];
    const stderr = [];
    child.stdout.on('data', (chunk) => stdout.push(chunk));
    child.stderr.on('data', (chunk) => stderr.push(chunk));
    child.once('error', reject);
    child.once('close', (code) => {
      const err = Buffer.concat(stderr).toString('utf8').trim();
      if (code !== 0) return reject(new Error(`${executable} exited ${code}: ${err}`));
      const value = Buffer.concat(stdout);
      accept(binary ? value : value.toString('utf8'));
    });
  });
}

async function probe(ffprobe, path) {
  const raw = await run(ffprobe, [
    '-v', 'error', '-show_entries',
    'format=duration:format_tags=comment:stream=index,codec_type,codec_name,width,height,avg_frame_rate,sample_rate,channels,duration',
    '-of', 'json', path,
  ]);
  return JSON.parse(raw);
}

async function samplePixel(ffmpeg, path, seconds, x, y) {
  const raw = await run(ffmpeg, [
    '-v', 'error', '-ss', seconds.toFixed(3), '-i', path,
    '-vf', `crop=2:2:${x}:${y},format=rgb24`, '-frames:v', '1', '-f', 'rawvideo', '-',
  ], true);
  if (raw.length < 12) throw new Error(`pixel sample from ${path} returned ${raw.length} bytes`);
  const totals = [0, 0, 0];
  for (let offset = 0; offset < 12; offset += 3) {
    totals[0] += raw[offset];
    totals[1] += raw[offset + 1];
    totals[2] += raw[offset + 2];
  }
  return totals.map((value) => Math.round(value / 4));
}

async function frameSignature(ffmpeg, path, seconds) {
  const raw = await run(ffmpeg, [
    '-v', 'error', '-ss', seconds.toFixed(3), '-i', path,
    '-vf', 'scale=64:64,format=rgb24', '-frames:v', '1', '-f', 'rawvideo', '-',
  ], true);
  if (raw.length !== 64 * 64 * 3) throw new Error(`frame sample from ${path} returned ${raw.length} bytes`);
  return createHash('sha256').update(raw).digest('hex');
}

async function audioIdentity(ffmpeg, path, seconds) {
  const sampleRate = 48_000;
  const raw = await run(ffmpeg, [
    '-v', 'error', '-ss', seconds.toFixed(3), '-i', path, '-t', '0.75',
    '-vn', '-ac', '1', '-ar', String(sampleRate), '-f', 's16le', '-',
  ], true);
  if (raw.length < sampleRate) throw new Error(`audio sample from ${path} returned only ${raw.length} bytes`);
  let squareSum = 0;
  let upwardCrossings = 0;
  let previous = raw.readInt16LE(0);
  const samples = Math.floor(raw.length / 2);
  for (let index = 0; index < samples; index++) {
    const value = raw.readInt16LE(index * 2);
    squareSum += value * value;
    if (previous <= 0 && value > 0) upwardCrossings++;
    previous = value;
  }
  return {
    frequency_hz: upwardCrossings / (samples / sampleRate),
    rms: Math.sqrt(squareSum / samples),
  };
}

function expectedSegmentIdentity(segmentID, index) {
  const digest = createHash('sha256').update(segmentID).digest();
  const color = [0, 1, 2].map((position) => 64 + digest[position] % 160);
  return {
    color_rgb: color.map((value) => value.toString(16).padStart(2, '0')).join(''),
    tone_hz: 300 + (digest[3] + index * 97) % 600,
  };
}

export function protectedCaptureWindow(segment, plan) {
  const ticks = (segment.kills ?? []).map((kill) => kill.tick).filter((tick) => tick > 0);
  const tickrate = plan.tickrate > 0 ? plan.tickrate : 64;
  let start = segment.tick_start;
  if (ticks.length > 0) {
    const latestStart = Math.min(...ticks) - tickrate;
    if (latestStart > start) start = Math.min(start + tickrate * 2, latestStart);
  }
  let end = segment.tick_end;
  if (plan.demo_duration_ticks > 1 && end > 0) {
    let margin = Math.min(2 * tickrate, Math.floor(plan.demo_duration_ticks / 4));
    margin = Math.max(1, margin);
    const softCap = Math.max(1, plan.demo_duration_ticks - margin);
    if (end > softCap) {
      end = softCap;
      const utilityTicks = (segment.utility ?? []).flatMap((item) => [item.tick, item.throw_tick, item.pop_tick]).filter((tick) => tick > 0);
      const lastEvent = Math.max(0, ...ticks, ...utilityTicks);
      if (lastEvent > 0 && end < lastEvent) end = lastEvent + tickrate;
      if (end >= plan.demo_duration_ticks) end = lastEvent > 0 && lastEvent < plan.demo_duration_ticks - 1
        ? plan.demo_duration_ticks - 1 : plan.demo_duration_ticks;
      end = Math.min(end, plan.demo_duration_ticks);
      if (end < segment.tick_start || lastEvent > 0 && end < lastEvent) end = segment.tick_end;
    }
  }
  return { start, end };
}

function expectedEventOffsets(segment, plan, duration) {
  const { start, end } = protectedCaptureWindow(segment, plan);
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return [];
  return (segment.kills ?? []).map((kill) => Math.max(0, Math.min(duration - 0.15,
    (kill.tick - start) / (end - start) * duration)));
}

function parseColor(hex) {
  if (!/^[0-9a-f]{6}$/i.test(hex)) throw new Error(`invalid instrumentation color ${hex}`);
  return [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16));
}

function colorDistance(actual, expected) {
  return Math.max(...actual.map((value, index) => Math.abs(value - expected[index])));
}

function stream(probeResult, type) {
  return probeResult.streams?.find((candidate) => candidate.codec_type === type);
}

function rational(value) {
  const [numerator, denominator] = String(value).split('/').map(Number);
  return denominator ? numerator / denominator : 0;
}

async function sha256(path) {
  const content = await readFile(path);
  return createHash('sha256').update(content).digest('hex');
}

async function existsAndNonEmpty(path) {
  const info = await stat(path);
  return info.isFile() && info.size > 0;
}

function addCheck(report, name, ok, detail) {
  report.checks.push({ name, ok, detail });
  if (!ok) report.failures.push(`${name}: ${detail}`);
}

async function inspectMedia(report, tools, path, expected, kind) {
  let metadata;
  try {
    if (!await existsAndNonEmpty(path)) throw new Error('not a non-empty regular file');
    metadata = await probe(tools.ffprobe, path);
  } catch (error) {
    addCheck(report, `${kind}:${expected.id}:decodable`, false, error.message);
    return null;
  }
  addCheck(report, `${kind}:${expected.id}:decodable`, true, path);
  const video = stream(metadata, 'video');
  const audio = stream(metadata, 'audio');
  addCheck(report, `${kind}:${expected.id}:video`, Boolean(video), video?.codec_name ?? 'missing video stream');
  addCheck(report, `${kind}:${expected.id}:audio`, Boolean(audio), audio?.codec_name ?? 'missing audio stream');
  if (video) {
    const dimensionsOK = video.width === expected.width && video.height === expected.height;
    addCheck(report, `${kind}:${expected.id}:dimensions`, dimensionsOK, `${video.width}x${video.height}, want ${expected.width}x${expected.height}`);
    const fps = rational(video.avg_frame_rate);
    addCheck(report, `${kind}:${expected.id}:fps`, Math.abs(fps - expected.fps) < 0.01, `${fps}, want ${expected.fps}`);
  }
  if (audio) {
    const rate = Number(audio.sample_rate);
    addCheck(report, `${kind}:${expected.id}:audio-shape`, audio.channels >= 1 && rate >= 44100 && rate <= 192000,
      `${audio.channels}ch @ ${audio.sample_rate}Hz`);
  }
  const duration = Number(video?.duration ?? metadata.format?.duration);
  addCheck(report, `${kind}:${expected.id}:duration`, Math.abs(duration - expected.duration) <= 0.15,
    `${duration}s, want ${expected.duration}s`);
  report.probes.push({ kind, id: expected.id, path, metadata });
  return { metadata, video, audio, duration };
}

export async function verifyMedia(options) {
  const [recording, instrumentation] = await Promise.all([
    readFile(options.recordingResult, 'utf8').then(JSON.parse),
    readFile(options.instrumentation, 'utf8').then(JSON.parse),
  ]);
  const report = {
    schema_version: 1,
    ok: false,
    recording_result: resolve(options.recordingResult),
    instrumentation: resolve(options.instrumentation),
    shorts_result: options.shortsResult ? resolve(options.shortsResult) : '',
    checks: [], failures: [], probes: [], hashes: [],
  };
  addCheck(report, 'safety:capture-mode', recording.capture_mode === 'fake',
    `capture_mode=${JSON.stringify(recording.capture_mode)}, want "fake"`);
  addCheck(report, 'safety:not-verified', recording.capture_verified !== true,
    `capture_verified=${JSON.stringify(recording.capture_verified ?? false)}`);
  addCheck(report, 'instrumentation:schema', instrumentation.schema_version === 1 && instrumentation.capture_mode === 'fake',
    `schema=${instrumentation.schema_version} mode=${instrumentation.capture_mode}`);

  const artifacts = new Map((recording.artifacts ?? []).map((artifact) => [artifact.segment_id, artifact]));
  const instrumentationByID = new Map((instrumentation.segments ?? []).map((segment) => [segment.id, segment]));
  const planSegments = new Map((recording.plan?.segments ?? []).map((segment) => [segment.id, segment]));
  const editorialIDs = recording.plan?.editorial_segment_ids ?? [...planSegments.keys()];
  const fakeDurationSeconds = 5;
  const expectedSegments = editorialIDs.map((id, index) => {
    const planned = planSegments.get(id) ?? { id, kills: [] };
    return {
      id,
      ...expectedSegmentIdentity(id, index),
      duration_seconds: fakeDurationSeconds,
      event_offsets: expectedEventOffsets(planned, recording.plan ?? {}, fakeDurationSeconds),
    };
  });
  addCheck(report, 'segments:count', artifacts.size === expectedSegments.length && instrumentationByID.size === expectedSegments.length,
    `artifacts=${artifacts.size} instrumentation=${instrumentationByID.size} plan=${expectedSegments.length}`);
  const identityHashes = new Set();
  for (const segment of expectedSegments) {
    const declared = instrumentationByID.get(segment.id);
    addCheck(report, `instrumentation:${segment.id}:identity`, Boolean(declared) &&
      declared.color_rgb === segment.color_rgb && declared.tone_hz === segment.tone_hz &&
      JSON.stringify(declared.event_offsets ?? []) === JSON.stringify(segment.event_offsets),
    declared ? `declared color=${declared.color_rgb} tone=${declared.tone_hz} events=${JSON.stringify(declared.event_offsets ?? [])}` : 'missing');
    const artifact = artifacts.get(segment.id);
    addCheck(report, `segment:${segment.id}:artifact`, Boolean(artifact), artifact ? artifact.path : 'missing artifact');
    if (!artifact) continue;
    const path = resolve(artifact.path);
    const inspected = await inspectMedia(report, options, path, {
      id: segment.id, width: recording.plan?.stream?.width, height: recording.plan?.stream?.height,
      fps: recording.plan?.stream?.fps, duration: segment.duration_seconds,
    }, 'source');
    if (!inspected) continue;
    const comment = inspected.metadata.format?.tags?.comment;
    addCheck(report, `source:${segment.id}:metadata`, comment === `cliphub-capturelab:${segment.id}`,
      `comment=${JSON.stringify(comment)}`);
    const identity = await samplePixel(options.ffmpeg, path, 0.5, 10, 10);
    const expectedColor = parseColor(segment.color_rgb);
    addCheck(report, `source:${segment.id}:identity-pixel`, colorDistance(identity, expectedColor) <= 18,
      `rgb=${identity.join(',')} want≈${expectedColor.join(',')}`);
    const hash = createHash('sha256').update(Buffer.from(identity)).digest('hex');
    identityHashes.add(hash);
    report.hashes.push({ role: 'source-identity', id: segment.id, sha256: hash });
    const [earlyFrame, laterFrame, audio] = await Promise.all([
      frameSignature(options.ffmpeg, path, 0.5),
      frameSignature(options.ffmpeg, path, 1.5),
      audioIdentity(options.ffmpeg, path, 0.5),
    ]);
    addCheck(report, `source:${segment.id}:motion`, earlyFrame !== laterFrame,
      `frame hashes 0.5s=${earlyFrame} 1.5s=${laterFrame}`);
    addCheck(report, `source:${segment.id}:audio-level`, audio.rms >= 100,
      `rms=${audio.rms.toFixed(1)}`);
    addCheck(report, `source:${segment.id}:tone`, Math.abs(audio.frequency_hz - segment.tone_hz) <= 6,
      `${audio.frequency_hz.toFixed(2)}Hz, want ${segment.tone_hz}Hz`);
    for (const [index, eventOffset] of (segment.event_offsets ?? []).entries()) {
      const positiveTime = eventOffset + 0.05;
      const negativeTime = eventOffset >= 0.08 ? eventOffset - 0.08 : Math.min(segment.duration_seconds - 0.02, eventOffset + 0.2);
      const [pulse, negative] = await Promise.all([
        samplePixel(options.ffmpeg, path, positiveTime, 10, Math.max(0, artifact.height - 50)),
        samplePixel(options.ffmpeg, path, negativeTime, 10, Math.max(0, artifact.height - 50)),
      ]);
      addCheck(report, `source:${segment.id}:event-${index + 1}`, pulse.every((value) => value >= 235),
        `rgb=${pulse.join(',')} at ${positiveTime}s`);
      addCheck(report, `source:${segment.id}:event-${index + 1}-negative`, !negative.every((value) => value >= 235),
        `rgb=${negative.join(',')} at ${negativeTime}s`);
    }
  }
  addCheck(report, 'segments:unique-identities', identityHashes.size === expectedSegments.length,
    `unique identities=${identityHashes.size}, segments=${expectedSegments.length}`);

  if (options.shortsResult) {
    const result = JSON.parse(await readFile(options.shortsResult, 'utf8'));
    addCheck(report, 'render:executed', result.executed === true && !result.error,
      `executed=${result.executed} error=${JSON.stringify(result.error ?? '')}`);
    addCheck(report, 'render:fake-source-retained', resolve(result.recording_result) === resolve(options.recordingResult),
      `recording_result=${result.recording_result}`);
    const shorts = result.shorts ?? [];
    addCheck(report, 'render:short-count', shorts.length > 0, `shorts=${shorts.length}`);
    for (const item of shorts) {
      const output = resolve(item.output);
      const published = resolve(item.publish_path);
      const expectedWidth = item.output_format === 'landscape-16x9' ? 1920 : 1080;
      const expectedHeight = item.output_format === 'landscape-16x9' ? 1080 : 1920;
      const expectedRenderDuration = expectedSegments.length * fakeDurationSeconds;
      addCheck(report, `render:${item.segment_id}:declared-duration`,
        Math.abs(item.duration_seconds - expectedRenderDuration) <= 0.15,
        `${item.duration_seconds}s, want independently derived ${expectedRenderDuration}s`);
      addCheck(report, `render:${item.segment_id}:cut-contract`, item.transition === 'cut',
        `transition=${JSON.stringify(item.transition)}, Capture Lab render requires "cut"`);
      const inspected = await inspectMedia(report, options, published, {
        id: item.segment_id, width: expectedWidth, height: expectedHeight,
        fps: item.output_fps, duration: expectedRenderDuration,
      }, 'render');
      if (!inspected) continue;
      const [outputHash, publishHash] = await Promise.all([sha256(output), sha256(published)]);
      addCheck(report, `render:${item.segment_id}:published-copy`, outputHash === publishHash,
        `output=${outputHash} publish=${publishHash}`);
      report.hashes.push({ role: 'render', id: item.segment_id, sha256: publishHash });
      const parts = item.parts ?? [];
      const ids = parts.map((part) => part.segment_id);
      const expectedIDs = expectedSegments.map((segment) => segment.id);
      addCheck(report, `render:${item.segment_id}:part-order`, ids.join(',') === expectedIDs.join(','),
        `parts=${ids.join(',')} want=${expectedIDs.join(',')}`);
      if (expectedSegments.length > 0) {
        const identity = await samplePixel(options.ffmpeg, published, 0.5, 10, 10);
        const expectedColor = parseColor(expectedSegments[0].color_rgb);
        addCheck(report, `render:${item.segment_id}:identity-survives`, colorDistance(identity, expectedColor) <= 45,
          `rgb=${identity.join(',')} want≈${expectedColor.join(',')}`);
      }
      let timelineOffset = 0;
      for (const segment of expectedSegments) {
        const [earlyFrame, laterFrame, audio, segmentIdentity] = await Promise.all([
          frameSignature(options.ffmpeg, published, timelineOffset + 0.5),
          frameSignature(options.ffmpeg, published, timelineOffset + 1.5),
          audioIdentity(options.ffmpeg, published, timelineOffset + 0.5),
          samplePixel(options.ffmpeg, published, timelineOffset + 0.5, 10, 10),
        ]);
        const expectedColor = parseColor(segment.color_rgb);
        addCheck(report, `render:${item.segment_id}:${segment.id}:identity`, colorDistance(segmentIdentity, expectedColor) <= 45,
          `rgb=${segmentIdentity.join(',')} want≈${expectedColor.join(',')}`);
        addCheck(report, `render:${item.segment_id}:${segment.id}:motion`, earlyFrame !== laterFrame,
          `frame hashes +0.5s=${earlyFrame} +1.5s=${laterFrame}`);
        addCheck(report, `render:${item.segment_id}:${segment.id}:audio-level`, audio.rms >= 50,
          `rms=${audio.rms.toFixed(1)}`);
        addCheck(report, `render:${item.segment_id}:${segment.id}:tone`, Math.abs(audio.frequency_hz - segment.tone_hz) <= 8,
          `${audio.frequency_hz.toFixed(2)}Hz, want ${segment.tone_hz}Hz`);
        for (const [index, eventOffset] of (segment.event_offsets ?? []).entries()) {
          const positiveTime = timelineOffset + eventOffset + 0.05;
          const negativeOffset = eventOffset >= 0.08 ? eventOffset - 0.08 : Math.min(segment.duration_seconds - 0.02, eventOffset + 0.2);
          const negativeTime = timelineOffset + negativeOffset;
          const [pulse, negative] = await Promise.all([
            samplePixel(options.ffmpeg, published, positiveTime, 10, expectedHeight - 70),
            samplePixel(options.ffmpeg, published, negativeTime, 10, expectedHeight - 70),
          ]);
          addCheck(report, `render:${item.segment_id}:${segment.id}:event-${index + 1}`, pulse.every((value) => value >= 230),
            `rgb=${pulse.join(',')} at ${positiveTime}s`);
          addCheck(report, `render:${item.segment_id}:${segment.id}:event-${index + 1}-negative`, !negative.every((value) => value >= 230),
            `rgb=${negative.join(',')} at ${negativeTime}s`);
        }
        timelineOffset += segment.duration_seconds;
      }
    }
  }
  report.ok = report.failures.length === 0;
  return report;
}

const invokedAsCLI = process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href;
if (invokedAsCLI) {
  try {
    const options = parseArgs(process.argv.slice(2));
    const report = await verifyMedia(options);
    if (options.output) {
      await mkdir(dirname(options.output), { recursive: true });
      await writeFile(options.output, `${JSON.stringify(report, null, 2)}\n`, 'utf8');
    }
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    if (!report.ok) process.exitCode = 2;
  } catch (error) {
    process.stderr.write(`capturelab media oracle: ${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  }
}
