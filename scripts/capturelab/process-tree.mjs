import { spawn } from 'node:child_process';

const sleep = (milliseconds) => new Promise((accept) => setTimeout(accept, milliseconds));

export function processTreeSpawnOptions() {
  // A dedicated POSIX process group lets timeouts kill grandchildren (go run,
  // pnpm, Playwright, Next, browsers, FFmpeg) without touching the agent.
  return process.platform === 'win32' ? {} : { detached: true };
}

function signalPOSIXGroup(pid, signal) {
  try {
    process.kill(-pid, signal);
    return true;
  } catch (error) {
    if (error?.code === 'ESRCH') return false;
    throw error;
  }
}

async function taskkill(pid) {
  await new Promise((accept) => {
    const killer = spawn('taskkill', ['/pid', String(pid), '/t', '/f'], {
      stdio: 'ignore', windowsHide: true,
    });
    const timer = setTimeout(() => {
      killer.kill();
      accept();
    }, 15_000);
    const finish = () => {
      clearTimeout(timer);
      accept();
    };
    killer.once('error', finish);
    killer.once('close', finish);
  });
}

export async function terminateProcessTree(child, graceMS = 1_000) {
  if (!child?.pid) return;
  if (process.platform === 'win32') {
    await taskkill(child.pid);
    return;
  }
  if (!signalPOSIXGroup(child.pid, 'SIGTERM')) return;
  const deadline = Date.now() + graceMS;
  while (Date.now() < deadline) {
    await sleep(50);
    if (!signalPOSIXGroup(child.pid, 0)) return;
  }
  signalPOSIXGroup(child.pid, 'SIGKILL');
}
