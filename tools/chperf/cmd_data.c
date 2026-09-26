/* Offline evidence ClipHub already records:
 *   pipeline: <data>/obs/spans.jsonl (+ rotated .1-.4) and journal.jsonl
 *   renders:  <data>/jobs/<job>/renders/<variant>/revisions/<rev>/render-result.json */
#include "chperf.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

#define SPAN_GENERATIONS 4 /* internal/obs spanJournalGenerations */

/* ---- pipeline ---- */

typedef struct {
	char stage[48], name[80];
	dvec_t ok_ms; /* chronological */
	size_t runs, ok, failed;
	double last_ms;
	char last_at[40], last_result[24];
} op_t;

typedef struct {
	char stage[48], cls[64];
	size_t count;
	char last_at[40];
	char last_message[256];
} errclass_t;

typedef struct {
	op_t *ops;
	size_t n, cap;
} ops_t;

static op_t *op_find(ops_t *o, const char *stage, const char *name)
{
	for (size_t i = 0; i < o->n; i++)
		if (strcmp(o->ops[i].stage, stage) == 0 && strcmp(o->ops[i].name, name) == 0)
			return &o->ops[i];
	if (o->n == o->cap) {
		size_t cap = o->cap ? o->cap * 2 : 16;
		op_t *p = realloc(o->ops, cap * sizeof *p);
		if (!p)
			return NULL;
		o->ops = p;
		o->cap = cap;
	}
	op_t *op = &o->ops[o->n++];
	memset(op, 0, sizeof *op);
	snprintf(op->stage, sizeof op->stage, "%s", stage);
	snprintf(op->name, sizeof op->name, "%s", name);
	return op;
}

/* Calls fn for every parsed JSON line of path. Returns lines read, -1 if absent. */
static long each_jsonl(const char *path, void (*fn)(const jval *, void *), void *arg, long *bad)
{
	size_t len;
	char *text = read_file(path, &len);
	if (!text)
		return -1;
	long lines = 0;
	char *p = text, *end = text + len;
	while (p < end) {
		char *nl = memchr(p, '\n', (size_t)(end - p));
		char *le = nl ? nl : end;
		size_t n = (size_t)(le - p);
		while (n && (p[n - 1] == '\r' || p[n - 1] == ' '))
			n--;
		if (n) {
			jval *v = json_parse(p, n, NULL, 0);
			if (v) {
				fn(v, arg);
				lines++;
			} else if (bad) {
				(*bad)++;
			}
			json_free(v);
		}
		p = nl ? nl + 1 : end;
	}
	free(text);
	return lines;
}

typedef struct {
	ops_t ops;
	const char *since;
	char first_at[40], last_at[40];
	long kept;
} span_scan_t;

static void on_span(const jval *v, void *arg)
{
	span_scan_t *s = arg;
	const char *at = jstr(jget(v, "time"), "");
	if (s->since && strcmp(at, s->since) < 0)
		return;
	op_t *op = op_find(&s->ops, jstr(jget(v, "stage"), "?"), jstr(jget(v, "name"), "?"));
	if (!op)
		return;
	const char *result = jstr(jget(v, "result"), "?");
	double ms = jnum(jget(v, "duration_ms"), NAN);
	op->runs++;
	if (strcmp(result, "ok") == 0) {
		op->ok++;
		if (isfinite(ms))
			dvec_push(&op->ok_ms, ms);
	} else {
		op->failed++;
	}
	op->last_ms = ms;
	snprintf(op->last_at, sizeof op->last_at, "%s", at);
	snprintf(op->last_result, sizeof op->last_result, "%s", result);
	if (!s->first_at[0] || strcmp(at, s->first_at) < 0)
		snprintf(s->first_at, sizeof s->first_at, "%s", at);
	if (strcmp(at, s->last_at) > 0)
		snprintf(s->last_at, sizeof s->last_at, "%s", at);
	s->kept++;
}

typedef struct {
	errclass_t *items;
	size_t n, cap;
	const char *since;
	long kept;
} err_scan_t;

static void on_error_event(const jval *v, void *arg)
{
	err_scan_t *s = arg;
	const char *at = jstr(jget(v, "time"), "");
	if (s->since && strcmp(at, s->since) < 0)
		return;
	const char *stage = jstr(jget(v, "stage"), "?"), *cls = jstr(jget(v, "class"), "?");
	errclass_t *e = NULL;
	for (size_t i = 0; i < s->n; i++)
		if (strcmp(s->items[i].stage, stage) == 0 && strcmp(s->items[i].cls, cls) == 0)
			e = &s->items[i];
	if (!e) {
		if (s->n == s->cap) {
			size_t cap = s->cap ? s->cap * 2 : 8;
			errclass_t *p = realloc(s->items, cap * sizeof *p);
			if (!p)
				return;
			s->items = p;
			s->cap = cap;
		}
		e = &s->items[s->n++];
		memset(e, 0, sizeof *e);
		snprintf(e->stage, sizeof e->stage, "%s", stage);
		snprintf(e->cls, sizeof e->cls, "%s", cls);
	}
	e->count++;
	s->kept++;
	if (strcmp(at, e->last_at) >= 0) {
		snprintf(e->last_at, sizeof e->last_at, "%s", at);
		snprintf(e->last_message, sizeof e->last_message, "%s", jstr(jget(v, "message"), ""));
	}
}

static int cmp_op_p50(const void *a, const void *b)
{
	const op_t *x = a, *y = b;
	stats_t sx, sy;
	stats_compute(x->ok_ms.v, x->ok_ms.n, &sx);
	stats_compute(y->ok_ms.v, y->ok_ms.n, &sy);
	double px = sx.n ? sx.p50 : -1, py = sy.n ? sy.p50 : -1;
	return (px < py) - (px > py);
}

int pipeline_report(ctx_t *ctx, jw_t *w, const char *since, long last)
{
	char obs[1100], path[1200];
	path_join(obs, sizeof obs, ctx->data_dir, "obs");
	span_scan_t s = {0};
	s.since = since;
	long bad = 0, files = 0;
	/* Oldest generation first so "last" means most recent. */
	for (int g = SPAN_GENERATIONS; g >= 0; g--) {
		char name[32];
		if (g)
			snprintf(name, sizeof name, "spans.jsonl.%d", g);
		else
			snprintf(name, sizeof name, "spans.jsonl");
		path_join(path, sizeof path, obs, name);
		if (each_jsonl(path, on_span, &s, &bad) >= 0)
			files++;
	}
	if (files == 0)
		return fail(ctx, EXIT_RUNTIME, "no_span_journal",
			    "no spans.jsonl under %s: no pipeline stage has finished with this data dir yet, or "
			    "--data-dir points elsewhere",
			    obs);

	if (s.ops.n)
		qsort(s.ops.ops, s.ops.n, sizeof *s.ops.ops, cmp_op_p50);
	double median_sum = 0;
	for (size_t i = 0; i < s.ops.n; i++) {
		op_t *op = &s.ops.ops[i];
		if (last > 0 && op->ok_ms.n > (size_t)last) {
			memmove(op->ok_ms.v, op->ok_ms.v + (op->ok_ms.n - (size_t)last), (size_t)last * sizeof(double));
			op->ok_ms.n = (size_t)last;
		}
		stats_t st;
		stats_compute(op->ok_ms.v, op->ok_ms.n, &st);
		if (st.n)
			median_sum += st.p50;
	}

	jw_obj(w, NULL);
	jw_str(w, "source", obs);
	jw_int(w, "journal_files", files);
	jw_int(w, "records", s.kept);
	if (bad)
		jw_int(w, "unparsable_lines", bad);
	if (since)
		jw_str(w, "since", since);
	if (last > 0)
		jw_int(w, "stats_last_n", last);
	jw_str(w, "first_at", s.first_at);
	jw_str(w, "last_at", s.last_at);
	jw_arr(w, "operations");
	for (size_t i = 0; i < s.ops.n; i++) {
		op_t *op = &s.ops.ops[i];
		stats_t st;
		stats_compute(op->ok_ms.v, op->ok_ms.n, &st);
		jw_obj(w, NULL);
		jw_str(w, "stage", op->stage);
		jw_str(w, "name", op->name);
		jw_int(w, "runs", (int64_t)op->runs);
		jw_int(w, "failed", (int64_t)op->failed);
		jw_stats(w, "ok_duration_ms", &st);
		if (st.n && median_sum > 0)
			jw_num(w, "share_of_median_sum_percent", 100.0 * st.p50 / median_sum);
		jw_num(w, "last_ms", op->last_ms);
		jw_str(w, "last_result", op->last_result);
		jw_str(w, "last_at", op->last_at);
		/* Trend: the newest ok run against the median of the ones before it. */
		if (op->ok_ms.n >= 3) {
			stats_t prior;
			stats_compute(op->ok_ms.v, op->ok_ms.n - 1, &prior);
			double newest = op->ok_ms.v[op->ok_ms.n - 1];
			double ratio = prior.p50 > 0 ? newest / prior.p50 : NAN;
			jw_num(w, "newest_vs_prior_median_ratio", ratio);
			if (ratio >= 1.5 && newest - prior.p50 >= 1000) {
				finding(ctx, "warn", "pipeline_slower",
					"%s newest ok run took %.0f ms, %.2fx the median of its %zu prior runs (%.0f ms)",
					op->name, newest, ratio, op->ok_ms.n - 1, prior.p50);
				/* Stage time scales with the demo; normalize before calling it a regression. */
				if (strstr(op->name, "record"))
					finding_hint(ctx, "a longer demo takes longer: check capture_s_per_video_s in chperf captures");
				else if (strstr(op->name, "render"))
					finding_hint(ctx, "a longer video takes longer: check render_s_per_media_s in chperf renders");
			}
		}
		jw_end_obj(w);
		if (op->failed)
			finding(ctx, "warn", "pipeline_failures", "%s failed %zu of %zu runs (last result: %s)", op->name,
				op->failed, op->runs, op->last_result);
	}
	jw_end_arr(w);
	if (s.ops.n && median_sum > 0) {
		op_t *top = &s.ops.ops[0];
		stats_t st;
		stats_compute(top->ok_ms.v, top->ok_ms.n, &st);
		if (st.n)
			finding(ctx, "info", "pipeline_dominant",
				"%s is the slowest stage: median %.0f ms, %.0f%% of the sum of stage medians", top->name,
				st.p50, 100.0 * st.p50 / median_sum);
	}

	err_scan_t e = {0};
	e.since = since;
	path_join(path, sizeof path, obs, "journal.jsonl");
	long elines = each_jsonl(path, on_error_event, &e, NULL);
	jw_obj(w, "errors");
	jw_bool(w, "journal_present", elines >= 0);
	jw_int(w, "events", e.kept);
	jw_arr(w, "by_class");
	for (size_t i = 0; i < e.n; i++) {
		jw_obj(w, NULL);
		jw_str(w, "stage", e.items[i].stage);
		jw_str(w, "class", e.items[i].cls);
		jw_int(w, "count", (int64_t)e.items[i].count);
		jw_str(w, "last_at", e.items[i].last_at);
		jw_str(w, "last_message", e.items[i].last_message);
		jw_end_obj(w);
	}
	jw_end_arr(w);
	jw_end_obj(w);
	jw_end_obj(w);

	for (size_t i = 0; i < s.ops.n; i++)
		dvec_free(&s.ops.ops[i].ok_ms);
	free(s.ops.ops);
	free(e.items);
	return 0;
}

int cmd_pipeline(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *since = NULL, *v;
	long last = 0;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--since", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--since needs an RFC 3339 UTC time, e.g. 2026-09-25T00:00:00Z");
			since = v;
		} else if (arg_val(argc, argv, &i, "--last", &v)) {
			if (parse_int_arg(ctx, "--last", v, 1, 100000, &last))
				return EXIT_USAGE;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "pipeline: unknown argument %s", argv[i]);
		}
	}
	return pipeline_report(ctx, w, since, last);
}

/* ---- renders ---- */

typedef struct {
	char path[1400];
	char source[16], job[80], variant[80], revision[80];
	int64_t mtime;
} render_file_t;

typedef struct {
	render_file_t *items;
	size_t n, cap;
	const char *job_filter;
	const char *source;
	char job[80], variant[80];
	char dir[1300];
} render_scan_t;

static void add_render(render_scan_t *s, const char *dir, const char *revision)
{
	char path[1400];
	path_join(path, sizeof path, dir, "render-result.json");
	int64_t mt;
	if (plat_file_mtime(path, &mt) != 0)
		return;
	if (s->n == s->cap) {
		size_t cap = s->cap ? s->cap * 2 : 16;
		render_file_t *p = realloc(s->items, cap * sizeof *p);
		if (!p)
			return;
		s->items = p;
		s->cap = cap;
	}
	render_file_t *r = &s->items[s->n++];
	memset(r, 0, sizeof *r);
	snprintf(r->path, sizeof r->path, "%s", path);
	snprintf(r->source, sizeof r->source, "%s", s->source);
	snprintf(r->job, sizeof r->job, "%s", s->job);
	snprintf(r->variant, sizeof r->variant, "%s", s->variant);
	snprintf(r->revision, sizeof r->revision, "%s", revision);
	r->mtime = mt;
}

static int on_revision(const char *name, int is_dir, void *arg)
{
	render_scan_t *s = arg;
	if (!is_dir)
		return 0;
	char dir[1400];
	path_join(dir, sizeof dir, s->dir, name);
	add_render(s, dir, name);
	return 0;
}

static int on_variant(const char *name, int is_dir, void *arg)
{
	render_scan_t *s = arg;
	if (!is_dir)
		return 0;
	char saved[1300], vdir[1300];
	snprintf(saved, sizeof saved, "%s", s->dir);
	path_join(vdir, sizeof vdir, saved, name);
	snprintf(s->variant, sizeof s->variant, "%s", name);
	add_render(s, vdir, "");
	path_join(s->dir, sizeof s->dir, vdir, "revisions");
	plat_list_dir(s->dir, on_revision, s);
	snprintf(s->dir, sizeof s->dir, "%s", saved);
	return 0;
}

static int on_job(const char *name, int is_dir, void *arg)
{
	render_scan_t *s = arg;
	if (!is_dir || (s->job_filter && strcmp(name, s->job_filter) != 0))
		return 0;
	char saved[1300];
	snprintf(saved, sizeof saved, "%s", s->dir);
	snprintf(s->job, sizeof s->job, "%s", name);
	char jdir[1300];
	path_join(jdir, sizeof jdir, saved, name);
	path_join(s->dir, sizeof s->dir, jdir, "renders");
	plat_list_dir(s->dir, on_variant, s);
	snprintf(s->dir, sizeof s->dir, "%s", saved);
	return 0;
}

static int cmp_render_mtime(const void *a, const void *b)
{
	const render_file_t *x = a, *y = b;
	return (x->mtime < y->mtime) - (x->mtime > y->mtime);
}

typedef struct {
	char stage[48];
	dvec_t wall_ms, share;
} stage_agg_t;

typedef struct {
	stage_agg_t items[64];
	size_t n;
} stage_aggs_t;

static stage_agg_t *stage_agg(stage_aggs_t *a, const char *stage)
{
	for (size_t i = 0; i < a->n; i++)
		if (strcmp(a->items[i].stage, stage) == 0)
			return &a->items[i];
	if (a->n == sizeof a->items / sizeof a->items[0])
		return NULL;
	stage_agg_t *s = &a->items[a->n++];
	memset(s, 0, sizeof *s);
	snprintf(s->stage, sizeof s->stage, "%s", stage);
	return s;
}

static int cmp_stage_wall(const void *a, const void *b)
{
	double x = jnum(jget(*(const jval *const *)a, "wall_ms"), 0), y = jnum(jget(*(const jval *const *)b, "wall_ms"), 0);
	return (x < y) - (x > y);
}

static int cmp_span_elapsed(const void *a, const void *b)
{
	double x = jnum(jget(*(const jval *const *)a, "elapsed_ms"), 0),
	       y = jnum(jget(*(const jval *const *)b, "elapsed_ms"), 0);
	return (x < y) - (x > y);
}

static void write_timing(ctx_t *ctx, jw_t *w, const char *key, const jval *t, double render_ms, int full,
			 stage_aggs_t *agg, const char *who)
{
	double wall = jnum(jget(t, "wall_ms"), NAN), sum = jnum(jget(t, "process_elapsed_sum_ms"), NAN);
	const jval *spans = jget(t, "spans"), *stages = jget(t, "stages");
	jw_obj(w, key);
	jw_num(w, "wall_ms", wall);
	jw_num(w, "process_elapsed_sum_ms", sum);
	if (wall > 0)
		jw_num(w, "parallelism_ratio", sum / wall);
	if (isfinite(render_ms) && isfinite(wall))
		jw_num(w, "outside_tool_spans_ms", render_ms - wall);
	size_t nspans = spans && spans->type == J_ARR ? spans->n : 0, failed = 0;
	for (size_t i = 0; i < nspans; i++)
		if (strcmp(jstr(jget(&spans->items[i], "outcome"), "ok"), "ok") != 0)
			failed++;
	jw_int(w, "spans", (int64_t)nspans);
	jw_int(w, "failed_spans", (int64_t)failed);
	if (jbool(jget(t, "truncated"), 0))
		jw_bool(w, "truncated", 1);
	if (failed)
		finding(ctx, "warn", "render_failed_spans", "%s: %zu tool spans ended with an outcome other than ok", who,
			failed);

	size_t ns = stages && stages->type == J_ARR ? stages->n : 0;
	const jval **order = ns ? malloc(ns * sizeof *order) : NULL;
	for (size_t i = 0; i < ns && order; i++)
		order[i] = &stages->items[i];
	if (order)
		qsort(order, ns, sizeof *order, cmp_stage_wall);
	jw_arr(w, "stages");
	size_t shown = full ? ns : (ns < 8 ? ns : 8);
	for (size_t i = 0; i < ns && order; i++) {
		const jval *st = order[i];
		const char *name = jstr(jget(st, "stage"), "?");
		double sw = jnum(jget(st, "wall_ms"), NAN), ss = jnum(jget(st, "process_elapsed_sum_ms"), NAN);
		double share = wall > 0 ? 100.0 * sw / wall : NAN;
		stage_agg_t *a = agg ? stage_agg(agg, name) : NULL;
		if (a) {
			dvec_push(&a->wall_ms, sw);
			dvec_push(&a->share, share);
		}
		if (i >= shown)
			continue;
		jw_obj(w, NULL);
		jw_str(w, "stage", name);
		jw_int(w, "spans", (int64_t)jnum(jget(st, "spans"), 0));
		jw_num(w, "wall_ms", sw);
		jw_num(w, "process_elapsed_sum_ms", ss);
		jw_num(w, "share_of_tool_wall_percent", share);
		if (sw > 0)
			jw_num(w, "parallelism_ratio", ss / sw);
		jw_end_obj(w);
	}
	jw_end_arr(w);
	if (ns > shown)
		jw_int(w, "stages_omitted", (int64_t)(ns - shown));
	free(order);

	const jval **sp = nspans ? malloc(nspans * sizeof *sp) : NULL;
	for (size_t i = 0; i < nspans && sp; i++)
		sp[i] = &spans->items[i];
	if (sp)
		qsort(sp, nspans, sizeof *sp, cmp_span_elapsed);
	jw_arr(w, "top_spans");
	for (size_t i = 0; i < nspans && sp && i < (size_t)(full ? 15 : 5); i++) {
		const jval *s = sp[i];
		jw_obj(w, NULL);
		jw_str(w, "stage", jstr(jget(s, "stage"), "?"));
		jw_int(w, "index", (int64_t)jnum(jget(s, "index"), 0));
		if (jget(s, "attempt"))
			jw_int(w, "attempt", (int64_t)jnum(jget(s, "attempt"), 0));
		if (jget(s, "variant"))
			jw_str(w, "variant", jstr(jget(s, "variant"), ""));
		if (jget(s, "label"))
			jw_str(w, "label", jstr(jget(s, "label"), ""));
		if (jget(s, "encoder"))
			jw_str(w, "encoder", jstr(jget(s, "encoder"), ""));
		jw_num(w, "start_ms", jnum(jget(s, "start_ms"), NAN));
		jw_num(w, "elapsed_ms", jnum(jget(s, "elapsed_ms"), NAN));
		jw_str(w, "outcome", jstr(jget(s, "outcome"), "?"));
		jw_end_obj(w);
	}
	jw_end_arr(w);
	free(sp);
	jw_end_obj(w);
}

static int cmp_perf_render_ms(const void *a, const void *b)
{
	double x = jnum(jpath(*(const jval *const *)a, "performance.render_ms"), 0),
	       y = jnum(jpath(*(const jval *const *)b, "performance.render_ms"), 0);
	return (x < y) - (x > y);
}

static void write_render(ctx_t *ctx, jw_t *w, const render_file_t *rf, const jval *doc, int full, stage_aggs_t *agg,
			 dvec_t *rt_factors)
{
	char iso[40];
	plat_unix_to_iso(rf->mtime, iso);
	const jval *shorts = jget(doc, "shorts");
	size_t n = shorts && shorts->type == J_ARR ? shorts->n : 0;
	size_t with_perf = 0;
	int full_demo = 0;
	const jval **perf = n ? malloc(n * sizeof *perf) : NULL;
	for (size_t i = 0; i < n && perf; i++) {
		const jval *sh = &shorts->items[i];
		if (jget(sh, "performance"))
			perf[with_perf++] = sh;
		if (jget(sh, "full_demo") || jpath(sh, "performance.full_demo_timing"))
			full_demo = 1;
	}
	jw_obj(w, NULL);
	jw_str(w, "source", rf->source);
	jw_str(w, "job_id", rf->job);
	jw_str(w, "variant", rf->variant);
	if (rf->revision[0])
		jw_str(w, "revision", rf->revision);
	jw_str(w, "written_at", iso);
	jw_str(w, "kind", full_demo ? "full_demo" : "shorts");
	jw_bool(w, "executed", jbool(jget(doc, "executed"), 0));
	jw_int(w, "outputs", (int64_t)n);
	jw_int(w, "outputs_with_performance", (int64_t)with_perf);
	if (jget(doc, "error"))
		jw_str(w, "error", jstr(jget(doc, "error"), ""));

	dvec_t render_ms = {0}, post_encode = {0};
	for (size_t i = 0; i < with_perf; i++) {
		const jval *p = jget(perf[i], "performance");
		double rms = jnum(jget(p, "render_ms"), NAN);
		if (isfinite(rms) && !jbool(jget(p, "reused"), 0))
			dvec_push(&render_ms, rms);
		double pe = jnum(jpath(p, "short_pack_timing.slot.post_encode_ms"), NAN);
		if (isfinite(pe))
			dvec_push(&post_encode, pe);
	}
	if (render_ms.n > 1) {
		stats_t st;
		stats_compute(render_ms.v, render_ms.n, &st);
		jw_stats(w, "render_ms", &st);
	}
	if (post_encode.n > 1) {
		stats_t st;
		stats_compute(post_encode.v, post_encode.n, &st);
		jw_stats(w, "slot_post_encode_ms", &st);
	}
	dvec_free(&render_ms);
	dvec_free(&post_encode);

	if (perf)
		qsort(perf, with_perf, sizeof *perf, cmp_perf_render_ms);
	size_t shown = full ? with_perf : (with_perf < 5 ? with_perf : 5);
	jw_arr(w, "items");
	for (size_t i = 0; i < shown; i++) {
		const jval *sh = perf[i], *p = jget(sh, "performance");
		double rms = jnum(jget(p, "render_ms"), NAN);
		double rt = jnum(jget(p, "render_seconds_per_media_second"), NAN);
		char who[160];
		snprintf(who, sizeof who, "job %.36s %s item %d", rf->job, rf->variant, (int)jnum(jget(sh, "index"), 0));
		jw_obj(w, NULL);
		jw_int(w, "index", (int64_t)jnum(jget(sh, "index"), 0));
		if (jget(sh, "segment_id"))
			jw_str(w, "segment_id", jstr(jget(sh, "segment_id"), ""));
		jw_num(w, "render_ms", rms);
		if (jget(p, "probe_ms"))
			jw_num(w, "probe_ms", jnum(jget(p, "probe_ms"), NAN));
		if (jget(p, "quality_check_ms"))
			jw_num(w, "quality_check_ms", jnum(jget(p, "quality_check_ms"), NAN));
		if (jget(p, "cover_ms"))
			jw_num(w, "cover_ms", jnum(jget(p, "cover_ms"), NAN));
		if (jget(p, "media_duration_seconds"))
			jw_num(w, "media_duration_s", jnum(jget(p, "media_duration_seconds"), NAN));
		if (isfinite(rt)) {
			jw_num(w, "render_s_per_media_s", rt);
			if (full_demo && rt_factors)
				dvec_push(rt_factors, rt);
		}
		double obytes = jnum(jget(p, "output_bytes"), NAN), media = jnum(jget(p, "media_duration_seconds"), NAN);
		if (isfinite(obytes))
			jw_num(w, "output_bytes", obytes);
		if (obytes > 0 && media > 0) {
			double mbps = obytes * 8.0 / media / 1e6;
			jw_num(w, "output_bitrate_mbps", mbps);
			/* A normal 1080p60 Full Demo delivers ~35-40 Mb/s; the 5.0.0
			 * black capture delivered 2.4 Mb/s and passed every other check. */
			if (full_demo && mbps < 6) {
				finding(ctx, "warn", "render_suspect_black",
					"%s delivered %.1f Mb/s; a normal 1080p60 Full Demo is ~35-40 Mb/s", who, mbps);
				finding_hint(ctx, "chperf captures --job <job> for raw take bitrate, then ffmpeg blackdetect on "
						  "the delivered file");
			}
		}
		if (jbool(jget(p, "reused"), 0))
			jw_bool(w, "reused", 1);
		const jval *fdt = jget(p, "full_demo_timing");
		if (fdt)
			write_timing(ctx, w, "full_demo_timing", fdt, rms, full, agg, who);
		const jval *spt = jget(p, "short_pack_timing");
		if (spt) {
			const jval *slot = jget(spt, "slot");
			jw_obj(w, "short_pack_timing");
			jw_num(w, "wall_ms", jnum(jget(spt, "wall_ms"), NAN));
			if (slot) {
				jw_num(w, "slot_held_ms", jnum(jget(slot, "held_ms"), NAN));
				jw_num(w, "slot_encode_ms", jnum(jget(slot, "encode_ms"), NAN));
				jw_num(w, "slot_post_encode_ms", jnum(jget(slot, "post_encode_ms"), NAN));
			}
			jw_end_obj(w);
		}
		jw_end_obj(w);
	}
	jw_end_arr(w);
	if (with_perf > shown)
		jw_int(w, "items_omitted", (int64_t)(with_perf - shown));
	jw_end_obj(w);
	free(perf);
}

int renders_report(ctx_t *ctx, jw_t *w, const char *job, long limit, int full)
{
	render_scan_t s = {0};
	s.job_filter = job;
	static const char *roots[] = {"jobs", "stream-jobs"};
	int any_root = 0;
	for (size_t r = 0; r < 2; r++) {
		path_join(s.dir, sizeof s.dir, ctx->data_dir, roots[r]);
		s.source = roots[r];
		if (plat_list_dir(s.dir, on_job, &s) == 0)
			any_root = 1;
	}
	if (!any_root)
		return fail(ctx, EXIT_RUNTIME, "no_jobs_dir", "no jobs directory under %s", ctx->data_dir);
	if (s.n)
		qsort(s.items, s.n, sizeof *s.items, cmp_render_mtime);

	stage_aggs_t *agg = calloc(1, sizeof *agg);
	dvec_t rt = {0};
	jw_obj(w, NULL);
	jw_str(w, "source", ctx->data_dir);
	jw_int(w, "render_results_found", (int64_t)s.n);
	if (job)
		jw_str(w, "job_filter", job);
	jw_arr(w, "renders");
	size_t shown = 0, unreadable = 0;
	for (size_t i = 0; i < s.n && shown < (size_t)limit; i++) {
		size_t len;
		char *text = read_file(s.items[i].path, &len);
		char err[128];
		jval *doc = text ? json_parse(text, len, err, sizeof err) : NULL;
		free(text);
		if (!doc) {
			unreadable++;
			continue;
		}
		write_render(ctx, w, &s.items[i], doc, full, agg, &rt);
		json_free(doc);
		shown++;
	}
	jw_end_arr(w);
	if (unreadable)
		jw_int(w, "unreadable_results", (int64_t)unreadable);

	/* Cross-render view: which Full Demo stage dominates, and is it stable. */
	jw_obj(w, "full_demo_aggregate");
	if (rt.n) {
		stats_t st;
		stats_compute(rt.v, rt.n, &st);
		jw_stats(w, "render_s_per_media_s", &st);
		if (rt.n >= 3) {
			stats_t prior;
			stats_compute(rt.v + 1, rt.n - 1, &prior); /* rt[0] is the newest */
			if (prior.p50 > 0 && rt.v[0] / prior.p50 >= 1.15)
				finding(ctx, "warn", "render_slower",
					"newest Full Demo render took %.3f s per media second, %.2fx the median of the "
					"%zu older renders shown (%.3f)",
					rt.v[0], rt.v[0] / prior.p50, rt.n - 1, prior.p50);
		}
	}
	jw_arr(w, "stages");
	double best_share = 0;
	const char *best = NULL;
	for (size_t i = 0; agg && i < agg->n; i++) {
		stats_t sw, sh;
		stats_compute(agg->items[i].wall_ms.v, agg->items[i].wall_ms.n, &sw);
		stats_compute(agg->items[i].share.v, agg->items[i].share.n, &sh);
		jw_obj(w, NULL);
		jw_str(w, "stage", agg->items[i].stage);
		jw_int(w, "renders", (int64_t)sw.n);
		jw_num(w, "median_wall_ms", sw.p50);
		jw_num(w, "median_share_of_tool_wall_percent", sh.p50);
		jw_end_obj(w);
		if (sh.p50 > best_share) {
			best_share = sh.p50;
			best = agg->items[i].stage;
		}
	}
	jw_end_arr(w);
	jw_end_obj(w);
	if (best)
		finding(ctx, "info", "render_dominant_stage",
			"Full Demo stage \"%s\" is the largest: median %.0f%% of tool wall time across the renders shown "
			"(stages overlap, so shares can sum past 100%%)",
			best, best_share);
	if (s.n == 0)
		finding(ctx, "info", "no_renders", "no render-result.json found under %s/jobs", ctx->data_dir);
	jw_end_obj(w);

	for (size_t i = 0; agg && i < agg->n; i++) {
		dvec_free(&agg->items[i].wall_ms);
		dvec_free(&agg->items[i].share);
	}
	free(agg);
	dvec_free(&rt);
	free(s.items);
	return 0;
}

int latest_render_of_job(ctx_t *ctx, const char *job, double *render_ms, double *media_s, char *variant, size_t vcap)
{
	render_scan_t s = {0};
	s.job_filter = job;
	s.source = "jobs";
	path_join(s.dir, sizeof s.dir, ctx->data_dir, "jobs");
	plat_list_dir(s.dir, on_job, &s);
	int found = -1;
	if (s.n) {
		qsort(s.items, s.n, sizeof *s.items, cmp_render_mtime);
		for (size_t i = 0; i < s.n && found < 0; i++) {
			size_t len;
			char *text = read_file(s.items[i].path, &len);
			jval *doc = text ? json_parse(text, len, NULL, 0) : NULL;
			free(text);
			const jval *shorts = jget(doc, "shorts");
			double ms = 0, media = 0;
			int any = 0;
			for (size_t k = 0; shorts && shorts->type == J_ARR && k < shorts->n; k++) {
				const jval *p = jget(&shorts->items[k], "performance");
				if (!p || jbool(jget(p, "reused"), 0))
					continue;
				ms += jnum(jget(p, "render_ms"), 0);
				media += jnum(jget(p, "media_duration_seconds"), 0);
				any = 1;
			}
			if (any) {
				*render_ms = ms;
				*media_s = media;
				snprintf(variant, vcap, "%s", s.items[i].variant);
				found = 0;
			}
			json_free(doc);
		}
	}
	free(s.items);
	return found;
}

int cmd_renders(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *job = NULL, *v;
	long limit = 5;
	int full = 0;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--job", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--job needs a job id");
			job = v;
		} else if (arg_val(argc, argv, &i, "--limit", &v)) {
			if (parse_int_arg(ctx, "--limit", v, 1, 1000, &limit))
				return EXIT_USAGE;
		} else if (arg_flag(argv, i, "--full")) {
			full = 1;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "renders: unknown argument %s", argv[i]);
		}
	}
	return renders_report(ctx, w, job, limit, full);
}
