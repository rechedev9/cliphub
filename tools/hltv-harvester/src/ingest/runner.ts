import { spawn, type SpawnOptions } from "node:child_process";
import { createReadStream } from "node:fs";

export type Command = { cmd: string; args: string[]; cwd?: string; stdinFile?: string; shell?: boolean };

export type Result = { code: number; stdout: string };

export type Runner = {
  dryRun: boolean;
  run(c: Command): Promise<Result>;
  act(description: string, fn: () => void): void;
};

function quote(arg: string): string {
  return /^[\w./:\\=@,+-]+$/.test(arg) ? arg : `"${arg.replaceAll('"', '\\"')}"`;
}

export function describe(c: Command): string {
  const line = [c.cmd, ...c.args].map(quote).join(" ");
  const cwd = c.cwd === undefined ? "" : ` (in ${c.cwd})`;
  const stdin = c.stdinFile === undefined ? "" : ` < ${c.stdinFile}`;
  return `${line}${stdin}${cwd}`;
}

export function dryRunner(print: (line: string) => void): Runner {
  return {
    dryRun: true,
    run: async (c) => {
      print(`[dry-run] run ${describe(c)}`);
      return { code: 0, stdout: "" };
    },
    act: (description) => print(`[dry-run] ${description}`),
  };
}

export function spawnRunner(echo: (line: string) => void): Runner {
  return {
    dryRun: false,
    run: (c) =>
      new Promise((resolve) => {
        const options: SpawnOptions = { cwd: c.cwd, stdio: [c.stdinFile === undefined ? "ignore" : "pipe", "pipe", "pipe"], windowsHide: true };
        const child = c.shell === true ? spawn([c.cmd, ...c.args].join(" "), { ...options, shell: true }) : spawn(c.cmd, c.args, options);
        let stdout = "";
        child.stdout?.setEncoding("utf8").on("data", (d: string) => { stdout += d; });
        child.stderr?.setEncoding("utf8").on("data", (d: string) => { for (const l of d.split(/\r?\n/)) if (l !== "") echo(l); });
        if (c.stdinFile !== undefined && child.stdin !== null) createReadStream(c.stdinFile).pipe(child.stdin);
        child.on("error", (err) => {
          echo(`${c.cmd}: ${String(err)}`);
          resolve({ code: 127, stdout });
        });
        child.on("close", (code) => resolve({ code: code ?? 1, stdout }));
      }),
    act: (_description, fn) => fn(),
  };
}
