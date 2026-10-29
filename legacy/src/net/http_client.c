#include <curl/curl.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "http_client.h"

typedef struct {
    char  *data;
    size_t len;
} WriteBuffer;

static size_t write_cb(void *contents, size_t size, size_t nmemb, void *userp) {
    size_t total = size * nmemb;
    WriteBuffer *buf = (WriteBuffer *)userp;
    char *tmp = realloc(buf->data, buf->len + total + 1);
    if (!tmp) return 0;
    buf->data = tmp;
    memcpy(buf->data + buf->len, contents, total);
    buf->len += total;
    buf->data[buf->len] = '\0';
    return total;
}

void http_response_free(HttpResponse *r) {
    free(r->body);
    r->body = NULL;
    r->body_len = 0;
    r->status_code = 0;
}

HttpResponse http_get(const char *url) {
    HttpResponse resp = {0, NULL, 0};
    WriteBuffer buf = {NULL, 0};

    CURL *curl = curl_easy_init();
    if (!curl) return resp;

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, 10L);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "chess-cli/1.0");

    CURLcode res = curl_easy_perform(curl);
    if (res == CURLE_OK) {
        long code;
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &code);
        resp.status_code = (int)code;
        resp.body     = buf.data;
        resp.body_len = buf.len;
    } else {
        free(buf.data);
    }

    curl_easy_cleanup(curl);
    return resp;
}

HttpResponse http_get_bearer(const char *url, const char *token) {
    HttpResponse resp = {0, NULL, 0};
    WriteBuffer buf = {NULL, 0};

    CURL *curl = curl_easy_init();
    if (!curl) return resp;

    char auth_header[256];
    snprintf(auth_header, sizeof(auth_header), "Authorization: Bearer %s", token);
    struct curl_slist *headers = curl_slist_append(NULL, auth_header);

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, 10L);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "chess-cli/1.0");

    CURLcode res = curl_easy_perform(curl);
    if (res == CURLE_OK) {
        long code;
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &code);
        resp.status_code = (int)code;
        resp.body     = buf.data;
        resp.body_len = buf.len;
    } else {
        free(buf.data);
    }

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    return resp;
}

HttpResponse http_post_bearer(const char *url, const char *token, const char *body) {
    HttpResponse resp = {0, NULL, 0};
    WriteBuffer buf = {NULL, 0};

    CURL *curl = curl_easy_init();
    if (!curl) return resp;

    char auth_header[256];
    snprintf(auth_header, sizeof(auth_header), "Authorization: Bearer %s", token);
    struct curl_slist *headers = curl_slist_append(NULL, auth_header);
    headers = curl_slist_append(headers, "Content-Type: application/x-www-form-urlencoded");

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, 10L);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "chess-cli/1.0");

    CURLcode res = curl_easy_perform(curl);
    if (res == CURLE_OK) {
        long code;
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &code);
        resp.status_code = (int)code;
        resp.body     = buf.data;
        resp.body_len = buf.len;
    } else {
        free(buf.data);
    }

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    return resp;
}

/* ---- Streaming NDJSON -------------------------------------------------- */

static size_t stream_write_cb(void *ptr, size_t size, size_t nmemb, void *ud) {
    HttpStream *s = (HttpStream *)ud;
    size_t total = size * nmemb;
    size_t space = HTTP_STREAM_BUFSIZE - (size_t)s->len - 1;
    if (total > space) total = space;
    memcpy(s->buf + s->len, ptr, total);
    s->len += (int)total;
    s->buf[s->len] = '\0';
    return size * nmemb;
}

HttpStream *http_stream_open(const char *url, const char *token) {
    HttpStream *s = calloc(1, sizeof(*s));
    if (!s) return NULL;

    CURLM *multi = curl_multi_init();
    if (!multi) { free(s); return NULL; }

    CURL *easy = curl_easy_init();
    if (!easy) { curl_multi_cleanup(multi); free(s); return NULL; }

    char auth_header[256];
    snprintf(auth_header, sizeof(auth_header), "Authorization: Bearer %s", token);
    struct curl_slist *headers = curl_slist_append(NULL, auth_header);
    headers = curl_slist_append(headers, "Accept: application/x-ndjson");

    curl_easy_setopt(easy, CURLOPT_URL, url);
    curl_easy_setopt(easy, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(easy, CURLOPT_WRITEFUNCTION, stream_write_cb);
    curl_easy_setopt(easy, CURLOPT_WRITEDATA, s);
    curl_easy_setopt(easy, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(easy, CURLOPT_CONNECTTIMEOUT, 10L);
    curl_easy_setopt(easy, CURLOPT_USERAGENT, "chess-cli/1.0");

    curl_multi_add_handle(multi, easy);

    s->multi   = multi;
    s->easy    = easy;
    s->headers = headers;
    return s;
}

int http_stream_tick(HttpStream *s, int timeout_ms,
                     http_line_fn fn, void *ud, int *stdin_ready) {
    if (stdin_ready) *stdin_ready = 0;

    struct curl_waitfd extra = { STDIN_FILENO, CURL_WAIT_POLLIN, 0 };
    int numfds = 0;
    curl_multi_wait((CURLM *)s->multi, &extra, 1, timeout_ms, &numfds);

    if (stdin_ready && (extra.revents & CURL_WAIT_POLLIN))
        *stdin_ready = 1;

    int still_running = 0;
    curl_multi_perform((CURLM *)s->multi, &still_running);

    /* Dispatch complete lines */
    char *start = s->buf;
    char *nl;
    while ((nl = memchr(start, '\n', (size_t)(s->buf + s->len - start))) != NULL) {
        *nl = '\0';
        if (nl > start && *(nl - 1) == '\r') *(nl - 1) = '\0';
        if (*start && fn) fn(start, ud);
        start = nl + 1;
    }

    /* Shift remaining partial line to front */
    int remaining = (int)(s->buf + s->len - start);
    if (remaining > 0 && start != s->buf)
        memmove(s->buf, start, (size_t)remaining);
    s->len = remaining;
    s->buf[s->len] = '\0';

    return still_running;
}

void http_stream_close(HttpStream *s) {
    if (!s) return;
    curl_multi_remove_handle((CURLM *)s->multi, (CURL *)s->easy);
    curl_easy_cleanup((CURL *)s->easy);
    curl_slist_free_all((struct curl_slist *)s->headers);
    curl_multi_cleanup((CURLM *)s->multi);
    free(s);
}
