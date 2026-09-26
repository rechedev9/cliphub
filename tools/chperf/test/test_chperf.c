/* Unit and end-to-end tests. Usage: chperf-test <path to built chperf>.
 * Run from tools/chperf (fixtures are relative). */
#include "chperf.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

static int failures, checks;

#define CHECK(cond, ...)                                                                                               \
	do {                                                                                                           \
		checks++;                                                                                              \
		if (!(cond)) {                                                                                         \
			failures++;                                                                                    \
			fprintf(stderr, "FAIL %s:%d: ", __FILE__, __LINE__);                                           \
			fprintf(stderr, __VA_ARGS__);                                                                  \
			fputc('\n', stderr);                                                                           \
		}                                                                                                      \
	} while (0)

#define FIXTURE_DATA "test/fixtures/data"

static jval *parse(const char *s) { return json_parse(s, strlen(s), NULL, 0); }

static void test_json_parser(void)
{
	jval *v = parse("\xEF\xBB\xBF {\"a\":[1,2.5,-3e2],\"s\":\"q\\\"\\n\\u00e9\\ud83d\\ude00\",\"t\":true,\"n\":null,\"a\":7}");
	CHECK(v != NULL, "BOM + nested document parses");
	CHECK(jnum(jget(v, "a"), 0) == 7, "duplicate key: last wins like encoding/json");
	CHECK(strcmp(jstr(jget(v, "s"), ""), "q\"\n\xC3\xA9\xF0\x9F\x98\x80") == 0, "escapes and surrogate pair decode to UTF-8");
	CHECK(jbool(jget(v, "t"), 0) == 1 && jget(v, "n")->type == J_NULL, "literals");
	json_free(v);

	char err[128];
	CHECK(json_parse("{\"a\":1} x", 9, err, sizeof err) == NULL && strstr(err, "trailing"), "trailing garbage rejected: %s", err);
	CHECK(json_parse("{\"a\":}", 6, err, sizeof err) == NULL, "missing value rejected");
	CHECK(json_parse("\"a\x01\"", 4, err, sizeof err) == NULL, "raw control char rejected");
	char deep[600];
	memset(deep, '[', 300);
	memset(deep + 300, ']', 300);
	CHECK(json_parse(deep, 600, err, sizeof err) == NULL && strstr(err, "deep"), "nesting cap stops stack exhaustion");
}

static void test_writer(void)
{
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	jw_obj(&w, NULL);
	jw_num(&w, "a", 0.1 + 0.2);
	jw_num(&w, "b", 272436);
	jw_num(&w, "c", NAN);
	jw_num(&w, "d", -0.0001);
	jw_str(&w, "e", "tab\there \"q\" \x01");
	jw_end_obj(&w);
	CHECK(strcmp(sb.p, "{\"a\":0.3,\"b\":272436,\"c\":null,\"d\":0,\"e\":\"tab\\there \\\"q\\\" \\u0001\"}") == 0, "writer output: %s",
	      sb.p);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	CHECK(v && strcmp(jstr(jget(v, "e"), ""), "tab\there \"q\" \x01") == 0, "writer output round-trips");
	json_free(v);
	sb_free(&sb);
}

static void test_stats(void)
{
	double v[] = {4, 1, 3, 2};
	stats_t s;
	stats_compute(v, 4, &s);
	CHECK(s.p50 == 2.5 && s.min == 1 && s.max == 4, "even-count median is the midpoint (got %g)", s.p50);
	double r[10] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
	stats_compute(r, 10, &s);
	CHECK(s.p90 == 9 && s.p95 == 10, "nearest-rank p90=%g p95=%g", s.p90, s.p95);
	stats_compute(r, 0, &s);
	CHECK(s.n == 0 && isnan(s.p50), "empty input has no median");
}

static void test_compare(void)
{
	CHECK(key_is_cost("wall_ms") && key_is_cost("render_s_per_media_s") && key_is_cost("working_set_peak_bytes"),
	      "cost keys");
	CHECK(!key_is_cost("share_of_tool_wall_percent") && !key_is_cost("data_disk_free_bytes") && !key_is_cost("runs") &&
		      !key_is_cost("output_bytes"),
	      "non-cost keys");

	/* Stages reordered between runs must still pair by name. */
	jval *a = parse("{\"stages\":[{\"stage\":\"items\",\"wall_ms\":1000},{\"stage\":\"mux\",\"wall_ms\":100}],"
			"\"total_ms\":{\"n\":3,\"min\":1,\"p50\":500,\"max\":900,\"stddev\":5},\"runs\":3}");
	jval *b = parse("{\"stages\":[{\"stage\":\"mux\",\"wall_ms\":105},{\"stage\":\"items\",\"wall_ms\":1300}],"
			"\"total_ms\":{\"n\":3,\"min\":1,\"p50\":400,\"max\":5000,\"stddev\":900},\"runs\":9}");
	changes_t c = {0};
	compare_values(a, b, "", 0, 10, -1, &c);
	int items_regress = 0, p50_improve = 0, other = 0;
	for (size_t i = 0; i < c.n; i++) {
		if (strcmp(c.items[i].path, "stages[stage=items].wall_ms") == 0 && c.items[i].verdict == 1)
			items_regress = 1;
		else if (strcmp(c.items[i].path, "total_ms.p50") == 0 && c.items[i].verdict == -1)
			p50_improve = 1;
		else
			other++, fprintf(stderr, "  unexpected change %s\n", c.items[i].path);
	}
	CHECK(items_regress, "items +30%% is a regression matched by stage name");
	CHECK(p50_improve, "stats p50 -20%% is an improvement");
	CHECK(other == 0, "mux +5%% is under threshold, max/stddev/runs are not gated");
	changes_free(&c);

	/* Operations share a stage; they must pair on stage AND name. */
	jval *o1 = parse("{\"operations\":[{\"stage\":\"worker\",\"name\":\"record\",\"last_ms\":400000},"
			 "{\"stage\":\"worker\",\"name\":\"parse\",\"last_ms\":2000}]}");
	jval *o2 = parse("{\"operations\":[{\"stage\":\"worker\",\"name\":\"parse\",\"last_ms\":2000},"
			 "{\"stage\":\"worker\",\"name\":\"record\",\"last_ms\":400000}]}");
	compare_values(o1, o2, "", 0, 10, -1, &c);
	CHECK(c.n == 0 && c.compared == 2, "reordered operations with a shared stage pair by name (%zu changes)", c.n);
	changes_free(&c);
	json_free(o1), json_free(o2);

	/* Absolute floor: 5 ms -> 8 ms is +60% but noise. */
	jval *x = parse("{\"t_ms\":5}"), *y = parse("{\"t_ms\":8}");
	compare_values(x, y, "", 0, 10, -1, &c);
	CHECK(c.n == 0, "sub-20ms change below the absolute floor is ignored");
	changes_free(&c);
	json_free(a), json_free(b), json_free(x), json_free(y);
}

static void test_pipeline_report(void)
{
	ctx_t ctx = {0};
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA);
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	CHECK(pipeline_report(&ctx, &w, NULL, 0) == 0, "pipeline report on fixture: %s", ctx.err_msg);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	CHECK(v != NULL, "pipeline output is JSON");
	const jval *ops = jget(v, "operations");
	CHECK(jnum(jget(v, "records"), 0) == 9 && jnum(jget(v, "unparsable_lines"), 0) == 1,
	      "rotated generation included, bad line counted");
	CHECK(ops && ops->n == 3, "three operations");
	const jval *first = jat(ops, 0);
	CHECK(strcmp(jstr(jget(first, "name"), ""), "record:demo") == 0, "slowest median first");
	/* record: 410000 (rotated, oldest), 400000, 420000, 900000 -> median 415000. */
	CHECK(jnum(jpath(first, "ok_duration_ms.p50"), 0) == 415000, "median across generations: %g",
	      jnum(jpath(first, "ok_duration_ms.p50"), 0));
	CHECK(fabs(jnum(jget(first, "newest_vs_prior_median_ratio"), 0) - 900000.0 / 410000.0) < 1e-3,
	      "newest vs prior median");
	const jval *render = NULL;
	for (size_t i = 0; ops && i < ops->n; i++)
		if (strcmp(jstr(jget(jat(ops, i), "name"), ""), "render:variant") == 0)
			render = jat(ops, i);
	CHECK(render && jnum(jget(render, "failed"), 0) == 1 && strcmp(jstr(jget(render, "last_result"), ""), "error") == 0,
	      "failed run counted and last result kept");
	CHECK(jnum(jpath(v, "errors.events"), 0) == 1, "error journal read");
	int slower = 0, failures_found = 0;
	for (size_t i = 0; i < ctx.nfindings; i++) {
		slower += strcmp(ctx.findings[i].code, "pipeline_slower") == 0;
		failures_found += strcmp(ctx.findings[i].code, "pipeline_failures") == 0;
	}
	CHECK(slower == 1 && failures_found == 1, "findings: slower=%d failures=%d", slower, failures_found);
	json_free(v);
	sb_free(&sb);
	free(ctx.findings);

	/* --since drops older records, --last trims the stats window. */
	ctx_t c2 = {0};
	snprintf(c2.data_dir, sizeof c2.data_dir, FIXTURE_DATA);
	jw_init(&w, &sb, 0);
	pipeline_report(&c2, &w, "2026-09-25T00:00:00Z", 1);
	v = json_parse(sb.p, sb.len, NULL, 0);
	CHECK(jnum(jget(v, "records"), 0) == 5, "since filter keeps 5 records, got %g", jnum(jget(v, "records"), 0));
	CHECK(jnum(jpath(jat(jget(v, "operations"), 0), "ok_duration_ms.n"), 0) == 1, "--last 1");
	json_free(v);
	sb_free(&sb);
	free(c2.findings);

	ctx_t c3 = {0};
	snprintf(c3.data_dir, sizeof c3.data_dir, "test/fixtures/missing");
	jw_init(&w, &sb, 0);
	CHECK(pipeline_report(&c3, &w, NULL, 0) == EXIT_RUNTIME && strcmp(c3.err_code, "no_span_journal") == 0,
	      "missing journal is a runtime error with a stable code");
	sb_free(&sb);
}

static const jval *find_render(const jval *doc, const char *revision)
{
	const jval *rs = jget(doc, "renders");
	for (size_t i = 0; rs && i < rs->n; i++)
		if (strcmp(jstr(jget(jat(rs, i), "revision"), ""), revision) == 0)
			return jat(rs, i);
	return NULL;
}

static void test_renders_report(void)
{
	ctx_t ctx = {0};
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA);
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	CHECK(renders_report(&ctx, &w, NULL, 10, 0) == 0, "renders report: %s", ctx.err_msg);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	CHECK(v && jnum(jget(v, "render_results_found"), 0) == 2, "both revisions found");
	const jval *fd = find_render(v, "rev-new"), *sp = find_render(v, "rev-old");
	CHECK(fd && strcmp(jstr(jget(fd, "kind"), ""), "full_demo") == 0, "full demo kind");
	const jval *t = jpath(jat(jget(fd, "items"), 0), "full_demo_timing");
	CHECK(jnum(jget(t, "outside_tool_spans_ms"), -1) == 1000, "render_ms - tool wall = time outside ffmpeg");
	CHECK(jnum(jget(t, "parallelism_ratio"), 0) == 2, "parallelism = elapsed sum / wall");
	CHECK(jnum(jget(t, "failed_spans"), 0) == 1, "cancelled span counted as not ok");
	const jval *top = jat(jget(t, "stages"), 0);
	CHECK(strcmp(jstr(jget(top, "stage"), ""), "items") == 0 &&
		      fabs(jnum(jget(top, "share_of_tool_wall_percent"), 0) - 66.667) < 0.01,
	      "stages sorted by wall with share");
	CHECK(jnum(jget(jat(jget(t, "top_spans"), 0), "elapsed_ms"), 0) == 6000, "slowest span first");
	CHECK(sp && strcmp(jstr(jget(sp, "kind"), ""), "shorts") == 0, "shorts kind");
	CHECK(jnum(jpath(sp, "render_ms.p50"), 0) == 4000, "reused output excluded from render_ms stats");
	CHECK(jnum(jpath(sp, "slot_post_encode_ms.max"), 0) == 800, "slot post-encode stats");
	CHECK(jnum(jpath(v, "full_demo_aggregate.render_s_per_media_s.p50"), 0) == 0.25, "aggregate rt factor");
	json_free(v);
	sb_free(&sb);
	free(ctx.findings);

	ctx_t c2 = {0};
	snprintf(c2.data_dir, sizeof c2.data_dir, FIXTURE_DATA);
	jw_init(&w, &sb, 0);
	renders_report(&c2, &w, "no-such-job", 10, 0);
	v = json_parse(sb.p, sb.len, NULL, 0);
	CHECK(jnum(jget(v, "render_results_found"), -1) == 0, "job filter");
	json_free(v);
	sb_free(&sb);
	free(c2.findings);
}

static const finding_t *find_finding(const ctx_t *ctx, const char *code)
{
	for (size_t i = 0; i < ctx->nfindings; i++)
		if (strcmp(ctx->findings[i].code, code) == 0)
			return &ctx->findings[i];
	return NULL;
}

static void test_render_bitrate(void)
{
	/* The rev-new fixture delivers 1 MB for 40 s: 0.2 Mb/s, a black-capture bitrate. */
	ctx_t ctx = {0};
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA);
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	renders_report(&ctx, &w, NULL, 10, 0);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	const jval *item = jat(jget(find_render(v, "rev-new"), "items"), 0);
	CHECK(fabs(jnum(jget(item, "output_bitrate_mbps"), 0) - 0.2) < 1e-9, "delivered bitrate");
	const finding_t *f = find_finding(&ctx, "render_suspect_black");
	CHECK(f && f->hint[0], "low delivered bitrate flagged with a hint");
	json_free(v);
	sb_free(&sb);
	free(ctx.findings);
}

static void test_captures_report(void)
{
	ctx_t ctx = {0};
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA);
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	CHECK(captures_report(&ctx, &w, "job-a", 5) == 0, "captures report: %s", ctx.err_msg);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	const jval *c = jat(jget(v, "captures"), 0);
	CHECK(c && jnum(jget(c, "segments"), 0) == 2 && jnum(jget(c, "captured_video_s"), 0) == 40, "segments summed");
	CHECK(jnum(jget(c, "demo_duration_s"), 0) == 100, "demo ticks / tick rate");
	CHECK(jnum(jget(c, "capture_s_per_video_s"), 0) == 0.5 && jnum(jget(c, "realtime_speedup_ratio"), 0) == 2,
	      "capture speed");
	CHECK(jnum(jget(c, "post_capture_ms"), 0) == 1000, "recorder total - prepare - capture");
	CHECK(jnum(jpath(c, "observed_capture_fps.p50"), 0) == 135, "observed fps stats");
	CHECK(jnum(jpath(c, "raw_video.takes"), 0) == 2 && jnum(jpath(c, "raw_video.bitrate_median_mbps"), 0) == 21.2,
	      "only raw takes count toward bitrate");
	CHECK(jnum(jpath(c, "raw_video.suspect_black_takes"), 0) == 1, "2.4 Mb/s 1080p60 take is suspect");
	const jval *e = jget(c, "end_to_end");
	CHECK(e && jnum(jget(e, "capture_ms"), 0) == 22000 &&
		      jnum(jget(e, "total_ms"), 0) == 22000 + jnum(jget(e, "render_ms"), NAN),
	      "end to end = recorder total + newest render");
	const finding_t *f = find_finding(&ctx, "capture_suspect_black");
	CHECK(f && strstr(f->message, "r2") && f->hint[0], "black take named with a hint");
	CHECK(find_finding(&ctx, "capture_speed") != NULL, "capture speed finding");
	json_free(v);
	sb_free(&sb);
	free(ctx.findings);
}

static void test_bench_report(void)
{
	size_t len;
	char *text = read_file("test/fixtures/bench.txt", &len);
	CHECK(text != NULL, "bench fixture readable");
	if (!text)
		return;
	ctx_t ctx = {0};
	sb_t sb = {0};
	jw_t w;
	jw_init(&w, &sb, 0);
	CHECK(bench_report(&ctx, &w, text, len, "fixture") == 0, "bench report");
	free(text);
	jval *v = json_parse(sb.p, sb.len, NULL, 0);
	const jval *bs = jget(v, "benchmarks");
	CHECK(strcmp(jstr(jget(v, "cpu"), ""), "Test CPU") == 0, "cpu line trimmed");
	CHECK(bs && bs->n == 4, "four benchmarks, got %zu", bs ? bs->n : 0);
	const jval *fast = jat(bs, 0), *noisy = jat(bs, 1), *inter = jat(bs, 2), *other = jat(bs, 3);
	CHECK(strcmp(jstr(jget(inter, "name"), ""), "BenchmarkInterleaved") == 0 && jnum(jget(inter, "samples"), 0) == 1 &&
		      jnum(jpath(inter, "ns_per_op.p50"), 0) == 1530115500,
	      "result after interleaved log lines joins its pending name; log lines are not samples");
	CHECK(strcmp(jstr(jget(fast, "name"), ""), "BenchmarkFast/slots-1/legacy") == 0, "GOMAXPROCS suffix stripped only");
	CHECK(jnum(jpath(fast, "ns_per_op.p50"), 0) == 101 && jnum(jpath(fast, "allocs_per_op.p50"), 0) == 2,
	      "per-op medians");
	CHECK(strcmp(jstr(jget(jat(jget(noisy, "metrics"), 0), "unit"), ""), "frames/op") == 0, "custom metric kept");
	CHECK(strcmp(jstr(jget(other, "pkg"), ""), "example.com/app/internal/b") == 0 &&
		      strcmp(jstr(jget(other, "name"), ""), "BenchmarkFast") == 0,
	      "same name in another package is a separate benchmark");
	CHECK(jnum(jget(v, "packages_failed"), 0) == 1 && ctx.threshold_hit, "FAIL line fails the run");
	CHECK(find_finding(&ctx, "bench_noisy") && find_finding(&ctx, "bench_failed") &&
		      find_finding(&ctx, "bench_few_samples"),
	      "noise, failure and thin-sample findings");
	CHECK(key_is_cost("ns_per_op") && key_is_cost("allocs_per_op") && !key_is_cost("ns_per_op_cv_percent"),
	      "per-op metrics are costs, noise is not");
	json_free(v);
	sb_free(&sb);
	free(ctx.findings);
}

static void test_orchestrator_port(void)
{
	ctx_t ctx = {0};
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA);
	CHECK(orchestrator_port(&ctx) == 1, "ports.json next to the data dir");
	snprintf(ctx.data_dir, sizeof ctx.data_dir, FIXTURE_DATA "/");
	CHECK(orchestrator_port(&ctx) == 1, "trailing slash tolerated");
}

/* Runs the real binary; returns its parsed --out document. */
static jval *run_cli(const char *bin, char **args, int *exit_code, run_result_t *rr)
{
	char out[1024];
	snprintf(out, sizeof out, "chperf-test-out-%ld.json", (long)(plat_now_ns() % 1000000));
	char *argv[32];
	int n = 0;
	argv[n++] = (char *)bin;
	argv[n++] = "--out";
	argv[n++] = out;
	for (int i = 0; args[i] && n < 31; i++)
		argv[n++] = args[i];
	argv[n] = NULL;
	CHECK(plat_run(argv, 120, 0, rr) == 0, "spawn %s: %s", bin, rr->err);
	*exit_code = rr->exit_code;
	size_t len;
	char *text = read_file(out, &len);
	remove(out);
	jval *v = text ? json_parse(text, len, NULL, 0) : NULL;
	free(text);
	return v;
}

static void test_cli(const char *bin)
{
	int code;
	run_result_t rr;
	char *a1[] = {"--data-dir", FIXTURE_DATA, "pipeline", NULL};
	jval *v = run_cli(bin, a1, &code, &rr);
	CHECK(code == 0 && v, "pipeline exit %d", code);
	CHECK(jbool(jget(v, "ok"), 0) && strcmp(jstr(jget(v, "command"), ""), "pipeline") == 0 &&
		      jget(v, "data") && jget(v, "findings"),
	      "envelope fields");
	CHECK(rr.output_bytes > 0 && strstr(rr.output_tail, "\"findings\":[") && strstr(rr.output_tail, "}\n"),
	      "stdout carries the JSON envelope too");
	json_free(v);

	char *a2[] = {"pipeline", "--bogus", NULL};
	v = run_cli(bin, a2, &code, &rr);
	CHECK(code == EXIT_USAGE && v && !jbool(jget(v, "ok"), 1) &&
		      strcmp(jstr(jpath(v, "error.code"), ""), "usage") == 0,
	      "unknown flag is a usage error (exit %d)", code);
	json_free(v);

	char *a3[] = {"http", "--url", "http://example.com:80", NULL};
	v = run_cli(bin, a3, &code, &rr);
	CHECK(code == EXIT_USAGE && strcmp(jstr(jpath(v, "error.code"), ""), "not_loopback") == 0,
	      "non-loopback URL refused");
	json_free(v);

	/* run measures a child; a failing child makes chperf exit 1 and shows its output. */
	char *a4[] = {"run", "--repeat", "2", "--", (char *)bin, "help", NULL};
	v = run_cli(bin, a4, &code, &rr);
	CHECK(code == 0 && jnum(jpath(v, "data.wall_ms.n"), 0) == 2 && jnum(jpath(v, "data.failed_runs"), -1) == 0,
	      "run --repeat 2 (exit %d)", code);
	CHECK(jnum(jpath(v, "data.output_bytes"), 0) > 100 && !jget(jget(v, "data"), "output_tail"),
	      "child output captured, tail hidden on success");
	json_free(v);

	char *a5[] = {"run", "--", (char *)bin, "no-such-command", NULL};
	v = run_cli(bin, a5, &code, &rr);
	CHECK(code == EXIT_THRESHOLD, "failing child -> exit 1 (got %d)", code);
	const jval *runs = jpath(v, "data.runs");
	CHECK(runs && jnum(jget(jat(runs, 0), "exit_code"), 0) == EXIT_USAGE, "child exit code reported");
	CHECK(strstr(jstr(jpath(v, "data.output_tail"), ""), "unknown_command") != NULL, "failing child's output shown");
	json_free(v);

	char *a6[] = {"--data-dir", FIXTURE_DATA, "snapshot", "--no-live", NULL};
	v = run_cli(bin, a6, &code, &rr);
	CHECK(code == 0 && jget(jget(v, "data"), "pipeline") && jget(jget(v, "data"), "renders") &&
		      !jget(jget(v, "data"), "procs"),
	      "snapshot --no-live sections");
	CHECK(jnum(jpath(v, "data.renders.render_results_found"), 0) == 2, "snapshot section body spliced");
	CHECK(jnum(jpath(v, "data.captures.recording_results_found"), 0) == 1, "snapshot captures section");
	const jval *fs = jget(v, "findings");
	int hinted = 0;
	for (size_t i = 0; fs && i < fs->n; i++)
		if (strcmp(jstr(jget(jat(fs, i), "code"), ""), "capture_suspect_black") == 0 && jstr(jget(jat(fs, i), "hint"), NULL))
			hinted = 1;
	CHECK(hinted, "findings carry their hint in the envelope");
	json_free(v);

	char *a8[] = {"bench", "--from", "test/fixtures/bench.txt", NULL};
	v = run_cli(bin, a8, &code, &rr);
	CHECK(code == EXIT_THRESHOLD && jnum(jpath(v, "data.packages_failed"), 0) == 1, "bench --from with a FAIL exits 1");
	json_free(v);

	char *a9[] = {"bench", NULL};
	v = run_cli(bin, a9, &code, &rr);
	CHECK(code == EXIT_USAGE, "bench without --pkg or --from is a usage error");
	json_free(v);

	char *a7[] = {"--data-dir", "test/fixtures/missing", "snapshot", "--no-live", NULL};
	v = run_cli(bin, a7, &code, &rr);
	CHECK(code == 0 && strcmp(jstr(jpath(v, "data.pipeline.error.code"), ""), "no_span_journal") == 0,
	      "a failing section is reported inside the snapshot");
	json_free(v);
}

int main(int argc, char **argv)
{
	test_json_parser();
	test_writer();
	test_stats();
	test_compare();
	test_pipeline_report();
	test_renders_report();
	test_render_bitrate();
	test_captures_report();
	test_bench_report();
	test_orchestrator_port();
	if (argc > 1)
		test_cli(argv[1]);
	else
		fprintf(stderr, "skipping CLI tests: pass the built chperf path\n");
	fprintf(stderr, "%d checks, %d failures\n", checks, failures);
	return failures ? 1 : 0;
}
