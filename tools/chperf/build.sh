#!/usr/bin/env sh
# Builds chperf into bin/ at the repo root. Needs only a C11 compiler
# (Windows: scoop gcc / MinGW-w64 from Git Bash; Linux: gcc or clang).
#   tools/chperf/build.sh         build bin/chperf(.exe)
#   tools/chperf/build.sh test    build, then run the unit and CLI tests
set -eu
cd "$(dirname "$0")"
CC="${CC:-gcc}"
BIN="../../bin"
CFLAGS="-std=c11 -O2 -Wall -Wextra -Wno-unused-parameter -Wno-missing-field-initializers -Werror"
SRC="util.c json.c datadir.c cmd_data.c cmd_live.c cmd_compare.c cmd_capture.c cmd_bench.c plat_win.c plat_posix.c"
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN*)
	EXE=.exe
	CFLAGS="$CFLAGS -D_WIN32_WINNT=0x0A00"
	LIBS="-lpsapi -lws2_32"
	;;
*)
	EXE=
	CFLAGS="$CFLAGS -D_POSIX_C_SOURCE=200809L"
	LIBS="-lm"
	;;
esac
mkdir -p "$BIN"
# shellcheck disable=SC2086
"$CC" $CFLAGS -o "$BIN/chperf$EXE" main.c $SRC $LIBS
if [ "${1:-}" = "test" ]; then
	# shellcheck disable=SC2086
	"$CC" $CFLAGS -I. -o "$BIN/chperf-test$EXE" test/test_chperf.c $SRC $LIBS
	"$BIN/chperf-test$EXE" "$(cd "$BIN" && pwd)/chperf$EXE"
fi
