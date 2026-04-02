#include <ctype.h>
#include <string.h>

#include "util.h"

char *trim(char *s) {
    while (*s && isspace((unsigned char)*s)) {
        s++;
    }

    if (*s == '\0') {
        return s;
    }

    char *end = s + strlen(s) - 1;
    while (end > s && isspace((unsigned char)*end)) {
        *end-- = '\0';
    }

    return s;
}

int is_ignorable_input(const char *s) {
    if (s == NULL || *s == '\0') {
        return 1;
    }

    for (int i = 0; s[i]; i++) {
        unsigned char c = (unsigned char)s[i];

        if (c == '\033') {
            return 1;
        }

        if (iscntrl(c) && !isspace(c)) {
            return 1;
        }
    }

    return 0;
}
