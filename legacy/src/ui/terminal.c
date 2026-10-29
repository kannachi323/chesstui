#define _POSIX_C_SOURCE 200809L

#include <errno.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <termios.h>
#include <time.h>
#include <unistd.h>

#include "terminal.h"

static int g_terminal_active = 0;
static int g_raw_mode_enabled = 0;
static int g_has_original_termios = 0;
static struct termios g_original_termios;

static void signal_handler(int sig) {
    terminal_restore();
    signal(sig, SIG_DFL);
    raise(sig);
}

void terminal_restore(void) {
    terminal_disable_raw_mode();

    if (!g_terminal_active) {
        return;
    }
}

void terminal_setup(void) {
    if (!isatty(STDIN_FILENO) || !isatty(STDOUT_FILENO)) {
        return;
    }

    g_terminal_active = 1;
    if (tcgetattr(STDIN_FILENO, &g_original_termios) == 0) {
        g_has_original_termios = 1;
    }

    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    signal(SIGHUP, signal_handler);
}

int terminal_enable_raw_mode(void) {
    struct termios raw;

    if (g_raw_mode_enabled) {
        return 1;
    }
    if (!g_terminal_active || !g_has_original_termios) {
        return 0;
    }

    raw = g_original_termios;
    raw.c_lflag &= (tcflag_t) ~(ECHO | ICANON);
    raw.c_iflag &= (tcflag_t) ~(IXON | ICRNL);
    raw.c_cc[VMIN] = 1;
    raw.c_cc[VTIME] = 0;

    if (tcsetattr(STDIN_FILENO, TCSAFLUSH, &raw) != 0) {
        return 0;
    }

    g_raw_mode_enabled = 1;
    return 1;
}

void terminal_disable_raw_mode(void) {
    if (!g_raw_mode_enabled || !g_has_original_termios) {
        return;
    }

    tcsetattr(STDIN_FILENO, TCSAFLUSH, &g_original_termios);
    g_raw_mode_enabled = 0;
}

int terminal_read_key(void) {
    unsigned char c;
    unsigned char seq[2];
    ssize_t nread;

    nread = read(STDIN_FILENO, &c, 1);
    if (nread <= 0) {
        return TERMINAL_KEY_UNKNOWN;
    }

    if (c == '\r' || c == '\n') {
        return TERMINAL_KEY_ENTER;
    }
    if (c == 127 || c == '\b') {
        return TERMINAL_KEY_BACKSPACE;
    }
    if (c != '\033') {
        return c;
    }

    if (read(STDIN_FILENO, &seq[0], 1) != 1) {
        return TERMINAL_KEY_ESCAPE;
    }
    if (read(STDIN_FILENO, &seq[1], 1) != 1) {
        return TERMINAL_KEY_ESCAPE;
    }

    if (seq[0] == '[') {
        if (seq[1] == 'D') return TERMINAL_KEY_LEFT;
        if (seq[1] == 'C') return TERMINAL_KEY_RIGHT;
        if (seq[1] == 'A') return TERMINAL_KEY_UP;
        if (seq[1] == 'B') return TERMINAL_KEY_DOWN;
    }

    return TERMINAL_KEY_UNKNOWN;
}

void terminal_sleep_ms(int ms) {
    struct timespec req;
    struct timespec rem;

    if (ms <= 0) {
        return;
    }

    req.tv_sec = ms / 1000;
    req.tv_nsec = (long) (ms % 1000) * 1000000L;

    while (nanosleep(&req, &rem) != 0) {
        if (errno != EINTR) {
            break;
        }
        req = rem;
    }
}
