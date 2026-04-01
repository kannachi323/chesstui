#define _POSIX_C_SOURCE 200809L

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#include "terminal.h"

static int g_alt_screen = 0;
static int g_terminal_active = 0;

static void disable_mouse_tracking(void) {
    if (!isatty(STDOUT_FILENO)) {
        return;
    }

    printf("\033[?1000l\033[?1002l\033[?1003l\033[?1006l\033[?1007l\033[?1015l");
    fflush(stdout);
}

void terminal_restore(void) {
    if (!g_terminal_active) {
        return;
    }

    disable_mouse_tracking();

    if (g_alt_screen) {
        printf("\033[?1049l");
        fflush(stdout);
        g_alt_screen = 0;
    }
}

static void signal_handler(int sig) {
    terminal_restore();
    signal(sig, SIG_DFL);
    raise(sig);
}

void terminal_setup(void) {
    if (!isatty(STDOUT_FILENO)) {
        return;
    }

    g_terminal_active = 1;
    disable_mouse_tracking();
    printf("\033[?1049h");
    fflush(stdout);
    g_alt_screen = 1;

    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    signal(SIGHUP, signal_handler);
}
