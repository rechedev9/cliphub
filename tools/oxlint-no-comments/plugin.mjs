import { existsSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

export const BASELINE_FILE = "comment-baseline.json";
export const REPORT_ENV = "CLIPHUB_COMMENT_REPORT";
export const REPORT_PREFIX = "comment-lines:";

const baselines = new Map();

function packageRoot(filename) {
  let dir = path.dirname(filename);
  while (!existsSync(path.join(dir, "package.json"))) {
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

function isDirective(comment) {
  return comment.type === "Line" && comment.value.startsWith("/ <reference ");
}

function lineCount(comment) {
  return comment.loc.end.line - comment.loc.start.line + 1;
}

const noComments = {
  meta: {
    type: "problem",
    docs: { description: "Code has no comments. Files that had some before the rule are frozen at their count." },
  },
  create(context) {
    return {
      "Program:exit"() {
        const comments = context.sourceCode.getAllComments().filter((comment) => !isDirective(comment));
        const lines = comments.reduce((sum, comment) => sum + lineCount(comment), 0);
        const root = packageRoot(context.filename);
        const rel = path.relative(root, context.filename).split(path.sep).join("/");
        if (process.env[REPORT_ENV]) {
          if (lines > 0) context.report({ loc: comments[0].loc, message: `${REPORT_PREFIX}${rel}=${lines}` });
          return;
        }
        const frozen = baselineFor(root)[rel] ?? 0;
        if (frozen === 0) {
          for (const comment of comments) {
            context.report({ loc: comment.loc, message: "Delete this comment: code has no comments." });
          }
          return;
        }
        if (lines > frozen) {
          context.report({
            loc: comments[0].loc,
            message: `This file has ${lines} comment lines and is frozen at ${frozen}. Delete comments, never raise ${BASELINE_FILE}.`,
          });
        } else if (lines < frozen) {
          context.report({
            loc: { line: 1, column: 0 },
            message: `This file is down to ${lines} comment lines from ${frozen}. Run node ../tools/oxlint-no-comments/shrink-baseline.mjs to lower ${BASELINE_FILE}.`,
          });
        }
      },
    };
  },
};

export default {
  meta: { name: "cliphub" },
  rules: { "no-comments": noComments },
};
