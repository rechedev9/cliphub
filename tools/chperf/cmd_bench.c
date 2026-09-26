/* bench: Go microbenchmarks as JSON. Runs `go test -bench` (or reads saved
 * output with --from) and reports per-benchmark medians over -count samples,
 * so `chperf compare` can gate a before/after like benchstat. */
#include "chperf.h"

#include <ctype.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

#define MAX_UNITS 12

typedef struct {
	char unit[32];
	dvec_t v;
} unit_series_t;

typedef struct {
	char pkg[256], name[256];
	unit_series_t units[MAX_UNITS];
	size_t nunits;
	size_t samples;
} bench_t;

typedef struct {
	bench_t *items;
	size_t n, cap;
	char pkg[256], goos[32], goarch[32], cpu[160];
	char failed[1024];
	size_t nfailed, nok;
	char pending[256]; /* benchmark name printed, result not yet seen */
} bench_set_t;

/* "1000000  101.0 ns/op ...": all-digit iterations, a number, a unit. */
static int is_result(char **tok, size_t nt)
{
	if (nt < 3 || !tok[0][0])
		return 0;
	for (const char *p = tok[0]; *p; p++)
		if (!isdigit((unsigned char)*p))
			return 0;
	char *end = NULL;
	strtod(tok[1], &end);
	return end && end != tok[1] && *end == 0 && strchr(tok[2], '/') != NULL;
}

static bench_t *bench_get(bench_set_t *s, const char *pkg, const char *name)
{
	for (size_t i = 0; i < s->n; i++)
		if (strcmp(s->items[i].pkg, pkg) == 0 && strcmp(s->items[i].name, name) == 0)
			return &s->items[i];
	if (s->n == s->cap) {
		size_t cap = s->cap ? s->cap * 2 : 32;
		bench_t *p = realloc(s->items, cap * sizeof *p);
		if (!p)
			return NULL;
		s->items = p;
		s->cap = cap;
	}
	bench_t *b = &s->items[s->n++];
	memset(b, 0, sizeof *b);
	snprintf(b->pkg, sizeof b->pkg, "%s", pkg);
	snprintf(b->name, sizeof b->name, "%s", name);
	return b;
}

static void bench_add(bench_t *b, const char *unit, double v)
{
	for (size_t i = 0; i < b->nunits; i++)
		if (strcmp(b->units[i].unit, unit) == 0) {
			dvec_push(&b->units[i].v, v);
			return;
		}
	if (b->nunits == MAX_UNITS)
		return;
	unit_series_t *u = &b->units[b->nunits++];
	memset(u, 0, sizeof *u);
	snprintf(u->unit, sizeof u->unit, "%s", unit);
	dvec_push(&u->v, v);
}

/* "BenchmarkFoo/sub-16" -> "BenchmarkFoo/sub": the suffix is GOMAXPROCS. */
static void strip_procs(char *name)
{
	char *dash = strrchr(name, '-');
	if (!dash || !dash[1])
		return;
	for (char *p = dash + 1; *p; p++)
		if (!isdigit((unsigned char)*p))
			return;
	*dash = 0;
}

static void copy_after(char *dst, size_t cap, const char *line, const char *prefix)
{
	const char *v = line + strlen(prefix);
	while (*v == ' ' || *v == '\t')
		v++;
	snprintf(dst, cap, "%s", v);
	size_t l = strlen(dst);
	while (l && (dst[l - 1] == ' ' || dst[l - 1] == '\t'))
		dst[--l] = 0;
}

static void parse_line(bench_set_t *s, char *line)
{
	if (strncmp(line, "pkg:", 4) == 0) {
		copy_after(s->pkg, sizeof s->pkg, line, "pkg:");
		s->pending[0] = 0;
		return;
	}
	if (strncmp(line, "goos:", 5) == 0) {
		copy_after(s->goos, sizeof s->goos, line, "goos:");
		return;
	}
	if (strncmp(line, "goarch:", 7) == 0) {
		copy_after(s->goarch, sizeof s->goarch, line, "goarch:");
		return;
	}
	if (strncmp(line, "cpu:", 4) == 0) {
		copy_after(s->cpu, sizeof s->cpu, line, "cpu:");
		return;
	}
	if (strncmp(line, "ok ", 3) == 0 || strncmp(line, "ok\t", 3) == 0) {
		s->nok++;
		return;
	}
	if (strncmp(line, "FAIL\t", 5) == 0 || strncmp(line, "FAIL ", 5) == 0) {
		char pkg[256];
		sscanf(line + 5, "%255s", pkg);
		size_t l = strlen(s->failed);
		if (l + strlen(pkg) + 2 < sizeof s->failed)
			snprintf(s->failed + l, sizeof s->failed - l, "%s%s", l ? "," : "", pkg);
		s->nfailed++;
		return;
	}
	char *tok[64];
	size_t nt = 0;
	for (char *t = strtok(line, " \t"); t && nt < 64; t = strtok(NULL, " \t"))
		tok[nt++] = t;
	/* go test prints the name, runs the benchmark, then prints the result,
	 * so log lines from the code under test can land in between. The name
	 * stays pending until a result line (iterations, value, unit) shows up. */
	size_t k = 0;
	if (nt && strncmp(tok[0], "Benchmark", 9) == 0) {
		snprintf(s->pending, sizeof s->pending, "%s", tok[0]);
		strip_procs(s->pending);
		k = 1;
	}
	if (!s->pending[0] || !is_result(tok + k, nt - k))
		return;
	bench_t *b = bench_get(s, s->pkg, s->pending);
	s->pending[0] = 0;
	if (!b)
		return;
	b->samples++;
	for (size_t i = k + 1; i + 1 < nt; i += 2) {
		char *end = NULL;
		double v = strtod(tok[i], &end);
		if (end && *end == 0)
			bench_add(b, tok[i + 1], v);
	}
}

static const char *unit_key(const char *unit)
{
	if (strcmp(unit, "ns/op") == 0)
		return "ns_per_op";
	if (strcmp(unit, "B/op") == 0)
		return "bytes_per_op";
	if (strcmp(unit, "allocs/op") == 0)
		return "allocs_per_op";
	return NULL;
}

int bench_report(ctx_t *ctx, jw_t *w, const char *text, size_t len, const char *source)
{
	bench_set_t s = {0};
	char *copy = malloc(len + 1);
	if (!copy)
		return fail(ctx, EXIT_RUNTIME, "oom", "out of memory");
	memcpy(copy, text, len);
	copy[len] = 0;
	for (char *line = copy; line && *line;) {
		char *nl = strchr(line, '\n');
		if (nl)
			*nl = 0;
		size_t l = strlen(line);
		if (l && line[l - 1] == '\r')
			line[l - 1] = 0;
		parse_line(&s, line);
		line = nl ? nl + 1 : NULL;
	}
	free(copy);

	jw_obj(w, NULL);
	jw_str(w, "source", source);
	if (s.goos[0])
		jw_str(w, "goos", s.goos);
	if (s.goarch[0])
		jw_str(w, "goarch", s.goarch);
	if (s.cpu[0])
		jw_str(w, "cpu", s.cpu);
	jw_int(w, "packages_ok", (int64_t)s.nok);
	jw_int(w, "packages_failed", (int64_t)s.nfailed);
	if (s.nfailed)
		jw_str(w, "failed_packages", s.failed);
	jw_str(w, "stat_note", "p50 is the median over -count samples; compare gates on it like benchstat");
	jw_arr(w, "benchmarks");
	size_t noisy = 0, thin = 0;
	for (size_t i = 0; i < s.n; i++) {
		bench_t *b = &s.items[i];
		jw_obj(w, NULL);
		jw_str(w, "pkg", b->pkg);
		jw_str(w, "name", b->name);
		jw_int(w, "samples", (int64_t)b->samples);
		int custom = 0;
		for (size_t u = 0; u < b->nunits; u++) {
			const char *key = unit_key(b->units[u].unit);
			if (!key) {
				custom = 1;
				continue;
			}
			stats_t st;
			stats_compute(b->units[u].v.v, b->units[u].v.n, &st);
			jw_stats(w, key, &st);
			if (strcmp(key, "ns_per_op") == 0 && st.n >= 3 && st.mean > 0) {
				double cv = 100.0 * st.stddev / st.mean;
				jw_num(w, "ns_per_op_cv_percent", cv);
				if (cv > 5)
					noisy++;
			}
			if (strcmp(key, "ns_per_op") == 0 && st.n < 3)
				thin++;
		}
		/* Custom b.ReportMetric units keep their name; direction unknown. */
		if (custom) {
			jw_arr(w, "metrics");
			for (size_t u = 0; u < b->nunits; u++) {
				if (unit_key(b->units[u].unit))
					continue;
				stats_t st;
				stats_compute(b->units[u].v.v, b->units[u].v.n, &st);
				jw_obj(w, NULL);
				jw_str(w, "unit", b->units[u].unit);
				jw_stats(w, "value", &st);
				jw_end_obj(w);
			}
			jw_end_arr(w);
		}
		jw_end_obj(w);
	}
	jw_end_arr(w);
	jw_end_obj(w);

	if (s.nfailed) {
		ctx->threshold_hit = 1;
		finding(ctx, "error", "bench_failed", "%zu packages failed: %s", s.nfailed, s.failed);
		finding_hint(ctx, "rerun with --show-output or go test on the package to see the failure");
	}
	if (s.n == 0 && !s.nfailed) {
		finding(ctx, "warn", "bench_none", "no benchmark lines parsed; check --bench and --pkg");
		finding_hint(ctx, "grep -rn '^func Benchmark' --include=*_test.go internal");
	}
	if (noisy) {
		finding(ctx, "warn", "bench_noisy", "%zu benchmarks vary more than 5%% between samples", noisy);
		finding_hint(ctx, "raise --count, close Studio/CS2 and rerun; do not trust deltas smaller than the spread");
	}
	if (thin) {
		finding(ctx, "info", "bench_few_samples", "%zu benchmarks have fewer than 3 samples", thin);
		finding_hint(ctx, "use --count 5 or more before comparing");
	}
	for (size_t i = 0; i < s.n; i++)
		for (size_t u = 0; u < s.items[i].nunits; u++)
			dvec_free(&s.items[i].units[u].v);
	free(s.items);
	return 0;
}

int cmd_bench(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *from = NULL, *bench = ".", *benchtime = NULL, *v;
	const char *pkgs[32];
	size_t npkgs = 0;
	long count = 5, timeout = 1800;
	int show_output = 0;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--from", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--from needs a file with go test -bench output");
			from = v;
		} else if (arg_val(argc, argv, &i, "--pkg", &v)) {
			if (!v || npkgs == 32)
				return fail(ctx, EXIT_USAGE, "usage", "--pkg needs a package pattern like ./internal/editor");
			pkgs[npkgs++] = v;
		} else if (arg_val(argc, argv, &i, "--bench", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--bench needs a regexp");
			bench = v;
		} else if (arg_val(argc, argv, &i, "--benchtime", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--benchtime needs a value like 2s or 100x");
			benchtime = v;
		} else if (arg_val(argc, argv, &i, "--count", &v)) {
			if (parse_int_arg(ctx, "--count", v, 1, 100, &count))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--timeout-s", &v)) {
			if (parse_int_arg(ctx, "--timeout-s", v, 1, 86400, &timeout))
				return EXIT_USAGE;
		} else if (arg_flag(argv, i, "--show-output")) {
			show_output = 1;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "bench: unknown argument %s", argv[i]);
		}
	}
	if (from) {
		size_t len;
		char *text = read_file(from, &len);
		if (!text)
			return fail(ctx, EXIT_RUNTIME, "read_failed", "cannot read %s", from);
		int rc = bench_report(ctx, w, text, len, from);
		free(text);
		return rc;
	}
	if (npkgs == 0)
		return fail(ctx, EXIT_USAGE, "usage",
			    "bench needs --pkg ./internal/<pkg> (repeatable) or --from FILE; running every package takes "
			    "minutes");

	char countstr[16], benchtimearg[80];
	snprintf(countstr, sizeof countstr, "%ld", count);
	char *args[64];
	int n = 0;
	args[n++] = "go";
	args[n++] = "test";
	args[n++] = "-run";
	args[n++] = "^$";
	args[n++] = "-bench";
	args[n++] = (char *)bench;
	args[n++] = "-benchmem";
	args[n++] = "-count";
	args[n++] = countstr;
	if (benchtime) {
		snprintf(benchtimearg, sizeof benchtimearg, "-benchtime=%s", benchtime);
		args[n++] = benchtimearg;
	}
	for (size_t i = 0; i < npkgs; i++)
		args[n++] = (char *)pkgs[i];
	args[n] = NULL;

	run_result_t r;
	if (plat_run(args, timeout, 1, &r) != 0)
		return fail(ctx, EXIT_RUNTIME, "spawn_failed", "%s", r.err);
	char source[512] = "";
	for (int i = 0; i < n; i++) {
		size_t l = strlen(source);
		snprintf(source + l, sizeof source - l, "%s%s", i ? " " : "", args[i]);
	}
	/* Splice run facts next to the parsed benchmarks. */
	sb_t sb = {0};
	jw_t bw;
	jw_init(&bw, &sb, 0);
	int rc = bench_report(ctx, &bw, r.output_full ? r.output_full : "", r.output_full ? strlen(r.output_full) : 0,
			      source);
	if (rc == 0) {
		jw_obj(w, NULL);
		jw_num(w, "run_wall_ms", r.wall_ms);
		jw_int(w, "exit_code", r.exit_code);
		if (r.timed_out)
			jw_bool(w, "timed_out", 1);
		if (show_output || r.exit_code != 0)
			jw_str(w, "output_tail", r.output_tail);
		if (sb.len > 2) {
			sb_puts(w->sb, ",");
			sb_add(w->sb, sb.p + 1, sb.len - 2);
		}
		jw_end_obj(w);
		if (r.exit_code != 0 || r.timed_out) {
			ctx->threshold_hit = 1;
			finding(ctx, "error", "bench_exit", "go test exited %d%s", r.exit_code, r.timed_out ? " (timed out)" : "");
		}
	}
	sb_free(&sb);
	free(r.output_full);
	return rc;
}
