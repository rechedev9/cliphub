#include "chperf.h"

#include <errno.h>
#include <math.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>

/* ---- string builder ---- */

void sb_add(sb_t *sb, const char *s, size_t n)
{
	if (sb->len + n + 1 > sb->cap) {
		size_t cap = sb->cap ? sb->cap : 4096;
		while (cap < sb->len + n + 1)
			cap *= 2;
		char *p = realloc(sb->p, cap);
		if (!p) {
			fputs("chperf: out of memory\n", stderr);
			exit(EXIT_RUNTIME);
		}
		sb->p = p;
		sb->cap = cap;
	}
	memcpy(sb->p + sb->len, s, n);
	sb->len += n;
	sb->p[sb->len] = 0;
}

void sb_puts(sb_t *sb, const char *s) { sb_add(sb, s, strlen(s)); }

void sb_printf(sb_t *sb, const char *fmt, ...)
{
	char buf[1024];
	va_list ap;
	va_start(ap, fmt);
	int n = vsnprintf(buf, sizeof buf, fmt, ap);
	va_end(ap);
	if (n < 0)
		return;
	if ((size_t)n < sizeof buf) {
		sb_add(sb, buf, (size_t)n);
		return;
	}
	char *big = malloc((size_t)n + 1);
	if (!big)
		return;
	va_start(ap, fmt);
	vsnprintf(big, (size_t)n + 1, fmt, ap);
	va_end(ap);
	sb_add(sb, big, (size_t)n);
	free(big);
}

void sb_free(sb_t *sb)
{
	free(sb->p);
	sb->p = NULL;
	sb->len = sb->cap = 0;
}

/* ---- JSON writer ---- */

void jw_init(jw_t *w, sb_t *sb, int pretty)
{
	memset(w, 0, sizeof *w);
	w->sb = sb;
	w->pretty = pretty;
}

static void jw_indent(jw_t *w)
{
	sb_puts(w->sb, "\n");
	for (int i = 0; i < w->depth; i++)
		sb_puts(w->sb, "  ");
}

static void jw_escaped(sb_t *sb, const char *s, size_t n)
{
	sb_puts(sb, "\"");
	for (size_t i = 0; i < n; i++) {
		unsigned char c = (unsigned char)s[i];
		switch (c) {
		case '"': sb_puts(sb, "\\\""); break;
		case '\\': sb_puts(sb, "\\\\"); break;
		case '\n': sb_puts(sb, "\\n"); break;
		case '\r': sb_puts(sb, "\\r"); break;
		case '\t': sb_puts(sb, "\\t"); break;
		default:
			if (c < 0x20)
				sb_printf(sb, "\\u%04x", c);
			else
				sb_add(sb, (const char *)&s[i], 1);
		}
	}
	sb_puts(sb, "\"");
}

static void jw_pre(jw_t *w, const char *key)
{
	if (w->need_comma[w->depth])
		sb_puts(w->sb, ",");
	if (w->pretty && w->depth > 0)
		jw_indent(w);
	w->need_comma[w->depth] = 1;
	if (key) {
		jw_escaped(w->sb, key, strlen(key));
		sb_puts(w->sb, w->pretty ? ": " : ":");
	}
}

static void jw_open(jw_t *w, const char *key, const char *brace)
{
	jw_pre(w, key);
	sb_puts(w->sb, brace);
	if (w->depth < JW_MAX_DEPTH - 1)
		w->depth++;
	w->need_comma[w->depth] = 0;
}

static void jw_close(jw_t *w, const char *brace)
{
	int had = w->need_comma[w->depth];
	if (w->depth > 0)
		w->depth--;
	if (w->pretty && had)
		jw_indent(w);
	sb_puts(w->sb, brace);
}

void jw_obj(jw_t *w, const char *key) { jw_open(w, key, "{"); }
void jw_arr(jw_t *w, const char *key) { jw_open(w, key, "["); }
void jw_end_obj(jw_t *w) { jw_close(w, "}"); }
void jw_end_arr(jw_t *w) { jw_close(w, "]"); }

void jw_strn(jw_t *w, const char *key, const char *val, size_t n)
{
	jw_pre(w, key);
	jw_escaped(w->sb, val, n);
}

void jw_str(jw_t *w, const char *key, const char *val)
{
	if (!val) {
		jw_null(w, key);
		return;
	}
	jw_strn(w, key, val, strlen(val));
}

void jw_num(jw_t *w, const char *key, double val)
{
	jw_pre(w, key);
	if (!isfinite(val)) {
		sb_puts(w->sb, "null");
		return;
	}
	if (val == floor(val) && fabs(val) < 9e15) {
		sb_printf(w->sb, "%.0f", val);
		return;
	}
	/* Three decimals keep ms and ratios readable without float noise. */
	char buf[64];
	snprintf(buf, sizeof buf, "%.3f", val);
	size_t n = strlen(buf);
	while (n > 0 && buf[n - 1] == '0')
		buf[--n] = 0;
	if (n > 0 && buf[n - 1] == '.')
		buf[--n] = 0;
	if (strcmp(buf, "-0") == 0)
		strcpy(buf, "0");
	sb_puts(w->sb, buf);
}

void jw_int(jw_t *w, const char *key, int64_t val)
{
	jw_pre(w, key);
	sb_printf(w->sb, "%lld", (long long)val);
}

void jw_bool(jw_t *w, const char *key, int val)
{
	jw_pre(w, key);
	sb_puts(w->sb, val ? "true" : "false");
}

void jw_null(jw_t *w, const char *key)
{
	jw_pre(w, key);
	sb_puts(w->sb, "null");
}

void jw_raw(jw_t *w, const char *key, const char *raw, size_t n)
{
	jw_pre(w, key);
	sb_add(w->sb, raw, n);
}

void jw_value(jw_t *w, const char *key, const jval *v)
{
	switch (v->type) {
	case J_NULL: jw_null(w, key); break;
	case J_BOOL: jw_bool(w, key, v->boolean); break;
	case J_NUM: jw_num(w, key, v->num); break;
	case J_STR: jw_str(w, key, v->str); break;
	case J_ARR:
		jw_arr(w, key);
		for (size_t i = 0; i < v->n; i++)
			jw_value(w, NULL, &v->items[i]);
		jw_end_arr(w);
		break;
	case J_OBJ:
		jw_obj(w, key);
		for (size_t i = 0; i < v->n; i++)
			jw_value(w, v->keys[i], &v->items[i]);
		jw_end_obj(w);
		break;
	}
}

/* ---- stats ---- */

static int cmp_double(const void *a, const void *b)
{
	double x = *(const double *)a, y = *(const double *)b;
	return (x > y) - (x < y);
}

/* Nearest-rank percentile, the same definition the repo's efficiency script uses. */
double percentile_sorted(const double *sorted, size_t n, double pct)
{
	if (n == 0)
		return NAN;
	double rank = ceil(pct / 100.0 * (double)n);
	size_t idx = rank < 1 ? 0 : (size_t)rank - 1;
	if (idx >= n)
		idx = n - 1;
	return sorted[idx];
}

void stats_compute(const double *v, size_t n, stats_t *s)
{
	memset(s, 0, sizeof *s);
	s->n = n;
	if (n == 0) {
		s->min = s->max = s->mean = s->stddev = s->p50 = s->p90 = s->p95 = s->p99 = NAN;
		return;
	}
	double *c = malloc(n * sizeof *c);
	if (!c)
		return;
	memcpy(c, v, n * sizeof *c);
	qsort(c, n, sizeof *c, cmp_double);
	double sum = 0;
	for (size_t i = 0; i < n; i++)
		sum += c[i];
	s->mean = sum / (double)n;
	double var = 0;
	for (size_t i = 0; i < n; i++)
		var += (c[i] - s->mean) * (c[i] - s->mean);
	s->stddev = n > 1 ? sqrt(var / (double)(n - 1)) : 0;
	s->min = c[0];
	s->max = c[n - 1];
	/* The median is the conventional midpoint so even samples are not biased low. */
	s->p50 = (n % 2) ? c[n / 2] : (c[n / 2 - 1] + c[n / 2]) / 2.0;
	s->p90 = percentile_sorted(c, n, 90);
	s->p95 = percentile_sorted(c, n, 95);
	s->p99 = percentile_sorted(c, n, 99);
	free(c);
}

void jw_stats(jw_t *w, const char *key, const stats_t *s)
{
	jw_obj(w, key);
	jw_int(w, "n", (int64_t)s->n);
	if (s->n > 0) {
		jw_num(w, "min", s->min);
		jw_num(w, "p50", s->p50);
		if (s->n >= 5) {
			jw_num(w, "p90", s->p90);
			jw_num(w, "p95", s->p95);
		}
		if (s->n >= 20)
			jw_num(w, "p99", s->p99);
		jw_num(w, "max", s->max);
		jw_num(w, "mean", s->mean);
		if (s->n > 1)
			jw_num(w, "stddev", s->stddev);
	}
	jw_end_obj(w);
}

void dvec_push(dvec_t *d, double x)
{
	if (d->n == d->cap) {
		size_t cap = d->cap ? d->cap * 2 : 16;
		double *p = realloc(d->v, cap * sizeof *p);
		if (!p)
			return;
		d->v = p;
		d->cap = cap;
	}
	d->v[d->n++] = x;
}

void dvec_free(dvec_t *d)
{
	free(d->v);
	memset(d, 0, sizeof *d);
}

/* ---- files ---- */

char *read_file(const char *path, size_t *len)
{
	FILE *f = fopen(path, "rb");
	if (!f)
		return NULL;
	sb_t sb = {0};
	char buf[65536];
	size_t n;
	while ((n = fread(buf, 1, sizeof buf, f)) > 0)
		sb_add(&sb, buf, n);
	fclose(f);
	if (!sb.p)
		sb_add(&sb, "", 0);
	if (len)
		*len = sb.len;
	return sb.p;
}

int write_file(const char *path, const char *data, size_t len)
{
	FILE *f = fopen(path, "wb");
	if (!f)
		return -1;
	size_t n = fwrite(data, 1, len, f);
	int rc = fclose(f);
	return (n == len && rc == 0) ? 0 : -1;
}

void path_join(char *out, size_t cap, const char *a, const char *b)
{
	size_t la = strlen(a);
	int sep = la > 0 && a[la - 1] != '/' && a[la - 1] != '\\';
	snprintf(out, cap, "%s%s%s", a, sep ? "/" : "", b);
}

/* ---- findings and errors ---- */

void finding(ctx_t *ctx, const char *level, const char *code, const char *fmt, ...)
{
	if (ctx->nfindings == ctx->capfindings) {
		size_t cap = ctx->capfindings ? ctx->capfindings * 2 : 16;
		finding_t *p = realloc(ctx->findings, cap * sizeof *p);
		if (!p)
			return;
		ctx->findings = p;
		ctx->capfindings = cap;
	}
	finding_t *f = &ctx->findings[ctx->nfindings++];
	snprintf(f->level, sizeof f->level, "%s", level);
	snprintf(f->code, sizeof f->code, "%s", code);
	f->hint[0] = 0;
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(f->message, sizeof f->message, fmt, ap);
	va_end(ap);
}

void finding_hint(ctx_t *ctx, const char *hint)
{
	if (ctx->nfindings)
		snprintf(ctx->findings[ctx->nfindings - 1].hint, sizeof ctx->findings[0].hint, "%s", hint);
}

int fail(ctx_t *ctx, int exit_code, const char *code, const char *fmt, ...)
{
	snprintf(ctx->err_code, sizeof ctx->err_code, "%s", code);
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(ctx->err_msg, sizeof ctx->err_msg, fmt, ap);
	va_end(ap);
	return exit_code;
}

/* ---- args ---- */

int arg_val(int argc, char **argv, int *i, const char *name, const char **val)
{
	size_t n = strlen(name);
	const char *a = argv[*i];
	if (strncmp(a, name, n) != 0)
		return 0;
	if (a[n] == '=') {
		*val = a + n + 1;
		return 1;
	}
	if (a[n] != 0)
		return 0;
	if (*i + 1 >= argc) {
		*val = NULL;
		return 1;
	}
	*val = argv[++*i];
	return 1;
}

int arg_flag(char **argv, int i, const char *name) { return strcmp(argv[i], name) == 0; }

int parse_int_arg(ctx_t *ctx, const char *name, const char *s, long lo, long hi, long *out)
{
	if (!s)
		return fail(ctx, EXIT_USAGE, "usage", "%s needs a value", name);
	char *end = NULL;
	errno = 0;
	long v = strtol(s, &end, 10);
	if (errno || !end || *end || v < lo || v > hi)
		return fail(ctx, EXIT_USAGE, "usage", "%s must be an integer in [%ld, %ld], got \"%s\"", name, lo, hi, s);
	*out = v;
	return 0;
}
