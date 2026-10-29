#pragma once

#include <stddef.h>

typedef struct {
    int    status_code;
    char  *body;       /* heap-allocated; caller must pass to http_response_free() */
    size_t body_len;
} HttpResponse;

void         http_response_free(HttpResponse *r);
HttpResponse http_get(const char *url);
HttpResponse http_get_bearer(const char *url, const char *token);
HttpResponse http_post_bearer(const char *url, const char *token, const char *body);

/* Streaming NDJSON -------------------------------------------------- */

#define HTTP_STREAM_BUFSIZE 32768

typedef struct {
    void  *multi;   /* CURLM* */
    void  *easy;    /* CURL*  */
    void  *headers; /* curl_slist* */
    char   buf[HTTP_STREAM_BUFSIZE];
    int    len;
} HttpStream;

typedef void (*http_line_fn)(const char *line, void *userdata);

/* Open a long-lived Bearer-authenticated GET stream. Returns NULL on error. */
HttpStream *http_stream_open(const char *url, const char *token);

/* Wait up to timeout_ms for data or stdin activity.
 * Calls fn(line, ud) for each complete NDJSON line received.
 * Sets *stdin_ready=1 if stdin had data.
 * Returns 1 if stream still active, 0 if the server closed it. */
int  http_stream_tick(HttpStream *s, int timeout_ms,
                      http_line_fn fn, void *ud, int *stdin_ready);

void http_stream_close(HttpStream *s);
