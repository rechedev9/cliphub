import { existsSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

export const BASELINE_FILE = "comment-baseline.json";
export const REPORT_ENV = "CLIPHUB_COMMENT_REPORT";
export const REPORT_PREFIX = "comment-lines:";

const CONFIG_FILE = ".oxlintrc.json";
const FILE_START = { line: 1, column: 0 };
const REFERENCE = /^\/ <reference (path|types|lib)="[^"]+" \/>$/;
const LINT_DIRECTIVE = /^\s*(eslint|oxlint)-(disable|enable)/;

const baselines = new Map();

function lintRoot(filename) {
  let dir = path.dirname(filename);
  while (!existsSync(path.join(dir, CONFIG_FILE))) {
    const parent = path.dirname(dir);
    if (parent === dir) return path.dirname(filename);
    dir = parent;
  }
  return dir;
}

function baselineFor(root) {
  const file = path.join(root, BASELINE_FILE);
  const mtime = existsSync(file) ? statSync(file).mtimeMs : 0;
  const cached = baselines.get(root);
  if (cached?.mtime === mtime) return cached.frozen;
  const frozen = mtime === 0 ? {} : JSON.parse(readFileSync(file, "utf8"));
  baselines.set(root, { mtime, frozen });
  return frozen;
}

function counted(comment) {
  if (comment.type === "Shebang") return false;
  return !(comment.type === "Line" && REFERENCE.test(comment.value));
}

function lineCount(comment) {
  return comment.loc.end.line - comment.loc.start.line + 1;
}

function startLines(comments) {
  return comments.map((comment) => comment.loc.start.line).join(", ");
}

const noComments = {
  meta: {
    type: "problem",
    docs: { description: "Code has no comments. Files that had some before the rule are frozen at their count." },
  },
  create(context) {
    return {
      "Program:exit"() {
        const comments = context.sourceCode.getAllComments().filter(counted);
        const lines = comments.reduce((sum, comment) => sum + lineCount(comment), 0);
        const root = lintRoot(context.filename);
        const rel = path.relative(root, context.filename).split(path.sep).join("/");
        const report = (message) => context.report({ loc: FILE_START, message });
        if (process.env[REPORT_ENV]) {
          report(`${REPORT_PREFIX}${rel}=${lines}`);
          return;
        }
        const directives = comments.filter((comment) => LINT_DIRECTIVE.test(comment.value));
        if (directives.length > 0) {
          report(`Lint directives are not allowed (lines ${startLines(directives)}). Change the code or ${CONFIG_FILE} instead.`);
        }
        const frozen = baselineFor(root)[rel] ?? 0;
        if (frozen === 0 && lines > 0) {
          report(`Delete the comments at lines ${startLines(comments)}: code has no comments.`);
        } else if (lines > frozen) {
          report(`This file has ${lines} comment lines and is frozen at ${frozen}. Delete comments, never raise ${BASELINE_FILE}.`);
        } else if (lines < frozen) {
          report(`This file is down to ${lines} comment lines from ${frozen}. Run node ../tools/oxlint-no-comments/shrink-baseline.mjs to lower ${BASELINE_FILE}.`);
        }
      },
    };
  },
};

export default {
  meta: { name: "cliphub" },
  rules: { "no-comments": noComments },
};
