/* compare: diff two chperf outputs (or any two JSON reports that follow the
 * unit-suffix convention) and flag cost metrics that got worse. */
#include "chperf.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

static int ends_with(const char *s, const char *suffix)
{
	size_t n = strlen(s), m = strlen(suffix);
	return n >= m && strcmp(s + n - m, suffix) == 0;
}

/* Lower-is-better metrics. Capacity, sizes of outputs and shares are not
 * costs: a larger share of one stage is not by itself a regression. */
int key_is_cost(const char *key)
{
	static const char *not_cost[] = {"share_",	 "free",	 "avail",	 "total_bytes",
					 "ram_",	 "output_bytes", "body_bytes",	 "start_ms",
					 "interval_ms", "media_duration", "utilization", "threshold",
					 "machine_", "demo_duration", "captured_video", "delivered_video",
					 "_cv_", "run_wall"};
	for (size_t i = 0; i < sizeof not_cost / sizeof not_cost[0]; i++)
		if (strstr(key, not_cost[i]))
			return 0;
	return ends_with(key, "_ms") || ends_with(key, "_s") || ends_with(key, "_bytes") || ends_with(key, "_percent") ||
	       ends_with(key, "_bytes_per_s") || ends_with(key, "_ratio_cost") || ends_with(key, "_per_op");
}

/* The smallest absolute change worth reporting, per unit. */
static double min_abs_for(const char *path)
{
	if (ends_with(path, "_ms") || strstr(path, "_ms."))
		return 20;
	if (ends_with(path, "_s") || strstr(path, "_s."))
		return 0.01;
	if (strstr(path, "_bytes"))
		return 1048576;
	if (strstr(path, "_percent"))
		return 1;
	return 0;
}

/* Array elements pair by every identity field they carry, so pipeline
 * operations match on stage AND name and spans on stage, label AND index. */
static const char *identity_keys[] = {"job_id", "revision", "stage", "name", "role", "pid", "path", "label", "index", "pkg"};
#define N_IDENTITY (sizeof identity_keys / sizeof identity_keys[0])

static int identity_label(const jval *obj, char *out, size_t cap)
{
	size_t len = 0;
	int any = 0;
	out[0] = 0;
	for (size_t i = 0; i < N_IDENTITY; i++) {
		const jval *v = jget(obj, identity_keys[i]);
		if (!v || (v->type != J_STR && v->type != J_NUM))
			continue;
		int n = v->type == J_STR ? snprintf(out + len, cap - len, "%s%s=%s", any ? "," : "", identity_keys[i], v->str)
					 : snprintf(out + len, cap - len, "%s%s=%g", any ? "," : "", identity_keys[i], v->num);
		if (n < 0 || (size_t)n >= cap - len)
			break;
		len += (size_t)n;
		any = 1;
	}
	return any;
}

static int same_identity(const jval *a, const jval *b)
{
	for (size_t i = 0; i < N_IDENTITY; i++) {
		const jval *x = jget(a, identity_keys[i]), *y = jget(b, identity_keys[i]);
		if (!x && !y)
			continue;
		if (!x || !y || x->type != y->type)
			return 0;
		if (x->type == J_STR ? strcmp(x->str, y->str) != 0 : x->num != y->num)
			return 0;
	}
	return 1;
}

static void push_change(changes_t *c, const change_t *ch)
{
	if (c->n == c->cap) {
		size_t cap = c->cap ? c->cap * 2 : 32;
		change_t *p = realloc(c->items, cap * sizeof *p);
		if (!p)
			return;
		c->items = p;
		c->cap = cap;
	}
	c->items[c->n++] = *ch;
}

void changes_free(changes_t *c)
{
	free(c->items);
	memset(c, 0, sizeof *c);
}

/* Stats children worth comparing; n/min/max/stddev are too noisy to gate on. */
static int stats_child_compared(const char *key)
{
	return strcmp(key, "p50") == 0 || strcmp(key, "p95") == 0;
}

static int looks_like_stats(const jval *obj)
{
	return obj && obj->type == J_OBJ && jget(obj, "n") && (jget(obj, "p50") || jget(obj, "min"));
}

void compare_values(const jval *base, const jval *cand, const char *path, int cost, double threshold_pct,
		    double min_abs, changes_t *out)
{
	if (!base || !cand || base->type != cand->type)
		return;
	char sub[512];
	if (base->type == J_NUM) {
		if (!cost)
			return;
		out->compared++;
		change_t ch = {0};
		snprintf(ch.path, sizeof ch.path, "%s", path);
		ch.base = base->num;
		ch.cand = cand->num;
		ch.delta = cand->num - base->num;
		ch.delta_percent = base->num != 0 ? 100.0 * ch.delta / fabs(base->num) : (ch.delta != 0 ? INFINITY : 0);
		double floor_abs = min_abs >= 0 ? min_abs : min_abs_for(path);
		if (fabs(ch.delta) < floor_abs || fabs(ch.delta_percent) < threshold_pct)
			ch.verdict = 0;
		else
			ch.verdict = ch.delta > 0 ? 1 : -1;
		if (ch.verdict)
			push_change(out, &ch);
		return;
	}
	if (base->type == J_OBJ) {
		int stats = looks_like_stats(base);
		for (size_t i = 0; i < base->n; i++) {
			const char *k = base->keys[i];
			if (stats && !stats_child_compared(k))
				continue;
			snprintf(sub, sizeof sub, "%s%s%s", path, path[0] ? "." : "", k);
			int child_cost = stats ? cost : key_is_cost(k);
			/* An object under a cost key (a stats block) inherits the unit. */
			if (!stats && base->items[i].type == J_OBJ && looks_like_stats(&base->items[i]))
				child_cost = key_is_cost(k);
			compare_values(&base->items[i], jget(cand, k), sub, child_cost, threshold_pct, min_abs, out);
		}
		return;
	}
	if (base->type == J_ARR) {
		for (size_t i = 0; i < base->n; i++) {
			const jval *b = &base->items[i], *c = NULL;
			char id[256];
			if (b->type == J_OBJ && identity_label(b, id, sizeof id)) {
				for (size_t j = 0; j < cand->n && !c; j++)
					if (same_identity(b, &cand->items[j]))
						c = &cand->items[j];
				snprintf(sub, sizeof sub, "%s[%s]", path, id);
			} else {
				c = jat(cand, i);
				snprintf(sub, sizeof sub, "%s[%zu]", path, i);
			}
			compare_values(b, c, sub, cost, threshold_pct, min_abs, out);
		}
	}
}

static int cmp_change_desc(const void *a, const void *b)
{
	double x = fabs(((const change_t *)a)->delta_percent), y = fabs(((const change_t *)b)->delta_percent);
	return (x < y) - (x > y);
}

static jval *load_json(ctx_t *ctx, const char *path, int *rc)
{
	size_t len;
	char *text = read_file(path, &len);
	if (!text) {
		*rc = fail(ctx, EXIT_RUNTIME, "read_failed", "cannot read %s", path);
		return NULL;
	}
	char err[160];
	jval *v = json_parse(text, len, err, sizeof err);
	free(text);
	if (!v)
		*rc = fail(ctx, EXIT_RUNTIME, "bad_json", "%s: %s", path, err);
	return v;
}

static void write_changes(jw_t *w, const char *key, const changes_t *c, int verdict, size_t limit)
{
	jw_arr(w, key);
	size_t shown = 0;
	for (size_t i = 0; i < c->n && shown < limit; i++) {
		const change_t *ch = &c->items[i];
		if (ch->verdict != verdict)
			continue;
		jw_obj(w, NULL);
		jw_str(w, "path", ch->path);
		jw_num(w, "base", ch->base);
		jw_num(w, "candidate", ch->cand);
		jw_num(w, "delta", ch->delta);
		jw_num(w, "delta_percent", ch->delta_percent);
		jw_end_obj(w);
		shown++;
	}
	jw_end_arr(w);
}

int cmd_compare(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *files[2] = {NULL, NULL}, *v;
	size_t nfiles = 0;
	long threshold = 10, limit = 25;
	double min_abs = -1;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--threshold-percent", &v)) {
			if (parse_int_arg(ctx, "--threshold-percent", v, 0, 1000, &threshold))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--limit", &v)) {
			if (parse_int_arg(ctx, "--limit", v, 1, 1000, &limit))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--min-abs", &v)) {
			long m;
			if (parse_int_arg(ctx, "--min-abs", v, 0, 0x7fffffff, &m))
				return EXIT_USAGE;
			min_abs = (double)m;
		} else if (argv[i][0] == '-' && argv[i][1] == '-') {
			return fail(ctx, EXIT_USAGE, "usage", "compare: unknown argument %s", argv[i]);
		} else if (nfiles < 2) {
			files[nfiles++] = argv[i];
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "compare takes exactly two files: BASE CANDIDATE");
		}
	}
	if (nfiles != 2)
		return fail(ctx, EXIT_USAGE, "usage", "compare takes exactly two files: BASE CANDIDATE");
	int rc = 0;
	jval *a = load_json(ctx, files[0], &rc);
	if (!a)
		return rc;
	jval *b = load_json(ctx, files[1], &rc);
	if (!b) {
		json_free(a);
		return rc;
	}
	/* chperf envelopes: compare their data blocks. */
	const jval *da = jget(a, "data") && jget(a, "tool") ? jget(a, "data") : a;
	const jval *db = jget(b, "data") && jget(b, "tool") ? jget(b, "data") : b;
	const char *ca = jstr(jget(a, "command"), NULL), *cb = jstr(jget(b, "command"), NULL);
	if (ca && cb && strcmp(ca, cb) != 0)
		finding(ctx, "warn", "compare_mismatch", "comparing a \"%s\" report with a \"%s\" report; only shared paths count",
			ca, cb);

	changes_t ch = {0};
	compare_values(da, db, "", 0, (double)threshold, min_abs, &ch);
	if (ch.n)
		qsort(ch.items, ch.n, sizeof *ch.items, cmp_change_desc);
	size_t reg = 0, imp = 0;
	for (size_t i = 0; i < ch.n; i++)
		ch.items[i].verdict > 0 ? reg++ : imp++;

	jw_obj(w, NULL);
	jw_obj(w, "base");
	jw_str(w, "file", files[0]);
	jw_str(w, "command", ca);
	jw_str(w, "generated_at", jstr(jget(a, "generated_at"), NULL));
	jw_end_obj(w);
	jw_obj(w, "candidate");
	jw_str(w, "file", files[1]);
	jw_str(w, "command", cb);
	jw_str(w, "generated_at", jstr(jget(b, "generated_at"), NULL));
	jw_end_obj(w);
	jw_int(w, "threshold_percent", threshold);
	jw_str(w, "rule", "cost metrics only (suffix _ms, _s, _bytes, _percent, _per_op); stats blocks compare p50 and p95; a "
			  "change counts when it passes both the percent threshold and a per-unit absolute floor");
	jw_int(w, "metrics_compared", (int64_t)ch.compared);
	jw_int(w, "regressions_count", (int64_t)reg);
	jw_int(w, "improvements_count", (int64_t)imp);
	jw_str(w, "verdict", reg ? "regress" : (imp ? "improve" : "same"));
	write_changes(w, "regressions", &ch, 1, (size_t)limit);
	write_changes(w, "improvements", &ch, -1, (size_t)limit);
	jw_end_obj(w);
	if (reg) {
		const change_t *worst = NULL;
		for (size_t i = 0; i < ch.n && !worst; i++)
			if (ch.items[i].verdict > 0)
				worst = &ch.items[i];
		ctx->threshold_hit = 1;
		finding(ctx, "warn", "compare_regress", "%zu cost metrics got worse by more than %ld%%; worst: %s (%+.1f%%)",
			reg, threshold, worst->path, worst->delta_percent);
	}
	if (ch.compared == 0)
		finding(ctx, "warn", "compare_empty", "no shared cost metric between the two files");
	changes_free(&ch);
	json_free(a);
	json_free(b);
	return 0;
}
