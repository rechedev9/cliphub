/* Live measurements: host, ClipHub processes, a command under test, and the
 * local orchestrator HTTP API. */
#include "chperf.h"

#include <ctype.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

/* ---- system ---- */

int system_report(ctx_t *ctx, jw_t *w)
{
	sysinfo_t si;
	plat_sysinfo(&si);
	jw_obj(w, NULL);
	jw_str(w, "os", si.os);
	jw_str(w, "arch", si.arch);
	jw_str(w, "cpu", si.cpu_name);
	jw_int(w, "logical_cpus", si.logical_cpus);
	jw_num(w, "ram_total_bytes", (double)si.ram_total_bytes);
	jw_num(w, "ram_available_bytes", (double)si.ram_avail_bytes);
	jw_str(w, "data_dir", ctx->data_dir);
	jw_str(w, "data_dir_source", ctx->data_dir_source);
	uint64_t fb = 0, tb = 0;
	if (plat_disk_space(ctx->data_dir, &fb, &tb) == 0) {
		jw_num(w, "data_disk_free_bytes", (double)fb);
		jw_num(w, "data_disk_total_bytes", (double)tb);
		/* A 1080p60 Full Demo capture plus render needs tens of GB. */
		if (fb < 30ULL * 1024 * 1024 * 1024)
			finding(ctx, "warn", "low_disk", "only %.1f GB free on the data disk; long Full Demo captures need tens of GB",
				(double)fb / 1073741824.0);
	}
	int port = orchestrator_port(ctx);
	if (port > 0)
		jw_int(w, "orchestrator_port", port);
	else
		jw_null(w, "orchestrator_port");
	if (si.ram_total_bytes && (double)si.ram_avail_bytes / (double)si.ram_total_bytes < 0.1)
		finding(ctx, "warn", "low_memory", "less than 10%% of RAM is available (%.1f GB)",
			(double)si.ram_avail_bytes / 1073741824.0);
	jw_end_obj(w);
	return 0;
}

int cmd_system(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	if (argc > 0)
		return fail(ctx, EXIT_USAGE, "usage", "system: unknown argument %s", argv[0]);
	return system_report(ctx, w);
}

/* ---- procs ---- */

/* Image names (lowercase, no .exe) that belong to ClipHub's runtime. */
static const struct {
	const char *name;
	const char *role;
	int prefix;
} known_roles[] = {
	{"cliphub studio", "studio", 0},
	{"zv-orchestrator", "orchestrator", 0},
	{"zv-", "zv-tool", 1},
	{"zv", "zv-tool", 0},
	{"ffmpeg", "ffmpeg", 0},
	{"ffprobe", "ffmpeg", 0},
	{"cs2", "cs2", 0},
	{"hlae", "hlae", 0},
};

static void norm_name(const char *in, char *out, size_t cap)
{
	size_t n = 0;
	for (; in[n] && n + 1 < cap; n++)
		out[n] = (char)tolower((unsigned char)in[n]);
	out[n] = 0;
	if (n > 4 && strcmp(out + n - 4, ".exe") == 0)
		out[n - 4] = 0;
}

static const char *role_for(const char *name, const char **extra, size_t nextra)
{
	char n[128];
	norm_name(name, n, sizeof n);
	for (size_t i = 0; i < sizeof known_roles / sizeof known_roles[0]; i++) {
		size_t l = strlen(known_roles[i].name);
		if (known_roles[i].prefix ? strncmp(n, known_roles[i].name, l) == 0 : strcmp(n, known_roles[i].name) == 0)
			return known_roles[i].role;
	}
	for (size_t i = 0; i < nextra; i++) {
		char e[128];
		norm_name(extra[i], e, sizeof e);
		if (strcmp(n, e) == 0)
			return extra[i];
	}
	return NULL;
}

typedef struct {
	uint32_t pid;
	char name[128];
	char role[48];
	uint64_t prev_cpu_ns, prev_r, prev_w;
	int has_prev;
	double cpu_sum;
	int cpu_samples;
	uint64_t ws_last, ws_peak, priv_last;
	uint32_t threads;
	int seen; /* alive in this sample */
} tracked_t;

typedef struct {
	char role[48];
	dvec_t cpu, ws, priv, rd, wr, count;
	uint64_t handles_last, threads_last;
} role_series_t;

#define MAX_TRACKED 512
#define MAX_ROLES 32

typedef struct {
	tracked_t t[MAX_TRACKED];
	size_t nt;
	role_series_t r[MAX_ROLES];
	size_t nr;
} procs_state_t;

static role_series_t *role_get(procs_state_t *st, const char *role)
{
	for (size_t i = 0; i < st->nr; i++)
		if (strcmp(st->r[i].role, role) == 0)
			return &st->r[i];
	if (st->nr == MAX_ROLES)
		return NULL;
	role_series_t *r = &st->r[st->nr++];
	memset(r, 0, sizeof *r);
	snprintf(r->role, sizeof r->role, "%s", role);
	return r;
}

/* Selects processes matched by name, the tree under root_pid, and every
 * descendant of a matched process (Studio's renderers, a worker's ffmpeg). */
static void select_procs(procs_state_t *st, const proc_entry_t *list, size_t n, long root_pid, const char **extra,
			 size_t nextra)
{
	char (*roles)[48] = calloc(n, sizeof *roles);
	if (!roles)
		return;
	for (size_t i = 0; i < n; i++) {
		const char *r = role_for(list[i].name, extra, nextra);
		if (root_pid > 0 && list[i].pid == (uint32_t)root_pid)
			r = "target";
		if (r)
			snprintf(roles[i], sizeof roles[i], "%s", r);
	}
	for (int changed = 1, guard = 0; changed && guard < 64; guard++) {
		changed = 0;
		for (size_t i = 0; i < n; i++) {
			if (roles[i][0] || list[i].ppid == 0)
				continue;
			for (size_t j = 0; j < n; j++) {
				if (list[j].pid == list[i].ppid && roles[j][0] && list[j].pid != list[i].pid) {
					/* Keep the parent role so an ffmpeg under a worker still
					 * counts for the pipeline that spawned it. */
					snprintf(roles[i], sizeof roles[i], "%s", roles[j]);
					changed = 1;
					break;
				}
			}
		}
	}
	for (size_t i = 0; i < n; i++) {
		if (!roles[i][0])
			continue;
		tracked_t *t = NULL;
		for (size_t k = 0; k < st->nt; k++)
			if (st->t[k].pid == list[i].pid && strcmp(st->t[k].name, list[i].name) == 0)
				t = &st->t[k];
		if (!t) {
			if (st->nt == MAX_TRACKED)
				continue;
			t = &st->t[st->nt++];
			memset(t, 0, sizeof *t);
			t->pid = list[i].pid;
			snprintf(t->name, sizeof t->name, "%s", list[i].name);
			/* A name match beats an inherited role (ffmpeg under Studio is ffmpeg). */
			const char *own = role_for(list[i].name, extra, nextra);
			snprintf(t->role, sizeof t->role, "%s", own && strcmp(roles[i], "target") != 0 ? own : roles[i]);
		}
		t->threads = list[i].threads;
		t->seen = 1;
	}
	free(roles);
}

static int cmp_tracked_cpu(const void *a, const void *b)
{
	const tracked_t *x = a, *y = b;
	double cx = x->cpu_samples ? x->cpu_sum / x->cpu_samples : 0, cy = y->cpu_samples ? y->cpu_sum / y->cpu_samples : 0;
	return (cx < cy) - (cx > cy);
}

int procs_report(ctx_t *ctx, jw_t *w, long seconds, long interval_ms, long root_pid, const char **extra, size_t nextra)
{
	procs_state_t *st = calloc(1, sizeof *st);
	if (!st)
		return fail(ctx, EXIT_RUNTIME, "oom", "out of memory");
	sysinfo_t si;
	plat_sysinfo(&si);
	int ncpu = si.logical_cpus > 0 ? si.logical_cpus : 1;
	long samples = seconds * 1000 / interval_ms;
	if (samples < 1)
		samples = 1;
	dvec_t sys_cpu = {0};
	uint64_t sb0 = 0, stt0 = 0;
	plat_system_cpu(&sb0, &stt0);
	int64_t prev_t = plat_now_ns(), start = prev_t;

	for (long s = 0; s <= samples; s++) {
		if (s > 0) {
			int64_t target = start + (int64_t)s * interval_ms * 1000000LL;
			long wait = (long)((target - plat_now_ns()) / 1000000LL);
			if (wait > 0)
				plat_sleep_ms(wait);
		}
		proc_entry_t *list = NULL;
		size_t n = 0;
		if (plat_proc_list(&list, &n) != 0) {
			free(st);
			return fail(ctx, EXIT_RUNTIME, "proc_list_failed", "cannot enumerate processes");
		}
		for (size_t k = 0; k < st->nt; k++)
			st->t[k].seen = 0;
		select_procs(st, list, n, root_pid, extra, nextra);
		free(list);
		int64_t now = plat_now_ns();
		double dt = (double)(now - prev_t) / 1e9;
		prev_t = now;
		uint64_t sb1 = 0, stt1 = 0;
		if (s > 0 && plat_system_cpu(&sb1, &stt1) == 0 && stt1 > stt0) {
			dvec_push(&sys_cpu, 100.0 * (double)(sb1 - sb0) / (double)(stt1 - stt0));
			sb0 = sb1;
			stt0 = stt1;
		}
		/* Per-role sums for this sample. */
		double rcpu[MAX_ROLES] = {0}, rws[MAX_ROLES] = {0}, rpriv[MAX_ROLES] = {0}, rrd[MAX_ROLES] = {0},
		       rwr[MAX_ROLES] = {0}, rcount[MAX_ROLES] = {0};
		uint64_t rh[MAX_ROLES] = {0}, rth[MAX_ROLES] = {0};
		for (size_t k = 0; k < st->nt; k++) {
			tracked_t *t = &st->t[k];
			if (!t->seen)
				continue;
			proc_detail_t d;
			if (plat_proc_detail(t->pid, &d) != 0 || !d.ok)
				continue;
			role_series_t *r = role_get(st, t->role);
			size_t ri = r ? (size_t)(r - st->r) : 0;
			if (t->has_prev && s > 0 && dt > 0 && r) {
				double cpu = d.cpu_ns >= t->prev_cpu_ns
						     ? 100.0 * (double)(d.cpu_ns - t->prev_cpu_ns) / 1e9 / dt / ncpu
						     : 0;
				rcpu[ri] += cpu;
				rrd[ri] += d.io_read_bytes >= t->prev_r ? (double)(d.io_read_bytes - t->prev_r) / dt : 0;
				rwr[ri] += d.io_write_bytes >= t->prev_w ? (double)(d.io_write_bytes - t->prev_w) / dt : 0;
				t->cpu_sum += cpu;
				t->cpu_samples++;
			}
			t->prev_cpu_ns = d.cpu_ns;
			t->prev_r = d.io_read_bytes;
			t->prev_w = d.io_write_bytes;
			t->has_prev = 1;
			t->ws_last = d.ws_bytes;
			t->priv_last = d.private_bytes;
			if (d.ws_bytes > t->ws_peak)
				t->ws_peak = d.ws_bytes;
			if (r) {
				rws[ri] += (double)d.ws_bytes;
				rpriv[ri] += (double)d.private_bytes;
				rcount[ri] += 1;
				rh[ri] += d.handles;
				rth[ri] += t->threads;
			}
		}
		if (s == 0)
			continue; /* baseline sample: no deltas yet */
		for (size_t ri = 0; ri < st->nr; ri++) {
			role_series_t *r = &st->r[ri];
			/* A role first seen mid-window was idle (absent) before it. */
			while (r->cpu.n < (size_t)(s - 1)) {
				dvec_push(&r->cpu, 0), dvec_push(&r->ws, 0), dvec_push(&r->priv, 0);
				dvec_push(&r->rd, 0), dvec_push(&r->wr, 0), dvec_push(&r->count, 0);
			}
			dvec_push(&r->cpu, rcpu[ri]);
			dvec_push(&r->ws, rws[ri]);
			dvec_push(&r->priv, rpriv[ri]);
			dvec_push(&r->rd, rrd[ri]);
			dvec_push(&r->wr, rwr[ri]);
			dvec_push(&r->count, rcount[ri]);
			r->handles_last = rh[ri];
			r->threads_last = rth[ri];
		}
	}

	jw_obj(w, NULL);
	jw_int(w, "interval_ms", interval_ms);
	jw_int(w, "samples", samples);
	jw_int(w, "logical_cpus", ncpu);
	jw_str(w, "cpu_percent_basis", "whole machine (100 = every logical CPU busy)");
	stats_t s;
	stats_compute(sys_cpu.v, sys_cpu.n, &s);
	/* Whole machine, other apps included: context for the ClipHub numbers,
	 * not a ClipHub cost, so compare ignores it (machine_ prefix). */
	jw_stats(w, "machine_cpu_percent", &s);

	/* Totals across roles, per sample. */
	dvec_t tcpu = {0}, tws = {0}, tpriv = {0};
	for (long k = 0; k < samples; k++) {
		double c = 0, ws = 0, pv = 0;
		for (size_t ri = 0; ri < st->nr; ri++) {
			if ((size_t)k < st->r[ri].cpu.n) {
				c += st->r[ri].cpu.v[k];
				ws += st->r[ri].ws.v[k];
				pv += st->r[ri].priv.v[k];
			}
		}
		dvec_push(&tcpu, c);
		dvec_push(&tws, ws);
		dvec_push(&tpriv, pv);
	}
	jw_obj(w, "cliphub_total");
	stats_compute(tcpu.v, tcpu.n, &s);
	jw_stats(w, "cpu_percent", &s);
	stats_compute(tws.v, tws.n, &s);
	jw_num(w, "working_set_peak_bytes", s.n ? s.max : 0);
	stats_compute(tpriv.v, tpriv.n, &s);
	jw_num(w, "private_peak_bytes", s.n ? s.max : 0);
	jw_int(w, "processes", (int64_t)st->nt);
	jw_end_obj(w);

	jw_arr(w, "roles");
	for (size_t ri = 0; ri < st->nr; ri++) {
		role_series_t *r = &st->r[ri];
		jw_obj(w, NULL);
		jw_str(w, "role", r->role);
		stats_compute(r->count.v, r->count.n, &s);
		jw_int(w, "processes_max", (int64_t)(s.n ? s.max : 0));
		stats_compute(r->cpu.v, r->cpu.n, &s);
		jw_stats(w, "cpu_percent", &s);
		if (s.n && s.p95 >= 50)
			finding(ctx, "info", "procs_busy_role", "%s used %.0f%% of the machine CPU at p95", r->role, s.p95);
		stats_compute(r->ws.v, r->ws.n, &s);
		jw_num(w, "working_set_peak_bytes", s.n ? s.max : 0);
		jw_num(w, "working_set_last_bytes", r->ws.n ? r->ws.v[r->ws.n - 1] : 0);
		stats_compute(r->priv.v, r->priv.n, &s);
		jw_num(w, "private_peak_bytes", s.n ? s.max : 0);
		stats_compute(r->rd.v, r->rd.n, &s);
		jw_num(w, "io_read_mean_bytes_per_s", s.n ? s.mean : 0);
		stats_compute(r->wr.v, r->wr.n, &s);
		jw_num(w, "io_write_mean_bytes_per_s", s.n ? s.mean : 0);
		jw_int(w, "handles_last", (int64_t)r->handles_last);
		jw_int(w, "threads_last", (int64_t)r->threads_last);
		jw_end_obj(w);
	}
	jw_end_arr(w);

	if (st->nt)
		qsort(st->t, st->nt, sizeof st->t[0], cmp_tracked_cpu);
	jw_arr(w, "top_processes");
	for (size_t k = 0; k < st->nt && k < 8; k++) {
		tracked_t *t = &st->t[k];
		jw_obj(w, NULL);
		jw_int(w, "pid", t->pid);
		jw_str(w, "name", t->name);
		jw_str(w, "role", t->role);
		jw_num(w, "cpu_mean_percent", t->cpu_samples ? t->cpu_sum / t->cpu_samples : 0);
		jw_num(w, "working_set_peak_bytes", (double)t->ws_peak);
		jw_num(w, "private_last_bytes", (double)t->priv_last);
		jw_end_obj(w);
	}
	jw_end_arr(w);
	jw_str(w, "gpu", "not sampled: Windows GPU counters take seconds to read and perturb short windows");
	jw_end_obj(w);

	if (st->nt == 0)
		finding(ctx, "warn", "procs_none", "no ClipHub process is running (Studio, zv-*, ffmpeg, cs2, hlae)");
	for (size_t ri = 0; ri < st->nr; ri++) {
		role_series_t *r = &st->r[ri];
		dvec_free(&r->cpu), dvec_free(&r->ws), dvec_free(&r->priv), dvec_free(&r->rd), dvec_free(&r->wr),
			dvec_free(&r->count);
	}
	dvec_free(&tcpu), dvec_free(&tws), dvec_free(&tpriv), dvec_free(&sys_cpu);
	free(st);
	return 0;
}

int cmd_procs(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	long seconds = 5, interval = 1000, pid = 0;
	const char *extra[16];
	size_t nextra = 0;
	const char *v;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--seconds", &v)) {
			if (parse_int_arg(ctx, "--seconds", v, 1, 3600, &seconds))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--interval-ms", &v)) {
			if (parse_int_arg(ctx, "--interval-ms", v, 100, 60000, &interval))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--pid", &v)) {
			if (parse_int_arg(ctx, "--pid", v, 1, 0x7fffffff, &pid))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--match", &v)) {
			if (!v || nextra == 16)
				return fail(ctx, EXIT_USAGE, "usage", "--match needs an image name (at most 16)");
			extra[nextra++] = v;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "procs: unknown argument %s", argv[i]);
		}
	}
	return procs_report(ctx, w, seconds, interval, pid, extra, nextra);
}

/* ---- run ---- */

int cmd_run(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	long repeat = 1, warmup = 0, timeout = 0;
	const char *label = NULL, *v;
	int show_output = 0, cmd_at = -1;
	for (int i = 0; i < argc; i++) {
		if (strcmp(argv[i], "--") == 0) {
			cmd_at = i + 1;
			break;
		}
		if (arg_val(argc, argv, &i, "--repeat", &v)) {
			if (parse_int_arg(ctx, "--repeat", v, 1, 1000, &repeat))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--warmup", &v)) {
			if (parse_int_arg(ctx, "--warmup", v, 0, 100, &warmup))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--timeout-s", &v)) {
			if (parse_int_arg(ctx, "--timeout-s", v, 1, 86400, &timeout))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--label", &v)) {
			label = v;
		} else if (arg_flag(argv, i, "--show-output")) {
			show_output = 1;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "run: unknown argument %s (put the command after --)", argv[i]);
		}
	}
	if (cmd_at < 0 || cmd_at >= argc)
		return fail(ctx, EXIT_USAGE, "usage", "run needs a command after --, e.g. chperf run --repeat 3 -- go version");
	char **cmd = &argv[cmd_at];

	run_result_t *res = calloc((size_t)repeat, sizeof *res);
	if (!res)
		return fail(ctx, EXIT_RUNTIME, "oom", "out of memory");
	for (long i = 0; i < warmup; i++) {
		run_result_t r;
		if (plat_run(cmd, timeout, 0, &r) != 0) {
			free(res);
			return fail(ctx, EXIT_RUNTIME, "spawn_failed", "%s", r.err);
		}
	}
	for (long i = 0; i < repeat; i++) {
		if (plat_run(cmd, timeout, 0, &res[i]) != 0) {
			char msg[256];
			snprintf(msg, sizeof msg, "%s", res[i].err);
			free(res);
			return fail(ctx, EXIT_RUNTIME, "spawn_failed", "%s", msg);
		}
	}

	dvec_t wall = {0}, cpu = {0}, util = {0};
	uint64_t peak = 0, rd = 0, wr = 0;
	long failed = 0;
	for (long i = 0; i < repeat; i++) {
		run_result_t *r = &res[i];
		dvec_push(&wall, r->wall_ms);
		dvec_push(&cpu, r->user_ms + r->sys_ms);
		if (r->wall_ms > 0)
			dvec_push(&util, 100.0 * (r->user_ms + r->sys_ms) / r->wall_ms);
		if (r->peak_memory_bytes > peak)
			peak = r->peak_memory_bytes;
		rd += r->io_read_bytes;
		wr += r->io_write_bytes;
		if (r->exit_code != 0 || r->timed_out)
			failed++;
	}
	jw_obj(w, NULL);
	if (label)
		jw_str(w, "label", label);
	jw_arr(w, "command");
	for (char **a = cmd; *a; a++)
		jw_str(w, NULL, *a);
	jw_end_arr(w);
	jw_int(w, "warmup", warmup);
	jw_int(w, "repeat", repeat);
	jw_int(w, "failed_runs", failed);
	stats_t s;
	stats_compute(wall.v, wall.n, &s);
	jw_stats(w, "wall_ms", &s);
	if (s.n > 1 && s.mean > 0) {
		double cv = 100.0 * s.stddev / s.mean;
		jw_num(w, "wall_cv_percent", cv);
		if (cv > 10)
			finding(ctx, "warn", "run_noisy",
				"wall time varies %.0f%% between runs; add --warmup or more --repeat before trusting a delta",
				cv);
	}
	stats_compute(cpu.v, cpu.n, &s);
	jw_stats(w, "cpu_ms", &s);
	stats_compute(util.v, util.n, &s);
	jw_num(w, "cpu_utilization_median_percent", s.p50);
	jw_str(w, "cpu_utilization_basis", "one logical CPU = 100");
	jw_num(w, "peak_memory_bytes", (double)peak);
	jw_str(w, "peak_memory_kind", res[0].peak_memory_kind);
	jw_num(w, "io_read_mean_bytes", (double)rd / (double)repeat);
	jw_num(w, "io_write_mean_bytes", (double)wr / (double)repeat);
	if (res[0].total_processes)
		jw_int(w, "processes_per_run", res[0].total_processes);
	jw_arr(w, "runs");
	for (long i = 0; i < repeat && i < 20; i++) {
		run_result_t *r = &res[i];
		jw_obj(w, NULL);
		jw_num(w, "wall_ms", r->wall_ms);
		jw_num(w, "cpu_ms", r->user_ms + r->sys_ms);
		jw_int(w, "exit_code", r->exit_code);
		if (r->timed_out)
			jw_bool(w, "timed_out", 1);
		if (r->stragglers)
			jw_int(w, "stragglers_killed", r->stragglers);
		jw_end_obj(w);
	}
	jw_end_arr(w);
	run_result_t *lastr = &res[repeat - 1];
	jw_num(w, "output_bytes", (double)lastr->output_bytes);
	if (show_output || failed)
		jw_str(w, "output_tail", lastr->output_tail);
	jw_end_obj(w);
	if (failed) {
		ctx->threshold_hit = 1;
		finding(ctx, "error", "run_failed", "%ld of %ld runs exited non-zero or timed out (last exit code %d)", failed,
			repeat, lastr->exit_code);
	}
	for (long i = 0; i < repeat; i++)
		if (res[i].stragglers) {
			finding(ctx, "warn", "run_stragglers",
				"child processes outlived the command and were killed; their CPU until then is counted");
			break;
		}
	dvec_free(&wall), dvec_free(&cpu), dvec_free(&util);
	free(res);
	return 0;
}

/* ---- http ---- */

typedef struct {
	char host[64];
	int port;
} url_t;

static int parse_loopback_url(ctx_t *ctx, const char *url, url_t *u)
{
	if (strncmp(url, "http://", 7) != 0)
		return fail(ctx, EXIT_USAGE, "usage", "--url must be http://<loopback>:<port>, got %s", url);
	const char *h = url + 7;
	const char *colon = h[0] == '[' ? strstr(h, "]:") : strchr(h, ':');
	if (!colon)
		return fail(ctx, EXIT_USAGE, "usage", "--url needs an explicit port");
	size_t hl = (size_t)(colon - h) + (h[0] == '[' ? 1 : 0);
	if (hl >= sizeof u->host)
		return fail(ctx, EXIT_USAGE, "usage", "host too long");
	memcpy(u->host, h, hl);
	u->host[hl] = 0;
	u->port = atoi(colon + (h[0] == '[' ? 2 : 1));
	/* Measuring someone else's server is out of scope, and the token must
	 * never leave the machine. */
	if (strcmp(u->host, "127.0.0.1") != 0 && strcmp(u->host, "localhost") != 0 && strcmp(u->host, "[::1]") != 0)
		return fail(ctx, EXIT_USAGE, "not_loopback", "chperf only measures loopback servers, not %s", u->host);
	if (u->host[0] == '[') {
		memmove(u->host, u->host + 1, strlen(u->host));
		u->host[strlen(u->host) - 1] = 0;
	}
	if (u->port <= 0 || u->port > 65535)
		return fail(ctx, EXIT_USAGE, "usage", "bad port in %s", url);
	return 0;
}

typedef struct {
	double connect_ms, ttfb_ms, total_ms;
	int status;
	long body_bytes;
	int ok;
} http_sample_t;

static void http_once(const url_t *u, const char *path, const char *token, http_sample_t *out)
{
	memset(out, 0, sizeof *out);
	int64_t t0 = plat_now_ns();
	sock_t s;
	char err[160];
	/* Loopback: a healthy server accepts in well under a millisecond. */
	if (plat_tcp_connect(u->host, u->port, 2000, &s, err, sizeof err) != 0)
		return;
	int64_t t1 = plat_now_ns();
	sb_t req = {0};
	sb_printf(&req, "GET %s HTTP/1.1\r\nHost: %s:%d\r\nUser-Agent: chperf/%s\r\nAccept: */*\r\nConnection: close\r\n",
		  path, u->host, u->port, CHPERF_VERSION);
	if (token)
		sb_printf(&req, "X-ClipHub-Token: %s\r\n", token);
	sb_puts(&req, "\r\n");
	int sent = plat_send_all(s, req.p, req.len);
	sb_free(&req);
	if (sent != 0) {
		plat_sock_close(s);
		return;
	}
	char buf[16384], head[512];
	size_t headlen = 0;
	long total = 0, header_end = -1;
	int64_t tf = 0;
	for (;;) {
		long k = plat_recv(s, buf, sizeof buf);
		if (k <= 0)
			break;
		if (!tf)
			tf = plat_now_ns();
		if (header_end < 0) {
			size_t take = (size_t)k < sizeof head - 1 - headlen ? (size_t)k : sizeof head - 1 - headlen;
			memcpy(head + headlen, buf, take);
			headlen += take;
			head[headlen] = 0;
			char *e = strstr(head, "\r\n\r\n");
			if (e)
				header_end = (long)(e - head) + 4;
		}
		total += k;
	}
	int64_t t2 = plat_now_ns();
	plat_sock_close(s);
	if (!tf)
		return;
	out->connect_ms = (double)(t1 - t0) / 1e6;
	out->ttfb_ms = (double)(tf - t0) / 1e6;
	out->total_ms = (double)(t2 - t0) / 1e6;
	if (strncmp(head, "HTTP/1.", 7) == 0)
		out->status = atoi(head + 9);
	out->body_bytes = header_end >= 0 ? total - header_end : 0;
	out->ok = out->status > 0;
}

int http_report(ctx_t *ctx, jw_t *w, const char *base, const char **paths, size_t npaths, long n, long warmup,
		const char *token_env)
{
	char defbase[64];
	if (!base) {
		int port = orchestrator_port(ctx);
		if (port <= 0)
			return fail(ctx, EXIT_RUNTIME, "no_orchestrator",
				    "no --url and no ports.json next to %s: is ClipHub Studio running?", ctx->data_dir);
		snprintf(defbase, sizeof defbase, "http://127.0.0.1:%d", port);
		base = defbase;
	}
	url_t u;
	if (parse_loopback_url(ctx, base, &u))
		return EXIT_USAGE;
	if (plat_net_init() != 0)
		return fail(ctx, EXIT_RUNTIME, "net_init", "socket layer init failed");
	const char *token = token_env ? plat_getenv(token_env) : NULL;
	static const char *public_paths[] = {"/healthz"};
	static const char *auth_paths[] = {"/healthz", "/api/capabilities", "/api/jobs", "/metrics"};
	if (npaths == 0) {
		paths = token ? auth_paths : public_paths;
		npaths = token ? 4 : 1;
	}
	jw_obj(w, NULL);
	jw_str(w, "base_url", base);
	jw_int(w, "requests_per_path", n);
	jw_str(w, "connection", "new TCP connection per request (Connection: close)");
	jw_bool(w, "authenticated", token != NULL);
	if (token_env)
		jw_str(w, "token_env", token_env);
	jw_arr(w, "paths");
	for (size_t p = 0; p < npaths; p++) {
		http_sample_t hs;
		for (long i = 0; i < warmup; i++) {
			http_once(&u, paths[p], token, &hs);
			if (!hs.ok)
				break; /* the measured loop reports the failure */
		}
		dvec_t conn = {0}, ttfb = {0}, tot = {0}, bytes = {0};
		int codes[8] = {0}, counts[8] = {0}, ncodes = 0;
		long errors = 0;
		long attempted = 0;
		for (long i = 0; i < n; i++) {
			attempted++;
			http_once(&u, paths[p], token, &hs);
			if (!hs.ok) {
				errors++;
				/* Nothing listening: report it now instead of waiting out
				 * every remaining connect timeout. */
				if (errors >= 2 && tot.n == 0)
					break;
				continue;
			}
			dvec_push(&conn, hs.connect_ms);
			dvec_push(&ttfb, hs.ttfb_ms);
			dvec_push(&tot, hs.total_ms);
			dvec_push(&bytes, (double)hs.body_bytes);
			int k;
			for (k = 0; k < ncodes && codes[k] != hs.status; k++)
				;
			if (k == ncodes && ncodes < 8)
				codes[ncodes++] = hs.status;
			if (k < 8)
				counts[k]++;
		}
		jw_obj(w, NULL);
		jw_str(w, "path", paths[p]);
		jw_obj(w, "status_count");
		for (int k = 0; k < ncodes; k++) {
			char key[8];
			snprintf(key, sizeof key, "%d", codes[k]);
			jw_int(w, key, counts[k]);
			if (codes[k] == 401)
				finding(ctx, "info", "http_needs_token",
					"%s answered 401: export the Studio session token in %s to measure protected routes",
					paths[p], token_env ? token_env : "ZV_MUTATION_TOKEN");
			else if (codes[k] >= 500)
				finding(ctx, "warn", "http_server_error", "%s answered %d", paths[p], codes[k]);
		}
		jw_end_obj(w);
		jw_int(w, "requests_attempted", attempted);
		jw_int(w, "transport_errors", errors);
		stats_t s;
		stats_compute(conn.v, conn.n, &s);
		jw_stats(w, "connect_ms", &s);
		stats_compute(ttfb.v, ttfb.n, &s);
		jw_stats(w, "ttfb_ms", &s);
		stats_compute(tot.v, tot.n, &s);
		jw_stats(w, "total_ms", &s);
		if (s.n && s.p50 > 250)
			finding(ctx, "warn", "http_slow", "%s median %.0f ms on loopback", paths[p], s.p50);
		stats_compute(bytes.v, bytes.n, &s);
		jw_num(w, "body_bytes_median", s.n ? s.p50 : 0);
		jw_end_obj(w);
		if (errors)
			finding(ctx, "warn", "http_transport_errors", "%s: %ld of %ld requests failed to connect or read",
				paths[p], errors, attempted);
		dvec_free(&conn), dvec_free(&ttfb), dvec_free(&tot), dvec_free(&bytes);
	}
	jw_end_arr(w);
	jw_end_obj(w);
	return 0;
}

int cmd_http(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *base = NULL, *token_env = "ZV_MUTATION_TOKEN", *v;
	const char *paths[32];
	size_t npaths = 0;
	long n = 30, warmup = 3;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--url", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--url needs a value");
			base = v;
		} else if (arg_val(argc, argv, &i, "--path", &v)) {
			if (!v || v[0] != '/' || npaths == 32)
				return fail(ctx, EXIT_USAGE, "usage", "--path needs an absolute path like /healthz (at most 32)");
			paths[npaths++] = v;
		} else if (arg_val(argc, argv, &i, "--n", &v)) {
			if (parse_int_arg(ctx, "--n", v, 1, 100000, &n))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--warmup", &v)) {
			if (parse_int_arg(ctx, "--warmup", v, 0, 1000, &warmup))
				return EXIT_USAGE;
		} else if (arg_val(argc, argv, &i, "--token-env", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--token-env needs an environment variable name");
			token_env = v;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "http: unknown argument %s", argv[i]);
		}
	}
	return http_report(ctx, w, base, paths, npaths, n, warmup, token_env);
}
