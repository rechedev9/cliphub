import { execFile } from 'node:child_process';

// The capture and render workers the orchestrator spawns; while one lives, replacing Studio would cut its job.
const JOB_IMAGES: ReadonlySet<string> = new Set(['zv-recorder.exe', 'zv-editor.exe']);

/** Whether `tasklist /FO CSV /NH` output lists a capture or render worker. */
export function tasklistHasStudioJob(csv: string): boolean {
  for (const line of csv.split(/\r?\n/)) {
    const image = /^"([^"]+)"/.exec(line)?.[1];
    if (image !== undefined && JOB_IMAGES.has(image.toLowerCase())) return true;
  }
  return false;
}

function readTasklist(): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile('tasklist', ['/FO', 'CSV', '/NH'], { windowsHide: true, timeout: 10_000 }, (error, stdout) => {
      if (error !== null) reject(error);
      else resolve(stdout);
    });
  });
}

/** True while a capture or render runs, and when the process list cannot be read. */
export async function studioJobsRunning(list: () => Promise<string> = readTasklist): Promise<boolean> {
  try {
    return tasklistHasStudioJob(await list());
  } catch {
    return true;
  }
}
