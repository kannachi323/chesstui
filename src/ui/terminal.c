#define _POSIX_C_SOURCE 200809L

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#include "terminal.h"

static int g_terminal_active = 0;

static void signal_handler(int sig) {
    terminal_restore();
    signal(sig, SIG_DFL);
    raise(sig);
}

void terminal_restore(void) {
    if (!g_terminal_active) {
        return;
    }
}

void terminal_setup(void) {
    if (!isatty(STDOUT_FILENO)) {
        return;
    }

    g_terminal_active = 1;

    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    signal(SIGHUP, signal_handler);
}
