#ifndef _WIN32
#define _GNU_SOURCE
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <poll.h>
#include <signal.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/statvfs.h>
#include <sys/time.h>
#include <sys/utsname.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#include "chperf.h"

int64_t plat_now_ns(void)
{
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (int64_t)ts.tv_sec * 1000000000LL + ts.tv_nsec;
}

void plat_utc_iso(char out[40])
{
	struct timespec ts;
	clock_gettime(CLOCK_REALTIME, &ts);
	struct tm tm;
	gmtime_r(&ts.tv_sec, &tm);
	char base[20];
	strftime(base, sizeof base, "%Y-%m-%dT%H:%M:%S", &tm);
	snprintf(out, 40, "%s.%03dZ", base, (int)(ts.tv_nsec / 1000000L) % 1000);
}

void plat_unix_to_iso(int64_t unix_s, char out[40])
{
	time_t t = (time_t)unix_s;
	struct tm tm;
	if (!gmtime_r(&t, &tm)) {
		snprintf(out, 40, "unknown");
		return;
	}
	strftime(out, 40, "%Y-%m-%dT%H:%M:%SZ", &tm);
}

void plat_sleep_ms(long ms)
{
	struct timespec ts = {ms / 1000, (ms % 1000) * 1000000L};
	while (nanosleep(&ts, &ts) != 0 && errno == EINTR)
		;
}

const char *plat_getenv(const char *name)
{
	const char *v = getenv(name);
	return v && *v ? v : NULL;
}

int plat_sysinfo(sysinfo_t *si)
{
	memset(si, 0, sizeof *si);
	si->logical_cpus = (int)sysconf(_SC_NPROCESSORS_ONLN);
	long pages = sysconf(_SC_PHYS_PAGES), avail = sysconf(_SC_AVPHYS_PAGES), page = sysconf(_SC_PAGESIZE);
	if (pages > 0 && page > 0) {
		si->ram_total_bytes = (uint64_t)pages * (uint64_t)page;
		si->ram_avail_bytes = (uint64_t)(avail > 0 ? avail : 0) * (uint64_t)page;
	}
	FILE *f = fopen("/proc/meminfo", "r");
	if (f) {
		char line[256];
		unsigned long long kb;
		while (fgets(line, sizeof line, f))
			if (sscanf(line, "MemAvailable: %llu kB", &kb) == 1)
				si->ram_avail_bytes = kb * 1024ULL;
		fclose(f);
	}
	f = fopen("/proc/cpuinfo", "r");
	if (f) {
		char line[512];
		while (fgets(line, sizeof line, f)) {
			if (strncmp(line, "model name", 10) == 0) {
				char *c = strchr(line, ':');
				if (c) {
					c++;
					while (*c == ' ')
						c++;
					c[strcspn(c, "\n")] = 0;
					snprintf(si->cpu_name, sizeof si->cpu_name, "%s", c);
				}
				break;
			}
		}
		fclose(f);
	}
	struct utsname u;
	if (uname(&u) == 0) {
		snprintf(si->os, sizeof si->os, "%s %s", u.sysname, u.release);
		snprintf(si->arch, sizeof si->arch, "%.15s",
			 strcmp(u.machine, "x86_64") == 0 ? "amd64" : strcmp(u.machine, "aarch64") == 0 ? "arm64" : u.machine);
	}
	return 0;
}

int plat_disk_space(const char *path, uint64_t *free_b, uint64_t *total_b)
{
	struct statvfs s;
	if (statvfs(path, &s) != 0)
		return -1;
	*free_b = (uint64_t)s.f_bavail * s.f_frsize;
	*total_b = (uint64_t)s.f_blocks * s.f_frsize;
	return 0;
}

int plat_list_dir(const char *path, dir_cb cb, void *ctx)
{
	DIR *d = opendir(path);
	if (!d)
		return -1;
	struct dirent *e;
	char full[2048];
	while ((e = readdir(d))) {
		if (strcmp(e->d_name, ".") == 0 || strcmp(e->d_name, "..") == 0)
			continue;
		path_join(full, sizeof full, path, e->d_name);
		if (cb(e->d_name, plat_is_dir(full), ctx))
			break;
	}
	closedir(d);
	return 0;
}

int plat_file_mtime(const char *path, int64_t *unix_s)
{
	struct stat st;
	if (stat(path, &st) != 0)
		return -1;
	*unix_s = (int64_t)st.st_mtime;
	return 0;
}

int plat_is_dir(const char *path)
{
	struct stat st;
	return stat(path, &st) == 0 && S_ISDIR(st.st_mode);
}

/* ---- processes (/proc) ---- */

static int is_pid_dir(const char *name)
{
	for (const char *p = name; *p; p++)
		if (*p < '0' || *p > '9')
			return 0;
	return *name != 0;
}

typedef struct {
	proc_entry_t *list;
	size_t n, cap;
} plist_t;

/* Reads comm, ppid and threads from /proc/<pid>/stat. */
static int read_stat(uint32_t pid, char *comm, size_t commcap, uint32_t *ppid, uint32_t *threads, uint64_t *cpu_ns)
{
	char path[64], buf[1024];
	snprintf(path, sizeof path, "/proc/%u/stat", pid);
	FILE *f = fopen(path, "r");
	if (!f)
		return -1;
	size_t n = fread(buf, 1, sizeof buf - 1, f);
	fclose(f);
	buf[n] = 0;
	char *l = strchr(buf, '('), *r = strrchr(buf, ')');
	if (!l || !r)
		return -1;
	if (comm) {
		size_t cn = (size_t)(r - l - 1);
		if (cn >= commcap)
			cn = commcap - 1;
		memcpy(comm, l + 1, cn);
		comm[cn] = 0;
	}
	/* Fields after ')': state(3) ppid(4) ... utime(14) stime(15) ... num_threads(20). */
	char state;
	unsigned long p, ut, st;
	long nth;
	int got = sscanf(r + 2, "%c %lu %*d %*d %*d %*d %*u %*u %*u %*u %*u %lu %lu %*d %*d %*d %*d %ld", &state, &p,
			 &ut, &st, &nth);
	if (got != 5)
		return -1;
	if (ppid)
		*ppid = (uint32_t)p;
	if (threads)
		*threads = (uint32_t)nth;
	if (cpu_ns) {
		long hz = sysconf(_SC_CLK_TCK);
		*cpu_ns = (uint64_t)(ut + st) * (1000000000ULL / (uint64_t)(hz > 0 ? hz : 100));
	}
	return 0;
}

static int proc_cb(const char *name, int is_dir, void *vctx)
{
	plist_t *pl = vctx;
	if (!is_dir || !is_pid_dir(name))
		return 0;
	if (pl->n == pl->cap) {
		size_t cap = pl->cap ? pl->cap * 2 : 256;
		proc_entry_t *p = realloc(pl->list, cap * sizeof *p);
		if (!p)
			return 1;
		pl->list = p;
		pl->cap = cap;
	}
	proc_entry_t *e = &pl->list[pl->n];
	memset(e, 0, sizeof *e);
	e->pid = (uint32_t)strtoul(name, NULL, 10);
	if (read_stat(e->pid, e->name, sizeof e->name, &e->ppid, &e->threads, NULL) == 0)
		pl->n++;
	return 0;
}

int plat_proc_list(proc_entry_t **out, size_t *n)
{
	plist_t pl = {0};
	if (plat_list_dir("/proc", proc_cb, &pl) != 0)
		return -1;
	*out = pl.list;
	*n = pl.n;
	return 0;
}

int plat_proc_detail(uint32_t pid, proc_detail_t *d)
{
	memset(d, 0, sizeof *d);
	if (read_stat(pid, NULL, 0, NULL, NULL, &d->cpu_ns) != 0)
		return -1;
	char path[64], line[256];
	snprintf(path, sizeof path, "/proc/%u/status", pid);
	FILE *f = fopen(path, "r");
	if (f) {
		unsigned long long kb;
		while (fgets(line, sizeof line, f)) {
			if (sscanf(line, "VmRSS: %llu kB", &kb) == 1)
				d->ws_bytes = kb * 1024ULL;
			else if (sscanf(line, "RssAnon: %llu kB", &kb) == 1)
				d->private_bytes = kb * 1024ULL;
		}
		fclose(f);
	}
	snprintf(path, sizeof path, "/proc/%u/io", pid);
	f = fopen(path, "r");
	if (f) {
		unsigned long long v;
		while (fgets(line, sizeof line, f)) {
			if (sscanf(line, "read_bytes: %llu", &v) == 1)
				d->io_read_bytes = v;
			else if (sscanf(line, "write_bytes: %llu", &v) == 1)
				d->io_write_bytes = v;
		}
		fclose(f);
	}
	snprintf(path, sizeof path, "/proc/%u/fd", pid);
	DIR *fd = opendir(path);
	if (fd) {
		struct dirent *e;
		while ((e = readdir(fd)))
			if (e->d_name[0] != '.')
				d->handles++;
		closedir(fd);
	}
	d->ok = 1;
	return 0;
}

int plat_system_cpu(uint64_t *busy, uint64_t *total)
{
	FILE *f = fopen("/proc/stat", "r");
	if (!f)
		return -1;
	unsigned long long v[10] = {0};
	int n = fscanf(f, "cpu %llu %llu %llu %llu %llu %llu %llu %llu %llu %llu", &v[0], &v[1], &v[2], &v[3], &v[4],
		       &v[5], &v[6], &v[7], &v[8], &v[9]);
	fclose(f);
	if (n < 4)
		return -1;
	uint64_t t = 0;
	for (int i = 0; i < 8; i++) /* guest time is already inside user */
		t += v[i];
	*total = t;
	*busy = t - v[3] - v[4];
	return 0;
}

/* ---- run ---- */

static double tv_ms(struct timeval tv) { return (double)tv.tv_sec * 1000.0 + (double)tv.tv_usec / 1000.0; }

int plat_run(char **argv, long timeout_s, int keep_full, run_result_t *r)
{
	memset(r, 0, sizeof *r);
	r->exit_code = -1;
	r->peak_memory_kind = "max_child_rss";
	char tmpl[] = "/tmp/chperf-XXXXXX";
	int out = mkstemp(tmpl);
	if (out < 0) {
		snprintf(r->err, sizeof r->err, "mkstemp failed: %s", strerror(errno));
		return -1;
	}
	unlink(tmpl);
	struct rusage before;
	getrusage(RUSAGE_CHILDREN, &before);
	int64_t t0 = plat_now_ns();
	pid_t pid = fork();
	if (pid < 0) {
		close(out);
		snprintf(r->err, sizeof r->err, "fork failed: %s", strerror(errno));
		return -1;
	}
	if (pid == 0) {
		setpgid(0, 0);
		int nul = open("/dev/null", O_RDONLY);
		if (nul >= 0)
			dup2(nul, 0);
		dup2(out, 1);
		dup2(out, 2);
		execvp(argv[0], argv);
		dprintf(2, "chperf: exec %s: %s\n", argv[0], strerror(errno));
		_exit(127);
	}
	setpgid(pid, pid);
	r->started = 1;
	int status = 0;
	int64_t deadline = timeout_s > 0 ? t0 + (int64_t)timeout_s * 1000000000LL : 0;
	for (;;) {
		pid_t w = waitpid(pid, &status, deadline ? WNOHANG : 0);
		if (w == pid)
			break;
		if (w < 0 && errno != EINTR)
			break;
		if (deadline && plat_now_ns() > deadline) {
			r->timed_out = 1;
			kill(-pid, SIGKILL);
			waitpid(pid, &status, 0);
			break;
		}
		if (deadline)
			plat_sleep_ms(20);
	}
	r->wall_ms = (double)(plat_now_ns() - t0) / 1e6;
	/* Any process still in the group outlived the root. */
	if (kill(-pid, 0) == 0) {
		r->stragglers = 1;
		kill(-pid, SIGKILL);
	}
	struct rusage after;
	getrusage(RUSAGE_CHILDREN, &after);
	r->user_ms = tv_ms(after.ru_utime) - tv_ms(before.ru_utime);
	r->sys_ms = tv_ms(after.ru_stime) - tv_ms(before.ru_stime);
	r->peak_memory_bytes = (uint64_t)after.ru_maxrss * 1024ULL;
	r->io_read_bytes = (uint64_t)(after.ru_inblock - before.ru_inblock) * 512ULL;
	r->io_write_bytes = (uint64_t)(after.ru_oublock - before.ru_oublock) * 512ULL;
	r->total_processes = 0; /* not observable without cgroups */
	if (WIFEXITED(status))
		r->exit_code = WEXITSTATUS(status);
	else if (WIFSIGNALED(status))
		r->exit_code = 128 + WTERMSIG(status);
	off_t size = lseek(out, 0, SEEK_END);
	if (size > 0) {
		r->output_bytes = (uint64_t)size;
		off_t keep = (off_t)sizeof r->output_tail - 1;
		lseek(out, size > keep ? size - keep : 0, SEEK_SET);
		ssize_t got = read(out, r->output_tail, (size_t)keep);
		r->output_tail[got > 0 ? got : 0] = 0;
	}
	if (keep_full) {
		sb_t sb = {0};
		char buf[65536];
		ssize_t k;
		lseek(out, 0, SEEK_SET);
		while ((k = read(out, buf, sizeof buf)) > 0)
			sb_add(&sb, buf, (size_t)k);
		if (!sb.p)
			sb_add(&sb, "", 0);
		r->output_full = sb.p;
	}
	close(out);
	return 0;
}

/* ---- sockets ---- */

int plat_net_init(void)
{
	signal(SIGPIPE, SIG_IGN);
	return 0;
}

int plat_tcp_connect(const char *host, int port, long timeout_ms, sock_t *s, char *err, size_t errlen)
{
	char portstr[16];
	snprintf(portstr, sizeof portstr, "%d", port);
	struct addrinfo hints = {0}, *res = NULL;
	hints.ai_family = AF_UNSPEC;
	hints.ai_socktype = SOCK_STREAM;
	if (getaddrinfo(host, portstr, &hints, &res) != 0 || !res) {
		snprintf(err, errlen, "cannot resolve %s", host);
		return -1;
	}
	int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd < 0) {
		freeaddrinfo(res);
		snprintf(err, errlen, "socket failed");
		return -1;
	}
	int fl = fcntl(fd, F_GETFL, 0);
	fcntl(fd, F_SETFL, fl | O_NONBLOCK);
	int rc = connect(fd, res->ai_addr, res->ai_addrlen);
	freeaddrinfo(res);
	if (rc != 0 && errno == EINPROGRESS) {
		struct pollfd p = {fd, POLLOUT, 0};
		int soerr = 0;
		socklen_t sl = sizeof soerr;
		rc = (poll(&p, 1, (int)timeout_ms) == 1 && getsockopt(fd, SOL_SOCKET, SO_ERROR, &soerr, &sl) == 0 &&
		      soerr == 0)
			     ? 0
			     : -1;
	}
	if (rc != 0) {
		close(fd);
		snprintf(err, errlen, "connect to %s:%d failed or timed out", host, port);
		return -1;
	}
	fcntl(fd, F_SETFL, fl);
	struct timeval tv = {timeout_ms / 1000, (timeout_ms % 1000) * 1000};
	setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof tv);
	setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof tv);
	*s = fd;
	return 0;
}

int plat_send_all(sock_t s, const char *buf, size_t n)
{
	while (n > 0) {
		ssize_t k = send((int)s, buf, n, 0);
		if (k <= 0)
			return -1;
		buf += k;
		n -= (size_t)k;
	}
	return 0;
}

long plat_recv(sock_t s, char *buf, size_t n) { return (long)recv((int)s, buf, n, 0); }

void plat_sock_close(sock_t s) { close((int)s); }

#endif
