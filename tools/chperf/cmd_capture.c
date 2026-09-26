/* captures: what record:demo spends its time on, from the evidence the
 * recorder writes at <data>/jobs/<job>/recording/recording-result.json
 * (performance.runs[] phases and segments, artifacts[] sizes). */
#include "chperf.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

/* Below this many bits per pixel per frame a 1080p60 capture is very likely
 * black: the 5.0.0 black-capture incident averaged ~0.02 (2.4 Mb/s) against
 * ~0.3 (40 Mb/s) for a normal CRF18 capture. */
#define SUSPECT_BLACK_BPP 0.05

typedef struct {
	char job[80];
	char path[1400];
	int64_t mtime;
} capture_file_t;

typedef struct {
	capture_file_t *items;
	size_t n, cap;
	const char *root;
	const char *job_filter;
} capture_scan_t;

static int on_capture_job(const char *name, int is_dir, void *arg)
{
	capture_scan_t *s = arg;
	if (!is_dir || (s->job_filter && strcmp(name, s->job_filter) != 0))
		return 0;
	char dir[1300], path[1400];
	path_join(dir, sizeof dir, s->root, name);
	path_join(path, sizeof path, dir, "recording/recording-result.json");
	int64_t mt;
	if (plat_file_mtime(path, &mt) != 0)
		return 0;
	if (s->n == s->cap) {
		size_t cap = s->cap ? s->cap * 2 : 16;
		capture_file_t *p = realloc(s->items, cap * sizeof *p);
		if (!p)
			return 1;
		s->items = p;
		s->cap = cap;
	}
	capture_file_t *c = &s->items[s->n++];
	snprintf(c->job, sizeof c->job, "%s", name);
	snprintf(c->path, sizeof c->path, "%s", path);
	c->mtime = mt;
	return 0;
}

static int cmp_capture_mtime(const void *a, const void *b)
{
	const capture_file_t *x = a, *y = b;
	return (x->mtime < y->mtime) - (x->mtime > y->mtime);
}

static const char *phase_keys[] = {"prepare_ms",   "launch_and_capture_ms", "incremental_mux_ms", "artifact_probe_ms",
				   "final_mux_ms", "validation_ms",	    "before_result_write_ms"};
#define N_PHASES (sizeof phase_keys / sizeof phase_keys[0])

typedef struct {
	double capture_s_per_video_s;
	double video_s;
	double before_result_ms;
} capture_summary_t;

static void write_capture(ctx_t *ctx, jw_t *w, const capture_file_t *cf, const jval *doc, capture_summary_t *sum)
{
	char iso[40];
	plat_unix_to_iso(cf->mtime, iso);
	memset(sum, 0, sizeof *sum);
	sum->capture_s_per_video_s = NAN;

	const jval *runs = jpath(doc, "performance.runs");
	size_t nruns = runs && runs->type == J_ARR ? runs->n : 0;
	double phase[N_PHASES] = {0};
	int phase_seen[N_PHASES] = {0};
	dvec_t obs_fps = {0};
	double seg_video_s = 0;
	size_t nseg = 0;
	const jval *stream = NULL;
	for (size_t r = 0; r < nruns; r++) {
		const jval *run = &runs->items[r];
		if (!stream)
			stream = jget(run, "stream");
		for (size_t k = 0; k < N_PHASES; k++) {
			const jval *v = jget(run, phase_keys[k]);
			if (v && v->type == J_NUM) {
				phase[k] += v->num;
				phase_seen[k] = 1;
			}
		}
		const jval *segs = jget(run, "segments");
		for (size_t i = 0; segs && segs->type == J_ARR && i < segs->n; i++) {
			const jval *sg = &segs->items[i];
			seg_video_s += jnum(jget(sg, "video_duration_seconds"), 0);
			nseg++;
			double f = jnum(jget(sg, "observed_frames_per_second"), NAN);
			if (isfinite(f))
				dvec_push(&obs_fps, f);
		}
	}

	/* Raw takes are what HLAE wrote; their bitrate exposes a black capture. */
	const jval *arts = jget(doc, "artifacts");
	dvec_t mbps = {0}, bpp = {0};
	double raw_bytes = 0, raw_video_s = 0;
	size_t raw_takes = 0;
	char suspects[256] = "";
	size_t nsuspects = 0;
	for (size_t i = 0; arts && arts->type == J_ARR && i < arts->n; i++) {
		const jval *a = &arts->items[i];
		if (strcmp(jstr(jget(a, "type"), ""), "video") != 0 || strcmp(jstr(jget(a, "role"), ""), "raw") != 0)
			continue;
		double bytes = jnum(jget(a, "size_bytes"), NAN), dur = jnum(jget(a, "duration_seconds"), NAN);
		double wdt = jnum(jget(a, "width"), NAN), hgt = jnum(jget(a, "height"), NAN);
		double frames = jnum(jget(a, "frame_count"), NAN);
		if (!(bytes > 0 && dur > 0))
			continue;
		raw_takes++;
		raw_bytes += bytes;
		raw_video_s += dur;
		double rate = bytes * 8.0 / dur;
		dvec_push(&mbps, rate / 1e6);
		double fps = frames > 0 ? frames / dur : 60;
		if (wdt > 0 && hgt > 0) {
			double b = rate / (wdt * hgt * fps);
			dvec_push(&bpp, b);
			if (b < SUSPECT_BLACK_BPP) {
				if (nsuspects < 8) {
					size_t l = strlen(suspects);
					snprintf(suspects + l, sizeof suspects - l, "%s%s", l ? "," : "",
						 jstr(jget(a, "segment_id"), "?"));
				}
				nsuspects++;
			}
		}
	}

	double tick_rate = jnum(jpath(doc, "plan.full_demo.clock.tick_rate"), NAN);
	if (!(tick_rate > 0))
		tick_rate = jnum(jpath(doc, "plan.tick_rate"), 64);
	double ticks = jnum(jpath(doc, "plan.demo_duration_ticks"), NAN);
	double video_s = seg_video_s > 0 ? seg_video_s : raw_video_s;
	double lac = phase_seen[1] ? phase[1] : NAN;

	jw_obj(w, NULL);
	jw_str(w, "job_id", cf->job);
	jw_str(w, "written_at", iso);
	jw_str(w, "capture_mode", jstr(jget(doc, "capture_mode"), "?"));
	jw_bool(w, "full_demo", jpath(doc, "plan.full_demo") != NULL);
	if (jget(doc, "capture_verified"))
		jw_bool(w, "capture_verified", jbool(jget(doc, "capture_verified"), 0));
	if (stream) {
		jw_obj(w, "stream");
		jw_str(w, "encoder", jstr(jget(stream, "encoder"), NULL));
		jw_num(w, "fps", jnum(jget(stream, "fps"), NAN));
		jw_num(w, "width", jnum(jget(stream, "width"), NAN));
		jw_num(w, "height", jnum(jget(stream, "height"), NAN));
		jw_num(w, "crf", jnum(jget(stream, "crf"), NAN));
		jw_end_obj(w);
	}
	jw_int(w, "recorder_runs", (int64_t)nruns);
	jw_int(w, "segments", (int64_t)nseg);
	if (ticks > 0)
		jw_num(w, "demo_duration_s", ticks / tick_rate);
	jw_num(w, "captured_video_s", video_s);
	jw_obj(w, "phases_ms");
	for (size_t k = 0; k < N_PHASES; k++)
		if (phase_seen[k])
			jw_num(w, phase_keys[k], phase[k]);
	jw_end_obj(w);
	jw_str(w, "phases_note", "incremental_mux_ms overlaps launch_and_capture_ms; before_result_write_ms is the recorder total");
	if (isfinite(lac) && video_s > 0) {
		sum->capture_s_per_video_s = lac / 1000.0 / video_s;
		jw_num(w, "capture_s_per_video_s", sum->capture_s_per_video_s);
		jw_num(w, "realtime_speedup_ratio", video_s / (lac / 1000.0));
	}
	if (phase_seen[6] && isfinite(lac) && phase[6] > 0)
		jw_num(w, "post_capture_ms", phase[6] - lac - (phase_seen[0] ? phase[0] : 0));
	if (obs_fps.n) {
		stats_t s;
		stats_compute(obs_fps.v, obs_fps.n, &s);
		jw_stats(w, "observed_capture_fps", &s);
	}
	jw_obj(w, "raw_video");
	jw_int(w, "takes", (int64_t)raw_takes);
	jw_num(w, "total_bytes_written", raw_bytes);
	if (mbps.n) {
		stats_t s;
		stats_compute(mbps.v, mbps.n, &s);
		jw_num(w, "bitrate_min_mbps", s.min);
		jw_num(w, "bitrate_median_mbps", s.p50);
		stats_compute(bpp.v, bpp.n, &s);
		if (s.n)
			jw_num(w, "bits_per_pixel_min", s.min);
	}
	jw_int(w, "suspect_black_takes", (int64_t)nsuspects);
	jw_end_obj(w);

	/* The cost of one match end to end: recorder total plus the newest
	 * render of the same job. Parse is seconds and lives in pipeline. */
	double rms = NAN, media = NAN;
	char variant[80] = "";
	if (phase_seen[6] && latest_render_of_job(ctx, cf->job, &rms, &media, variant, sizeof variant) == 0) {
		jw_obj(w, "end_to_end");
		jw_str(w, "render_variant", variant);
		jw_num(w, "capture_ms", phase[6]);
		jw_num(w, "render_ms", rms);
		jw_num(w, "total_ms", phase[6] + rms);
		if (media > 0) {
			jw_num(w, "delivered_video_s", media);
			jw_num(w, "total_s_per_delivered_video_s", (phase[6] + rms) / 1000.0 / media);
		}
		jw_num(w, "capture_share_percent", 100.0 * phase[6] / (phase[6] + rms));
		jw_end_obj(w);
	}
	jw_end_obj(w);

	if (nsuspects) {
		finding(ctx, "warn", "capture_suspect_black",
			"job %.36s: %zu raw takes (%s) average under %.2f bits per pixel, the signature of the 5.0.0 black "
			"first-person capture",
			cf->job, nsuspects, suspects, SUSPECT_BLACK_BPP);
		finding_hint(ctx, "extract frames from those takes and run ffmpeg blackdetect/signalstats before trusting the render");
	}
	sum->video_s = video_s;
	sum->before_result_ms = phase_seen[6] ? phase[6] : NAN;
	dvec_free(&obs_fps), dvec_free(&mbps), dvec_free(&bpp);
}

int captures_report(ctx_t *ctx, jw_t *w, const char *job, long limit)
{
	char root[1100];
	path_join(root, sizeof root, ctx->data_dir, "jobs");
	capture_scan_t s = {0};
	s.root = root;
	s.job_filter = job;
	if (plat_list_dir(root, on_capture_job, &s) != 0)
		return fail(ctx, EXIT_RUNTIME, "no_jobs_dir", "no jobs directory under %s", ctx->data_dir);
	if (s.n)
		qsort(s.items, s.n, sizeof *s.items, cmp_capture_mtime);

	jw_obj(w, NULL);
	jw_str(w, "source", root);
	jw_int(w, "recording_results_found", (int64_t)s.n);
	jw_arr(w, "captures");
	dvec_t speed = {0};
	size_t shown = 0;
	for (size_t i = 0; i < s.n && shown < (size_t)limit; i++) {
		size_t len;
		char *text = read_file(s.items[i].path, &len);
		jval *doc = text ? json_parse(text, len, NULL, 0) : NULL;
		free(text);
		if (!doc)
			continue;
		capture_summary_t cs;
		write_capture(ctx, w, &s.items[i], doc, &cs);
		if (isfinite(cs.capture_s_per_video_s))
			dvec_push(&speed, cs.capture_s_per_video_s);
		json_free(doc);
		shown++;
	}
	jw_end_arr(w);
	if (speed.n) {
		stats_t st;
		stats_compute(speed.v, speed.n, &st);
		jw_stats(w, "capture_s_per_video_s", &st);
		finding(ctx, "info", "capture_speed",
			"newest capture spent %.3f s of HLAE/CS2 time per second of video (%.2fx faster than real time)",
			speed.v[0], 1.0 / speed.v[0]);
		if (speed.n >= 3) {
			stats_t prior;
			stats_compute(speed.v + 1, speed.n - 1, &prior);
			if (prior.p50 > 0 && speed.v[0] / prior.p50 >= 1.25) {
				finding(ctx, "warn", "capture_slower",
					"newest capture is %.2fx slower per video second than the median of the %zu before it",
					speed.v[0] / prior.p50, speed.n - 1);
				finding_hint(ctx, "compare the CS2 build (game/csgo/steam.inf PatchVersion) and HLAE pin "
						  "with the last fast capture; run chperf procs during a capture for CPU/IO");
			}
		}
	}
	jw_end_obj(w);
	dvec_free(&speed);
	free(s.items);
	return 0;
}

int cmd_captures(ctx_t *ctx, int argc, char **argv, jw_t *w)
{
	const char *job = NULL, *v;
	long limit = 5;
	for (int i = 0; i < argc; i++) {
		if (arg_val(argc, argv, &i, "--job", &v)) {
			if (!v)
				return fail(ctx, EXIT_USAGE, "usage", "--job needs a job id");
			job = v;
		} else if (arg_val(argc, argv, &i, "--limit", &v)) {
			if (parse_int_arg(ctx, "--limit", v, 1, 1000, &limit))
				return EXIT_USAGE;
		} else {
			return fail(ctx, EXIT_USAGE, "usage", "captures: unknown argument %s", argv[i]);
		}
	}
	return captures_report(ctx, w, job, limit);
}
