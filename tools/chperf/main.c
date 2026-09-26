#include "chperf.h"

#include <stdlib.h>
#include <string.h>
#ifdef _WIN32
#include <fcntl.h>
#include <io.h>
#endif

/* ---- help ---- */

typedef struct {
	const char *name, *summary;
	const char *args[8];
} cmd_help_t;

static const cmd_help_t help_table[] = {
	{"snapshot",
	 "Everything at once: system, pipeline stage history, recent renders, a short process sample and "
	 "orchestrator HTTP latency. Start here.",
	 {"--seconds N (process sample, default 3)", "--no-live (skip procs and http)", "--limit N (renders, default 3)"}},
	{"pipeline",
	 "Stage durations from <data>/obs/spans.jsonl (parse, scan, record, render...) with p50/p90, failures, "
	 "trend of the newest run, and the error journal by class.",
	 {"--since RFC3339", "--last N (stats over the newest N ok runs)"}},
	{"renders",
	 "Render evidence from render-result.json: render time, seconds per media second, per-stage Full Demo "
	 "ffmpeg timing, parallelism, slowest spans, shorts slot occupancy.",
	 {"--job ID", "--limit N (default 5)", "--full (all stages, spans and items)"}},
	{"procs",
	 "Samples ClipHub processes (Studio, zv-*, ffmpeg, cs2, hlae and their children): CPU %, working set, "
	 "private bytes, IO rate, by role.",
	 {"--seconds N (default 5)", "--interval-ms N (default 1000)", "--pid P (also sample this tree)",
	  "--match NAME (extra image name, repeatable)"}},
	{"run",
	 "Runs a command N times in a job object / process group and measures the whole process tree: wall, "
	 "CPU, peak memory, IO. Child output goes to a temp file, never to stdout.",
	 {"--repeat N", "--warmup N", "--timeout-s N", "--label TEXT", "--show-output", "-- COMMAND ARGS..."}},
	{"http",
	 "GET latency of the local orchestrator (loopback only; port from ports.json). Protected routes need the "
	 "Studio token in an env var.",
	 {"--url http://127.0.0.1:PORT", "--path /healthz (repeatable)", "--n N (default 30)", "--warmup N",
	  "--token-env NAME (default ZV_MUTATION_TOKEN)"}},
	{"captures",
	 "Game capture evidence from recording-result.json: capture vs video seconds, recorder phases, observed "
	 "fps, raw take bitrate with a black-capture detector, and capture+render end-to-end cost per job.",
	 {"--job ID", "--limit N (default 5)"}},
	{"bench",
	 "Runs Go benchmarks (go test -run ^$ -bench -benchmem -count N) or parses saved output; per-benchmark "
	 "ns/op, B/op, allocs/op medians and noise, comparable with compare.",
	 {"--pkg ./internal/X (repeatable)", "--bench REGEXP (default .)", "--count N (default 5)", "--benchtime 2s",
	  "--timeout-s N", "--from FILE", "--show-output"}},
	{"system", "Host facts that bound performance: CPU, RAM, OS, data dir and its free disk.", {NULL}},
	{"compare",
	 "Diffs two saved chperf outputs. Exit 1 when a cost metric regressed past the threshold.",
	 {"BASE.json CANDIDATE.json", "--threshold-percent N (default 10)", "--min-abs N", "--limit N"}},
	{"help", "This document.", {NULL}},
};

static void write_help(jw_t *w)
{
	jw_obj(w, NULL);
	jw_str(w, "usage", "chperf [--data-dir DIR] [--out FILE] [--pretty] <command> [args]");
	jw_str(w, "purpose", "Measure ClipHub performance and print one JSON document per call for an AI agent.");
	jw_arr(w, "commands");
	for (size_t i = 0; i < sizeof help_table / sizeof help_table[0]; i++) {
		jw_obj(w, NULL);
		jw_str(w, "name", help_table[i].name);
		jw_str(w, "summary", help_table[i].summary);
		jw_arr(w, "args");
		for (size_t a = 0; a < 8 && help_table[i].args[a]; a++)
			jw_str(w, NULL, help_table[i].args[a]);
		jw_end_arr(w);
		jw_end_obj(w);
	}
	jw_end_arr(w);
	jw_obj(w, "conventions");
	jw_str(w, "envelope", "{tool, version, schema, command, ok, generated_at, elapsed_ms, data | error, findings[]}");
	jw_str(w, "units", "key suffix: _ms milliseconds, _s seconds, _bytes, _percent, _per_s rate, _ratio unitless");
	jw_str(w, "stats", "{n, min, p50, p90, p95 (n>=5), p99 (n>=20), max, mean, stddev}; p50 is the median");
	jw_str(w, "findings", "pre-digested observations: level info|warn|error, stable code, one-line message");
	jw_str(w, "exit_codes", "0 ok, 1 regression or failed child run, 2 usage, 3 runtime error");
	jw_str(w, "save_and_compare", "chperf --out before.json snapshot; change code; chperf --out after.json snapshot; "
				      "chperf compare before.json after.json");
	jw_end_obj(w);
	jw_end_obj(w);
}

/* ---- snapshot ---- */

typedef int (*section_fn)(ctx_t *ctx, jw_t *w, void *arg);

typedef struct {
	long seconds, limit;
} snap_args_t;

static int sec_system(ctx_t *ctx, jw_t *w, void *a)
{
	(void)a;
	return system_report(ctx, w);
}
static int sec_pipeline(ctx_t *ctx, jw_t *w, void *a)
{
	(void)a;
	return pipeline_report(ctx, w, NULL, 20);
}
static int sec_renders(ctx_t *ctx, jw_t *w, void *a) { return renders_report(ctx, w, NULL, ((snap_args_t *)a)->limit, 0); }
static int sec_captures(ctx_t *ctx, jw_t *w, void *a) { return captures_report(ctx, w, NULL, ((snap_args_t *)a)->limit); }
static int sec_procs(ctx_t *ctx, jw_t *w, void *a)
{
	return procs_report(ctx, w, ((snap_args_t *)a)->seconds, 1000, 0, NULL, 0);
}
static int sec_http(ctx_t *ctx, jw_t *w, void *a)
{
	(void)a;
	return http_report(ctx, w, NULL, NULL, 0, 15, 2, "ZV_MUTATION_TOKEN");
}

/* Each section is rendered on its own so one failure (Studio closed, no
 * renders yet) becomes an error inside that section, not a failed snapshot. */
static void section(ctx_t *ctx, jw_t *w, const char *key, section_fn fn, void *arg)
{
	sb_t sb = {0};
	jw_t sw;
	jw_init(&sw, &sb, 0);
	ctx->err_code[0] = ctx->err_msg[0] = 0;
	int64_t t0 = plat_now_ns();
	int rc = fn(ctx, &sw, arg);
	double ms = (double)(plat_now_ns() - t0) / 1e6;
	jw_obj(w, key);
	jw_num(w, "section_elapsed_ms", ms);
	if (rc == 0) {
		/* Splice the section's object: drop its braces, keep the members. */
		if (sb.len > 2) {
			if (w->need_comma[w->depth])
				sb_puts(w->sb, ",");
			sb_add(w->sb, sb.p + 1, sb.len - 2);
			w->need_comma[w->depth] = 1;
		}
	} else {
		jw_obj(w, "error");
		jw_str(w, "code", ctx->err_code);
		jw_str(w, "message", ctx->err_msg);
		jw_end_obj(w);
	}
	jw_end_obj(w);
	sb_free(&sb);
	ctx->err_code[0] = ctx->err_msg[0] = 0;
}

static int cmd_snapshot(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	snap_args_t a = {3, 3};
	int live = 1;
	const char *v;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--seconds", &v)) {
			if (parse_int_arg(ctx, "--seconds", v, 1, 600, &a.seconds))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--limit", &v)) {
			if (parse_int_arg(ctx, "--limit", v, 1, 100, &a.limit))
				return EXIT_USAGE;
		} else if (arg_flag(argv, i, "--no-live")) {
			live = 0;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "snapshot: unknown argument %s", argv[i]);
		}
	}
	jw_obj(w, NULL);
	section(ctx, w, "system", sec_system, &a);
	section(ctx, w, "pipeline", sec_pipeline, &a);
	section(ctx, w, "renders", sec_renders, &a);
	section(ctx, w, "captures", sec_captures, &a);
	if (live) {
		section(ctx, w, "procs", sec_procs, &a);
		section(ctx, w, "http", sec_http, &a);
	}
	jw_end_obj(w);
	return 0;
}

/* ---- main ---- */

typedef int (*cmd_fn)(ctx_t *, int, char **, jw_t *);

static const struct {
	const char *name;
	cmd_fn fn;
	int needs_data;
} commands[] = {
	{"snapshot", cmd_snapshot, 1}, {"pipeline", cmd_pipeline, 1}, {"renders", cmd_renders, 1},
	{"procs", cmd_procs, 0},       {"run", cmd_run, 0},	      {"http", cmd_http, 1},
	{"system", cmd_system, 1},     {"compare", cmd_compare, 0},   {"captures", cmd_captures, 1},
	{"bench", cmd_bench, 0},
};

int main(int argc, char **argv)
{
	ctx_t ctx = {0};
	const char *data_flag = NULL, *out = NULL, *command = NULL;
	char **rest = calloc((size_t)argc + 1, sizeof *rest);
	int nrest = 0, usage_err = 0;
	char usage_msg[256] = "";
	if (!rest)
		return EXIT_RUNTIME;
	/* Global flags may sit anywhere before "--" so agents need not remember order. */
	int passthrough = 0;
	for (int i = 1; i < argc; i++) {
		const char *v;
		if (!passthrough && strcmp(argv[i], "--") == 0) {
			passthrough = 1;
			rest[nrest++] = argv[i];
		} else if (!passthrough && arg_flag(argv, i, "--pretty")) {
			ctx.pretty = 1;
		} else if (!passthrough && arg_val(argc, argv, &i, "--data-dir", &v)) {
			if (!v)
				usage_err = 1, snprintf(usage_msg, sizeof usage_msg, "--data-dir needs a directory");
			data_flag = v;
		} else if (!passthrough && arg_val(argc, argv, &i, "--out", &v)) {
			if (!v)
				usage_err = 1, snprintf(usage_msg, sizeof usage_msg, "--out needs a file path");
			out = v;
		} else if (!command && !passthrough) {
			command = argv[i];
		} else {
			rest[nrest++] = argv[i];
		}
	}
	if (!command || strcmp(command, "--help") == 0 || strcmp(command, "-h") == 0)
		command = "help";

	sb_t body = {0};
	jw_t bw;
	jw_init(&bw, &body, 0);
	int rc = EXIT_OK;
	int64_t t0 = plat_now_ns();
	if (usage_err) {
		rc = fail(&ctx, EXIT_USAGE, "usage", "%s", usage_msg);
	} else if (strcmp(command, "help") == 0) {
		write_help(&bw);
	} else {
		cmd_fn fn = NULL;
		int needs_data = 0;
		for (size_t i = 0; i < sizeof commands / sizeof commands[0]; i++)
			if (strcmp(commands[i].name, command) == 0)
				fn = commands[i].fn, needs_data = commands[i].needs_data;
		if (!fn) {
			rc = fail(&ctx, EXIT_USAGE, "unknown_command", "unknown command \"%s\"; run chperf help", command);
		} else {
			if (needs_data || data_flag)
				resolve_data_dir(&ctx, data_flag);
			rc = fn(&ctx, nrest, rest, &bw);
		}
	}
	double elapsed = (double)(plat_now_ns() - t0) / 1e6;

	sb_t env = {0};
	jw_t w;
	jw_init(&w, &env, 0);
	char now[40];
	plat_utc_iso(now);
	jw_obj(&w, NULL);
	jw_str(&w, "tool", "chperf");
	jw_str(&w, "version", CHPERF_VERSION);
	jw_int(&w, "schema", CHPERF_SCHEMA);
	jw_str(&w, "command", command);
	jw_bool(&w, "ok", rc == EXIT_OK);
	jw_str(&w, "generated_at", now);
	jw_num(&w, "elapsed_ms", elapsed);
	if (rc == EXIT_OK) {
		jw_raw(&w, "data", body.p ? body.p : "null", body.p ? body.len : 4);
	} else {
		jw_obj(&w, "error");
		jw_str(&w, "code", ctx.err_code[0] ? ctx.err_code : "error");
		jw_str(&w, "message", ctx.err_msg);
		jw_end_obj(&w);
	}
	jw_arr(&w, "findings");
	for (size_t i = 0; i < ctx.nfindings; i++) {
		jw_obj(&w, NULL);
		jw_str(&w, "level", ctx.findings[i].level);
		jw_str(&w, "code", ctx.findings[i].code);
		jw_str(&w, "message", ctx.findings[i].message);
		if (ctx.findings[i].hint[0])
			jw_str(&w, "hint", ctx.findings[i].hint);
		jw_end_obj(&w);
	}
	jw_end_arr(&w);
	jw_end_obj(&w);
	if (ctx.pretty) {
		/* Re-emit through the DOM: one writer path, indentation for free. */
		jval *doc = json_parse(env.p, env.len, NULL, 0);
		if (doc) {
			sb_t pretty = {0};
			jw_t pw;
			jw_init(&pw, &pretty, 1);
			jw_value(&pw, NULL, doc);
			json_free(doc);
			sb_free(&env);
			env = pretty;
		}
	}
	sb_puts(&env, "\n");

	if (rc == EXIT_OK && ctx.threshold_hit)
		rc = EXIT_THRESHOLD;
	if (out && write_file(out, env.p, env.len) != 0) {
		fprintf(stderr, "chperf: cannot write %s\n", out);
		if (rc == EXIT_OK)
			rc = EXIT_RUNTIME;
	}
#ifdef _WIN32
	/* Same bytes on stdout as in --out: no CRLF translation. */
	_setmode(_fileno(stdout), _O_BINARY);
#endif
	fwrite(env.p, 1, env.len, stdout);
	fflush(stdout);
	sb_free(&env);
	sb_free(&body);
	free(ctx.findings);
	free(rest);
	return rc;
}
