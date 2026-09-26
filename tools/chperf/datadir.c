#include "chperf.h"

#include <stdlib.h>
#include <string.h>

/* ---- data dir ---- */

static int has_evidence(const char *dir)
{
	char p[1100];
	path_join(p, sizeof p, dir, "obs");
	if (plat_is_dir(p))
		return 1;
	path_join(p, sizeof p, dir, "jobs");
	return plat_is_dir(p);
}

/* Order: --data-dir, $ZV_DATA_DIR (what workers use), the installed Studio's
 * data dir, then ./data for a repo checkout. */
int resolve_data_dir(ctx_t *ctx, const char *flag)
{
	char cand[1024];
	if (flag) {
		snprintf(ctx->data_dir, sizeof ctx->data_dir, "%s", flag);
		snprintf(ctx->data_dir_source, sizeof ctx->data_dir_source, "flag");
		return 0;
	}
	const char *env = plat_getenv("ZV_DATA_DIR");
	if (env) {
		snprintf(ctx->data_dir, sizeof ctx->data_dir, "%s", env);
		snprintf(ctx->data_dir_source, sizeof ctx->data_dir_source, "ZV_DATA_DIR");
		return 0;
	}
#ifdef _WIN32
	const char *appdata = plat_getenv("APPDATA");
	if (appdata) {
		snprintf(cand, sizeof cand, "%s/cliphub-studio/data", appdata);
		for (char *c = cand; *c; c++)
			if (*c == '\\')
				*c = '/';
		if (plat_is_dir(cand)) {
			snprintf(ctx->data_dir, sizeof ctx->data_dir, "%s", cand);
			snprintf(ctx->data_dir_source, sizeof ctx->data_dir_source, "studio");
			return 0;
		}
	}
#else
	const char *xdg = plat_getenv("XDG_CONFIG_HOME"), *home = plat_getenv("HOME");
	if (xdg)
		snprintf(cand, sizeof cand, "%s/cliphub-studio/data", xdg);
	else if (home)
		snprintf(cand, sizeof cand, "%s/.config/cliphub-studio/data", home);
	else
		cand[0] = 0;
	if (cand[0] && plat_is_dir(cand)) {
		snprintf(ctx->data_dir, sizeof ctx->data_dir, "%s", cand);
		snprintf(ctx->data_dir_source, sizeof ctx->data_dir_source, "studio");
		return 0;
	}
#endif
	snprintf(ctx->data_dir, sizeof ctx->data_dir, "data");
	snprintf(ctx->data_dir_source, sizeof ctx->data_dir_source, has_evidence("data") ? "cwd" : "cwd-empty");
	return 0;
}

/* Studio writes {"orchestrator":PORT,"web":PORT} next to its data dir. */
int orchestrator_port(ctx_t *ctx)
{
	char parent[1024], path[1100];
	snprintf(parent, sizeof parent, "%s", ctx->data_dir);
	size_t n = strlen(parent);
	while (n && (parent[n - 1] == '/' || parent[n - 1] == '\\'))
		parent[--n] = 0;
	char *slash = strrchr(parent, '/'), *bslash = strrchr(parent, '\\');
	if (bslash > slash)
		slash = bslash;
	if (slash)
		*slash = 0;
	else
		snprintf(parent, sizeof parent, ".");
	path_join(path, sizeof path, parent, "ports.json");
	size_t len;
	char *text = read_file(path, &len);
	if (!text)
		return -1;
	jval *v = json_parse(text, len, NULL, 0);
	free(text);
	int port = (int)jnum(jget(v, "orchestrator"), -1);
	json_free(v);
	return port;
}
