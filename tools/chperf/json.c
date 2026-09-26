/* Minimal RFC 8259 parser into a heap DOM. Inputs are ClipHub's own evidence
 * files (a few MB at most), so a recursive parser with a depth cap is enough. */
#include "chperf.h"

#include <stdlib.h>
#include <string.h>

#define JSON_MAX_DEPTH 256

typedef struct {
	const char *start, *p, *end;
	char *err;
	size_t errlen;
	int depth;
} parser_t;

static int perr(parser_t *ps, const char *msg)
{
	if (ps->err && ps->errlen && !ps->err[0])
		snprintf(ps->err, ps->errlen, "%s at byte %ld", msg, (long)(ps->p - ps->start));
	return -1;
}

static void skip_ws(parser_t *ps)
{
	while (ps->p < ps->end && (*ps->p == ' ' || *ps->p == '\t' || *ps->p == '\n' || *ps->p == '\r'))
		ps->p++;
}

static void free_inner(jval *v)
{
	if (v->type == J_STR)
		free(v->str);
	if (v->type == J_ARR || v->type == J_OBJ) {
		for (size_t i = 0; i < v->n; i++) {
			free_inner(&v->items[i]);
			if (v->keys)
				free(v->keys[i]);
		}
		free(v->items);
		free(v->keys);
	}
}

void json_free(jval *v)
{
	if (!v)
		return;
	free_inner(v);
	free(v);
}

static int hexval(char c)
{
	if (c >= '0' && c <= '9') return c - '0';
	if (c >= 'a' && c <= 'f') return c - 'a' + 10;
	if (c >= 'A' && c <= 'F') return c - 'A' + 10;
	return -1;
}

static int read_hex4(parser_t *ps, unsigned *out)
{
	if (ps->end - ps->p < 4)
		return perr(ps, "truncated \\u escape");
	unsigned v = 0;
	for (int i = 0; i < 4; i++) {
		int h = hexval(ps->p[i]);
		if (h < 0)
			return perr(ps, "bad \\u escape");
		v = v * 16 + (unsigned)h;
	}
	ps->p += 4;
	*out = v;
	return 0;
}

static void put_utf8(sb_t *sb, unsigned cp)
{
	char b[4];
	if (cp < 0x80) {
		b[0] = (char)cp;
		sb_add(sb, b, 1);
	} else if (cp < 0x800) {
		b[0] = (char)(0xC0 | (cp >> 6));
		b[1] = (char)(0x80 | (cp & 0x3F));
		sb_add(sb, b, 2);
	} else if (cp < 0x10000) {
		b[0] = (char)(0xE0 | (cp >> 12));
		b[1] = (char)(0x80 | ((cp >> 6) & 0x3F));
		b[2] = (char)(0x80 | (cp & 0x3F));
		sb_add(sb, b, 3);
	} else {
		b[0] = (char)(0xF0 | (cp >> 18));
		b[1] = (char)(0x80 | ((cp >> 12) & 0x3F));
		b[2] = (char)(0x80 | ((cp >> 6) & 0x3F));
		b[3] = (char)(0x80 | (cp & 0x3F));
		sb_add(sb, b, 4);
	}
}

static int parse_string(parser_t *ps, char **out)
{
	/* caller guarantees *ps->p == '"' */
	ps->p++;
	sb_t sb = {0};
	sb_add(&sb, "", 0);
	while (ps->p < ps->end) {
		char c = *ps->p++;
		if (c == '"') {
			*out = sb.p;
			return 0;
		}
		if ((unsigned char)c < 0x20) {
			sb_free(&sb);
			return perr(ps, "control character in string");
		}
		if (c != '\\') {
			sb_add(&sb, &c, 1);
			continue;
		}
		if (ps->p >= ps->end)
			break;
		char e = *ps->p++;
		switch (e) {
		case '"': sb_add(&sb, "\"", 1); break;
		case '\\': sb_add(&sb, "\\", 1); break;
		case '/': sb_add(&sb, "/", 1); break;
		case 'b': sb_add(&sb, "\b", 1); break;
		case 'f': sb_add(&sb, "\f", 1); break;
		case 'n': sb_add(&sb, "\n", 1); break;
		case 'r': sb_add(&sb, "\r", 1); break;
		case 't': sb_add(&sb, "\t", 1); break;
		case 'u': {
			unsigned cp;
			if (read_hex4(ps, &cp)) {
				sb_free(&sb);
				return -1;
			}
			if (cp >= 0xD800 && cp <= 0xDBFF && ps->end - ps->p >= 6 && ps->p[0] == '\\' && ps->p[1] == 'u') {
				unsigned lo;
				ps->p += 2;
				if (read_hex4(ps, &lo)) {
					sb_free(&sb);
					return -1;
				}
				if (lo >= 0xDC00 && lo <= 0xDFFF)
					cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
				else
					cp = 0xFFFD;
			} else if (cp >= 0xD800 && cp <= 0xDFFF) {
				cp = 0xFFFD;
			}
			put_utf8(&sb, cp);
			break;
		}
		default:
			sb_free(&sb);
			return perr(ps, "bad escape");
		}
	}
	sb_free(&sb);
	return perr(ps, "unterminated string");
}

static int parse_value(parser_t *ps, jval *out);

static int push_item(jval *v, size_t *cap, const jval *item, char *key)
{
	if (v->n == *cap) {
		size_t nc = *cap ? *cap * 2 : 8;
		jval *items = realloc(v->items, nc * sizeof *items);
		if (!items)
			return -1;
		v->items = items;
		if (v->type == J_OBJ) {
			char **keys = realloc(v->keys, nc * sizeof *keys);
			if (!keys)
				return -1;
			v->keys = keys;
		}
		*cap = nc;
	}
	v->items[v->n] = *item;
	if (v->type == J_OBJ)
		v->keys[v->n] = key;
	v->n++;
	return 0;
}

static int parse_container(parser_t *ps, jval *out, int obj)
{
	char close = obj ? '}' : ']';
	ps->p++;
	out->type = obj ? J_OBJ : J_ARR;
	size_t cap = 0;
	if (++ps->depth > JSON_MAX_DEPTH)
		return perr(ps, "nesting too deep");
	skip_ws(ps);
	if (ps->p < ps->end && *ps->p == close) {
		ps->p++;
		ps->depth--;
		return 0;
	}
	for (;;) {
		char *key = NULL;
		skip_ws(ps);
		if (obj) {
			if (ps->p >= ps->end || *ps->p != '"')
				return perr(ps, "expected object key");
			if (parse_string(ps, &key))
				return -1;
			skip_ws(ps);
			if (ps->p >= ps->end || *ps->p != ':') {
				free(key);
				return perr(ps, "expected ':'");
			}
			ps->p++;
		}
		jval item = {0};
		if (parse_value(ps, &item)) {
			free(key);
			free_inner(&item);
			return -1;
		}
		if (push_item(out, &cap, &item, key)) {
			free(key);
			free_inner(&item);
			return perr(ps, "out of memory");
		}
		skip_ws(ps);
		if (ps->p < ps->end && *ps->p == ',') {
			ps->p++;
			continue;
		}
		if (ps->p < ps->end && *ps->p == close) {
			ps->p++;
			ps->depth--;
			return 0;
		}
		return perr(ps, obj ? "expected ',' or '}'" : "expected ',' or ']'");
	}
}

static int match_lit(parser_t *ps, const char *lit)
{
	size_t n = strlen(lit);
	if ((size_t)(ps->end - ps->p) < n || memcmp(ps->p, lit, n) != 0)
		return 0;
	ps->p += n;
	return 1;
}

static int parse_number(parser_t *ps, jval *out)
{
	const char *s = ps->p;
	if (ps->p < ps->end && *ps->p == '-')
		ps->p++;
	int digits = 0;
	while (ps->p < ps->end && ((*ps->p >= '0' && *ps->p <= '9') || *ps->p == '.' || *ps->p == 'e' ||
				   *ps->p == 'E' || *ps->p == '+' || *ps->p == '-')) {
		if (*ps->p >= '0' && *ps->p <= '9')
			digits++;
		ps->p++;
	}
	if (!digits)
		return perr(ps, "unexpected character");
	char buf[64];
	size_t n = (size_t)(ps->p - s);
	if (n >= sizeof buf)
		return perr(ps, "number too long");
	memcpy(buf, s, n);
	buf[n] = 0;
	char *end = NULL;
	out->type = J_NUM;
	out->num = strtod(buf, &end);
	if (!end || *end)
		return perr(ps, "bad number");
	return 0;
}

static int parse_value(parser_t *ps, jval *out)
{
	skip_ws(ps);
	if (ps->p >= ps->end)
		return perr(ps, "unexpected end of input");
	switch (*ps->p) {
	case '{': return parse_container(ps, out, 1);
	case '[': return parse_container(ps, out, 0);
	case '"':
		out->type = J_STR;
		return parse_string(ps, &out->str);
	case 't':
		if (!match_lit(ps, "true"))
			return perr(ps, "bad literal");
		out->type = J_BOOL;
		out->boolean = 1;
		return 0;
	case 'f':
		if (!match_lit(ps, "false"))
			return perr(ps, "bad literal");
		out->type = J_BOOL;
		return 0;
	case 'n':
		if (!match_lit(ps, "null"))
			return perr(ps, "bad literal");
		out->type = J_NULL;
		return 0;
	default:
		return parse_number(ps, out);
	}
}

jval *json_parse(const char *text, size_t len, char *err, size_t errlen)
{
	parser_t ps = {text, text, text + len, err, errlen, 0};
	if (err && errlen)
		err[0] = 0;
	/* Tolerate a UTF-8 BOM: PowerShell writes one by default. */
	if (len >= 3 && (unsigned char)text[0] == 0xEF && (unsigned char)text[1] == 0xBB &&
	    (unsigned char)text[2] == 0xBF)
		ps.p += 3;
	jval *v = calloc(1, sizeof *v);
	if (!v)
		return NULL;
	if (parse_value(&ps, v)) {
		json_free(v);
		return NULL;
	}
	skip_ws(&ps);
	if (ps.p != ps.end) {
		perr(&ps, "trailing data");
		json_free(v);
		return NULL;
	}
	return v;
}

const jval *jget(const jval *obj, const char *key)
{
	if (!obj || obj->type != J_OBJ)
		return NULL;
	/* Last duplicate wins, like encoding/json. */
	for (size_t i = obj->n; i-- > 0;)
		if (strcmp(obj->keys[i], key) == 0)
			return &obj->items[i];
	return NULL;
}

const jval *jat(const jval *arr, size_t i)
{
	if (!arr || (arr->type != J_ARR && arr->type != J_OBJ) || i >= arr->n)
		return NULL;
	return &arr->items[i];
}

double jnum(const jval *v, double def) { return v && v->type == J_NUM ? v->num : def; }
const char *jstr(const jval *v, const char *def) { return v && v->type == J_STR ? v->str : def; }
int jbool(const jval *v, int def) { return v && v->type == J_BOOL ? v->boolean : def; }

const jval *jpath(const jval *v, const char *dotted)
{
	char key[128];
	const char *s = dotted;
	while (v && *s) {
		const char *dot = strchr(s, '.');
		size_t n = dot ? (size_t)(dot - s) : strlen(s);
		if (n >= sizeof key)
			return NULL;
		memcpy(key, s, n);
		key[n] = 0;
		v = jget(v, key);
		s += n + (dot ? 1 : 0);
	}
	return v;
}
