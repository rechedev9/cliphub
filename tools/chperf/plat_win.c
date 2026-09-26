#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#define PSAPI_VERSION 2
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <psapi.h>
#include <tlhelp32.h>

#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "chperf.h"

/* 100 ns ticks between 1601-01-01 and 1970-01-01. */
#define EPOCH_DIFF_100NS 116444736000000000ULL

static uint64_t ft_u64(FILETIME ft) { return ((uint64_t)ft.dwHighDateTime << 32) | ft.dwLowDateTime; }

int64_t plat_now_ns(void)
{
	static LARGE_INTEGER freq;
	LARGE_INTEGER now;
	if (!freq.QuadPart)
		QueryPerformanceFrequency(&freq);
	QueryPerformanceCounter(&now);
	return (int64_t)((double)now.QuadPart * 1e9 / (double)freq.QuadPart);
}

void plat_utc_iso(char out[40])
{
	SYSTEMTIME st;
	GetSystemTime(&st);
	snprintf(out, 40, "%04u-%02u-%02uT%02u:%02u:%02u.%03uZ", st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute,
		 st.wSecond, st.wMilliseconds);
}

void plat_unix_to_iso(int64_t unix_s, char out[40])
{
	__time64_t t = unix_s;
	struct tm tm;
	if (_gmtime64_s(&tm, &t) != 0) {
		snprintf(out, 40, "unknown");
		return;
	}
	strftime(out, 40, "%Y-%m-%dT%H:%M:%SZ", &tm);
}

void plat_sleep_ms(long ms) { Sleep((DWORD)(ms < 0 ? 0 : ms)); }

const char *plat_getenv(const char *name)
{
	const char *v = getenv(name);
	return v && *v ? v : NULL;
}

static int reg_string(HKEY root, const char *key, const char *name, char *out, DWORD cap)
{
	DWORD size = cap;
	if (RegGetValueA(root, key, name, RRF_RT_REG_SZ, NULL, out, &size) != ERROR_SUCCESS) {
		out[0] = 0;
		return -1;
	}
	return 0;
}

int plat_sysinfo(sysinfo_t *si)
{
	memset(si, 0, sizeof *si);
	reg_string(HKEY_LOCAL_MACHINE, "HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorNameString",
		   si->cpu_name, sizeof si->cpu_name);
	/* Trim the padding some vendors put in the brand string. */
	size_t n = strlen(si->cpu_name);
	while (n && si->cpu_name[n - 1] == ' ')
		si->cpu_name[--n] = 0;
	si->logical_cpus = (int)GetActiveProcessorCount(ALL_PROCESSOR_GROUPS);
	MEMORYSTATUSEX ms = {sizeof ms};
	if (GlobalMemoryStatusEx(&ms)) {
		si->ram_total_bytes = ms.ullTotalPhys;
		si->ram_avail_bytes = ms.ullAvailPhys;
	}
	char product[96] = "", display[32] = "", build[16] = "";
	const char *cv = "SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion";
	reg_string(HKEY_LOCAL_MACHINE, cv, "ProductName", product, sizeof product);
	reg_string(HKEY_LOCAL_MACHINE, cv, "DisplayVersion", display, sizeof display);
	reg_string(HKEY_LOCAL_MACHINE, cv, "CurrentBuild", build, sizeof build);
	/* Windows 11 keeps "Windows 10" in ProductName; the build number decides. */
	if (atoi(build) >= 22000 && strncmp(product, "Windows 10", 10) == 0)
		product[8] = '1', product[9] = '1';
	snprintf(si->os, sizeof si->os, "%s %s (build %s)", product, display, build);
	SYSTEM_INFO sys;
	GetNativeSystemInfo(&sys);
	snprintf(si->arch, sizeof si->arch, "%s",
		 sys.wProcessorArchitecture == PROCESSOR_ARCHITECTURE_AMD64   ? "amd64"
		 : sys.wProcessorArchitecture == PROCESSOR_ARCHITECTURE_ARM64 ? "arm64"
									      : "other");
	return 0;
}

int plat_disk_space(const char *path, uint64_t *free_b, uint64_t *total_b)
{
	ULARGE_INTEGER avail, total;
	if (!GetDiskFreeSpaceExA(path, &avail, &total, NULL))
		return -1;
	*free_b = avail.QuadPart;
	*total_b = total.QuadPart;
	return 0;
}

int plat_list_dir(const char *path, dir_cb cb, void *ctx)
{
	char pattern[1100];
	snprintf(pattern, sizeof pattern, "%s\\*", path);
	WIN32_FIND_DATAA fd;
	HANDLE h = FindFirstFileA(pattern, &fd);
	if (h == INVALID_HANDLE_VALUE)
		return -1;
	do {
		if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0)
			continue;
		if (cb(fd.cFileName, (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) != 0, ctx))
			break;
	} while (FindNextFileA(h, &fd));
	FindClose(h);
	return 0;
}

int plat_file_mtime(const char *path, int64_t *unix_s)
{
	WIN32_FILE_ATTRIBUTE_DATA fa;
	if (!GetFileAttributesExA(path, GetFileExInfoStandard, &fa))
		return -1;
	*unix_s = (int64_t)((ft_u64(fa.ftLastWriteTime) - EPOCH_DIFF_100NS) / 10000000ULL);
	return 0;
}

int plat_is_dir(const char *path)
{
	DWORD a = GetFileAttributesA(path);
	return a != INVALID_FILE_ATTRIBUTES && (a & FILE_ATTRIBUTE_DIRECTORY);
}

/* ---- processes ---- */

int plat_proc_list(proc_entry_t **out, size_t *n)
{
	*out = NULL;
	*n = 0;
	HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
	if (snap == INVALID_HANDLE_VALUE)
		return -1;
	size_t cap = 256;
	proc_entry_t *list = malloc(cap * sizeof *list);
	PROCESSENTRY32W pe = {sizeof pe};
	if (list && Process32FirstW(snap, &pe)) {
		do {
			if (*n == cap) {
				cap *= 2;
				proc_entry_t *p = realloc(list, cap * sizeof *list);
				if (!p)
					break;
				list = p;
			}
			proc_entry_t *e = &list[(*n)++];
			e->pid = pe.th32ProcessID;
			e->ppid = pe.th32ParentProcessID;
			e->threads = pe.cntThreads;
			if (!WideCharToMultiByte(CP_UTF8, 0, pe.szExeFile, -1, e->name, sizeof e->name, NULL, NULL))
				e->name[0] = 0;
		} while (Process32NextW(snap, &pe));
	}
	CloseHandle(snap);
	*out = list;
	return list ? 0 : -1;
}

int plat_proc_detail(uint32_t pid, proc_detail_t *d)
{
	memset(d, 0, sizeof *d);
	HANDLE h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
	if (!h)
		return -1;
	FILETIME c, e, k, u;
	if (GetProcessTimes(h, &c, &e, &k, &u))
		d->cpu_ns = (ft_u64(k) + ft_u64(u)) * 100ULL;
	PROCESS_MEMORY_COUNTERS_EX pmc = {0};
	pmc.cb = sizeof pmc;
	if (GetProcessMemoryInfo(h, (PROCESS_MEMORY_COUNTERS *)&pmc, sizeof pmc)) {
		d->ws_bytes = pmc.WorkingSetSize;
		d->private_bytes = pmc.PrivateUsage;
	}
	IO_COUNTERS io;
	if (GetProcessIoCounters(h, &io)) {
		d->io_read_bytes = io.ReadTransferCount;
		d->io_write_bytes = io.WriteTransferCount;
	}
	DWORD handles = 0;
	if (GetProcessHandleCount(h, &handles))
		d->handles = handles;
	CloseHandle(h);
	d->ok = 1;
	return 0;
}

int plat_system_cpu(uint64_t *busy, uint64_t *total)
{
	FILETIME idle, kernel, user;
	if (!GetSystemTimes(&idle, &kernel, &user))
		return -1;
	/* Kernel time includes idle time. */
	*total = ft_u64(kernel) + ft_u64(user);
	*busy = *total - ft_u64(idle);
	return 0;
}

/* ---- run ---- */

/* Quote one argument by the MSVCRT CommandLineToArgvW rules. */
static void quote_arg(sb_t *sb, const char *a)
{
	if (*a && !strpbrk(a, " \t\n\v\"")) {
		sb_puts(sb, a);
		return;
	}
	sb_puts(sb, "\"");
	for (const char *p = a;; p++) {
		size_t bs = 0;
		while (*p == '\\') {
			p++;
			bs++;
		}
		if (!*p) {
			for (size_t i = 0; i < bs * 2; i++)
				sb_puts(sb, "\\");
			break;
		}
		if (*p == '"') {
			for (size_t i = 0; i < bs * 2 + 1; i++)
				sb_puts(sb, "\\");
			sb_puts(sb, "\"");
		} else {
			for (size_t i = 0; i < bs; i++)
				sb_puts(sb, "\\");
			sb_add(sb, p, 1);
		}
	}
	sb_puts(sb, "\"");
}

static void read_tail(HANDLE f, run_result_t *r)
{
	LARGE_INTEGER size;
	if (!GetFileSizeEx(f, &size))
		return;
	r->output_bytes = (uint64_t)size.QuadPart;
	LONGLONG keep = (LONGLONG)sizeof r->output_tail - 1;
	LARGE_INTEGER off;
	off.QuadPart = size.QuadPart > keep ? size.QuadPart - keep : 0;
	if (!SetFilePointerEx(f, off, NULL, FILE_BEGIN))
		return;
	DWORD got = 0;
	if (ReadFile(f, r->output_tail, (DWORD)keep, &got, NULL))
		r->output_tail[got] = 0;
}

static void read_full(HANDLE f, run_result_t *r)
{
	LARGE_INTEGER zero = {0};
	if (!SetFilePointerEx(f, zero, NULL, FILE_BEGIN))
		return;
	sb_t sb = {0};
	char buf[65536];
	DWORD got;
	while (ReadFile(f, buf, sizeof buf, &got, NULL) && got > 0)
		sb_add(&sb, buf, got);
	if (!sb.p)
		sb_add(&sb, "", 0);
	r->output_full = sb.p;
}

int plat_run(char **argv, long timeout_s, int keep_full, run_result_t *r)
{
	memset(r, 0, sizeof *r);
	r->exit_code = -1;
	r->peak_memory_kind = "job_committed";

	sb_t cmd = {0};
	for (int i = 0; argv[i]; i++) {
		if (i)
			sb_puts(&cmd, " ");
		quote_arg(&cmd, argv[i]);
	}
	wchar_t *wcmd = NULL;
	int wn = MultiByteToWideChar(CP_UTF8, 0, cmd.p, -1, NULL, 0);
	if (wn > 0 && (wcmd = malloc((size_t)wn * sizeof *wcmd)))
		MultiByteToWideChar(CP_UTF8, 0, cmd.p, -1, wcmd, wn);
	sb_free(&cmd);
	if (!wcmd) {
		snprintf(r->err, sizeof r->err, "cannot encode command line");
		return -1;
	}

	SECURITY_ATTRIBUTES sa = {sizeof sa, NULL, TRUE};
	char tmpdir[MAX_PATH], tmpfile[MAX_PATH];
	GetTempPathA(sizeof tmpdir, tmpdir);
	GetTempFileNameA(tmpdir, "chp", 0, tmpfile);
	/* The child's output goes to a delete-on-close file so the JSON on our
	 * stdout stays clean; only its tail is reported. */
	HANDLE out = CreateFileA(tmpfile, GENERIC_READ | GENERIC_WRITE, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
				 &sa, CREATE_ALWAYS, FILE_ATTRIBUTE_TEMPORARY | FILE_FLAG_DELETE_ON_CLOSE, NULL);
	HANDLE nul = CreateFileA("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE, &sa, OPEN_EXISTING, 0, NULL);
	HANDLE job = CreateJobObjectA(NULL, NULL);
	if (out == INVALID_HANDLE_VALUE || nul == INVALID_HANDLE_VALUE || !job) {
		snprintf(r->err, sizeof r->err, "cannot prepare child (error %lu)", GetLastError());
		free(wcmd);
		return -1;
	}
	JOBOBJECT_EXTENDED_LIMIT_INFORMATION lim = {0};
	lim.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
	SetInformationJobObject(job, JobObjectExtendedLimitInformation, &lim, sizeof lim);

	/* Inherit only the two handles we mean to, never our own stdout pipe:
	 * a straggling grandchild holding it would keep the caller waiting. */
	HANDLE inherit[2] = {out, nul};
	SIZE_T attr_size = 0;
	InitializeProcThreadAttributeList(NULL, 1, 0, &attr_size);
	LPPROC_THREAD_ATTRIBUTE_LIST attrs = malloc(attr_size);
	STARTUPINFOEXW si = {0};
	si.StartupInfo.cb = sizeof si;
	si.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
	si.StartupInfo.hStdInput = nul;
	si.StartupInfo.hStdOutput = out;
	si.StartupInfo.hStdError = out;
	if (attrs && InitializeProcThreadAttributeList(attrs, 1, 0, &attr_size) &&
	    UpdateProcThreadAttribute(attrs, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, inherit, sizeof inherit, NULL, NULL))
		si.lpAttributeList = attrs;

	PROCESS_INFORMATION pi = {0};
	int64_t t0 = plat_now_ns();
	BOOL ok = CreateProcessW(NULL, wcmd, NULL, NULL, TRUE,
				 CREATE_SUSPENDED | CREATE_NO_WINDOW | (si.lpAttributeList ? EXTENDED_STARTUPINFO_PRESENT : 0),
				 NULL, NULL, &si.StartupInfo, &pi);
	DWORD create_err = GetLastError();
	free(wcmd);
	if (si.lpAttributeList)
		DeleteProcThreadAttributeList(attrs);
	free(attrs);
	if (!ok) {
		snprintf(r->err, sizeof r->err, "CreateProcess failed (error %lu): is \"%s\" on PATH and an .exe?",
			 create_err, argv[0]);
		CloseHandle(out);
		CloseHandle(nul);
		CloseHandle(job);
		return -1;
	}
	r->started = 1;
	AssignProcessToJobObject(job, pi.hProcess);
	ResumeThread(pi.hThread);
	CloseHandle(pi.hThread);

	DWORD wait = WaitForSingleObject(pi.hProcess, timeout_s > 0 ? (DWORD)timeout_s * 1000 : INFINITE);
	if (wait == WAIT_TIMEOUT) {
		r->timed_out = 1;
		TerminateJobObject(job, 1);
		WaitForSingleObject(pi.hProcess, 5000);
	}
	r->wall_ms = (double)(plat_now_ns() - t0) / 1e6;
	DWORD code = 0;
	if (GetExitCodeProcess(pi.hProcess, &code))
		r->exit_code = (int)code;
	CloseHandle(pi.hProcess);

	/* Give children that outlive the root (go run, cmd /c) a moment to finish. */
	JOBOBJECT_BASIC_AND_IO_ACCOUNTING_INFORMATION acc = {0};
	for (int i = 0; i < 40; i++) {
		if (!QueryInformationJobObject(job, JobObjectBasicAndIoAccountingInformation, &acc, sizeof acc, NULL))
			break;
		if (acc.BasicInfo.ActiveProcesses == 0)
			break;
		Sleep(50);
	}
	r->stragglers = acc.BasicInfo.ActiveProcesses;
	r->user_ms = (double)acc.BasicInfo.TotalUserTime.QuadPart / 1e4;
	r->sys_ms = (double)acc.BasicInfo.TotalKernelTime.QuadPart / 1e4;
	r->total_processes = acc.BasicInfo.TotalProcesses;
	r->io_read_bytes = acc.IoInfo.ReadTransferCount;
	r->io_write_bytes = acc.IoInfo.WriteTransferCount;
	JOBOBJECT_EXTENDED_LIMIT_INFORMATION ext = {0};
	if (QueryInformationJobObject(job, JobObjectExtendedLimitInformation, &ext, sizeof ext, NULL))
		r->peak_memory_bytes = ext.PeakJobMemoryUsed;
	TerminateJobObject(job, 1);
	CloseHandle(job);
	read_tail(out, r);
	if (keep_full)
		read_full(out, r);
	CloseHandle(out);
	CloseHandle(nul);
	return 0;
}

/* ---- sockets ---- */

int plat_net_init(void)
{
	static int done;
	if (done)
		return 0;
	WSADATA wd;
	if (WSAStartup(MAKEWORD(2, 2), &wd) != 0)
		return -1;
	done = 1;
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
	SOCKET fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd == INVALID_SOCKET) {
		freeaddrinfo(res);
		snprintf(err, errlen, "socket failed");
		return -1;
	}
	u_long nb = 1;
	ioctlsocket(fd, FIONBIO, &nb);
	int rc = connect(fd, res->ai_addr, (int)res->ai_addrlen);
	freeaddrinfo(res);
	if (rc != 0 && WSAGetLastError() == WSAEWOULDBLOCK) {
		fd_set wr, ex;
		FD_ZERO(&wr);
		FD_ZERO(&ex);
		FD_SET(fd, &wr);
		FD_SET(fd, &ex);
		struct timeval tv = {timeout_ms / 1000, (timeout_ms % 1000) * 1000};
		rc = select(0, NULL, &wr, &ex, &tv) == 1 && FD_ISSET(fd, &wr) ? 0 : -1;
	}
	if (rc != 0) {
		closesocket(fd);
		snprintf(err, errlen, "connect to %s:%d failed or timed out", host, port);
		return -1;
	}
	nb = 0;
	ioctlsocket(fd, FIONBIO, &nb);
	DWORD tv = (DWORD)timeout_ms;
	setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, (const char *)&tv, sizeof tv);
	setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, (const char *)&tv, sizeof tv);
	*s = (sock_t)fd;
	return 0;
}

int plat_send_all(sock_t s, const char *buf, size_t n)
{
	while (n > 0) {
		int k = send((SOCKET)s, buf, (int)n, 0);
		if (k <= 0)
			return -1;
		buf += k;
		n -= (size_t)k;
	}
	return 0;
}

long plat_recv(sock_t s, char *buf, size_t n) { return recv((SOCKET)s, buf, (int)n, 0); }

void plat_sock_close(sock_t s) { closesocket((SOCKET)s); }

#endif
