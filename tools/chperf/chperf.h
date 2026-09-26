/* chperf: ClipHub performance probe for AI agents.
 *
 * Every command prints exactly one JSON document on stdout (the envelope
 * written by main.c) so an agent can parse it without scraping text. Units
 * live in key suffixes: _ms, _bytes, _percent, _per_s, _count. */
#ifndef CHPERF_H
#define CHPERF_H

#include <stddef.h>
#include <stdint.h>
#include <stdio.h>

#define CHPERF_VERSION "1.0.0"
#define CHPERF_SCHEMA 1

/* Exit codes. */
enum {
	EXIT_OK = 0,
	EXIT_THRESHOLD = 1, /* measured fine, but a regression or failed child */
	EXIT_USAGE = 2,
	EXIT_RUNTIME = 3
};

/* ---- string builder ---- */
typedef struct {
	char *p;
	size_t len, cap;
} sb_t;

void sb_add(sb_t *sb, const char *s, size_t n);
void sb_puts(sb_t *sb, const char *s);
void sb_printf(sb_t *sb, const char *fmt, ...);
void sb_free(sb_t *sb);

/* ---- JSON writer ---- */
#define JW_MAX_DEPTH 64
typedef struct {
	sb_t *sb;
	int depth;
	unsigned char need_comma[JW_MAX_DEPTH];
	int pretty;
} jw_t;

void jw_init(jw_t *w, sb_t *sb, int pretty);
void jw_obj(jw_t *w, const char *key);
void jw_arr(jw_t *w, const char *key);
void jw_end_obj(jw_t *w);
void jw_end_arr(jw_t *w);
void jw_str(jw_t *w, const char *key, const char *val);
void jw_strn(jw_t *w, const char *key, const char *val, size_t n);
void jw_num(jw_t *w, const char *key, double val);
void jw_int(jw_t *w, const char *key, int64_t val);
void jw_bool(jw_t *w, const char *key, int val);
void jw_null(jw_t *w, const char *key);
/* Writes already-serialized JSON as the value of key. */
void jw_raw(jw_t *w, const char *key, const char *raw, size_t n);

/* ---- JSON parser ---- */
enum { J_NULL, J_BOOL, J_NUM, J_STR, J_ARR, J_OBJ };

typedef struct jval jval;
struct jval {
	int type;
	int boolean;
	double num;
	char *str; /* J_STR */
	size_t n;  /* items in J_ARR/J_OBJ */
	jval *items;
	char **keys; /* J_OBJ only, parallel to items */
};

jval *json_parse(const char *text, size_t len, char *err, size_t errlen);
void json_free(jval *v);
const jval *jget(const jval *obj, const char *key);
const jval *jat(const jval *arr, size_t i);
double jnum(const jval *v, double def);
const char *jstr(const jval *v, const char *def);
int jbool(const jval *v, int def);
/* jpath("a.b.c") walks object keys only. */
const jval *jpath(const jval *v, const char *dotted);
/* Serializes a parsed value (used by --pretty). */
void jw_value(jw_t *w, const char *key, const jval *v);

/* ---- stats ---- */
typedef struct {
	size_t n;
	double min, max, mean, stddev, p50, p90, p95, p99;
} stats_t;

void stats_compute(const double *v, size_t n, stats_t *out);
double percentile_sorted(const double *sorted, size_t n, double pct);
void jw_stats(jw_t *w, const char *key, const stats_t *s);

typedef struct {
	double *v;
	size_t n, cap;
} dvec_t;
void dvec_push(dvec_t *d, double x);
void dvec_free(dvec_t *d);

/* ---- files ---- */
char *read_file(const char *path, size_t *len);
int write_file(const char *path, const char *data, size_t len);
void path_join(char *out, size_t cap, const char *a, const char *b);

/* ---- findings ---- */
typedef struct {
	char level[8]; /* info | warn | error */
	char code[48];
	char message[512];
	char hint[256]; /* the chperf command (or check) that drills into it */
} finding_t;

typedef struct {
	int pretty;
	char data_dir[1024];
	char data_dir_source[32];
	finding_t *findings;
	size_t nfindings, capfindings;
	char err_code[32];
	char err_msg[512];
	int threshold_hit;
} ctx_t;

void finding(ctx_t *ctx, const char *level, const char *code, const char *fmt, ...);
/* Attaches a next step to the finding added last. */
void finding_hint(ctx_t *ctx, const char *hint);
int fail(ctx_t *ctx, int exit_code, const char *code, const char *fmt, ...);

/* ---- arg helpers ---- */
/* Returns 1 and sets *val if argv[*i] is --name value or --name=value. */
int arg_val(int argc, char **argv, int *i, const char *name, const char **val);
int arg_flag(char **argv, int i, const char *name);
int parse_int_arg(ctx_t *ctx, const char *name, const char *s, long lo, long hi, long *out);

/* ---- data dir ---- */
int resolve_data_dir(ctx_t *ctx, const char *flag);
int orchestrator_port(ctx_t *ctx);

/* ---- commands ---- */
int cmd_system(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_pipeline(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_renders(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_procs(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_run(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_http(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_compare(ctx_t *ctx, int argc, char **argv, jw_t *w);

/* Shared cores so snapshot can reuse them with defaults. */
int pipeline_report(ctx_t *ctx, jw_t *w, const char *since, long last);
int renders_report(ctx_t *ctx, jw_t *w, const char *job, long limit, int full);
int procs_report(ctx_t *ctx, jw_t *w, long seconds, long interval_ms, long root_pid, const char **extra, size_t nextra);
int http_report(ctx_t *ctx, jw_t *w, const char *base, const char **paths, size_t npaths, long n, long warmup,
		const char *token_env);
int system_report(ctx_t *ctx, jw_t *w);
int captures_report(ctx_t *ctx, jw_t *w, const char *job, long limit);
int bench_report(ctx_t *ctx, jw_t *w, const char *text, size_t len, const char *source);
int cmd_captures(ctx_t *ctx, int argc, char **argv, jw_t *w);
int cmd_bench(ctx_t *ctx, int argc, char **argv, jw_t *w);
/* Newest render-result.json of job with performance: summed render_ms and
 * media seconds of its non-reused outputs. -1 when none. */
int latest_render_of_job(ctx_t *ctx, const char *job, double *render_ms, double *media_s, char *variant, size_t vcap);

/* compare core, exposed for tests. */
typedef struct {
	char path[512];
	double base, cand, delta, delta_percent;
	int verdict; /* -1 improve, 0 same, 1 regress */
} change_t;

typedef struct {
	change_t *items;
	size_t n, cap;
	size_t compared;
} changes_t;

void compare_values(const jval *base, const jval *cand, const char *path, int cost, double threshold_pct,
		    double min_abs, changes_t *out);
void changes_free(changes_t *c);
int key_is_cost(const char *key);

/* ---- platform ---- */
int64_t plat_now_ns(void);
void plat_utc_iso(char out[40]);
void plat_unix_to_iso(int64_t unix_s, char out[40]);
void plat_sleep_ms(long ms);

typedef struct {
	char cpu_name[128];
	int logical_cpus;
	uint64_t ram_total_bytes, ram_avail_bytes;
	char os[160];
	char arch[16];
} sysinfo_t;
int plat_sysinfo(sysinfo_t *si);
int plat_disk_space(const char *path, uint64_t *free_b, uint64_t *total_b);

typedef int (*dir_cb)(const char *name, int is_dir, void *ctx);
int plat_list_dir(const char *path, dir_cb cb, void *ctx);
int plat_file_mtime(const char *path, int64_t *unix_s);
int plat_is_dir(const char *path);
const char *plat_getenv(const char *name);

/* Process sampling. cpu_ns is cumulative user+kernel CPU time. */
typedef struct {
	uint32_t pid, ppid;
	char name[128];
	uint32_t threads;
} proc_entry_t;

typedef struct {
	uint64_t cpu_ns;
	uint64_t ws_bytes, private_bytes;
	uint64_t io_read_bytes, io_write_bytes;
	uint32_t handles;
	int ok;
} proc_detail_t;

int plat_proc_list(proc_entry_t **out, size_t *n);
int plat_proc_detail(uint32_t pid, proc_detail_t *d);
/* Cumulative system CPU counters in arbitrary equal units. */
int plat_system_cpu(uint64_t *busy, uint64_t *total);

typedef struct {
	int exit_code;
	int timed_out;
	int started;
	double wall_ms, user_ms, sys_ms;
	uint64_t peak_memory_bytes;
	const char *peak_memory_kind;
	uint64_t io_read_bytes, io_write_bytes;
	uint32_t total_processes;
	uint32_t stragglers;
	uint64_t output_bytes;
	char output_tail[1536];
	char *output_full; /* malloc'd whole output when requested; caller frees */
	char err[256];
} run_result_t;

/* keep_full: also return the child's entire stdout+stderr in output_full. */
int plat_run(char **argv, long timeout_s, int keep_full, run_result_t *r);

typedef intptr_t sock_t;
int plat_net_init(void);
int plat_tcp_connect(const char *host, int port, long timeout_ms, sock_t *s, char *err, size_t errlen);
int plat_send_all(sock_t s, const char *buf, size_t n);
long plat_recv(sock_t s, char *buf, size_t n);
void plat_sock_close(sock_t s);

#endif
