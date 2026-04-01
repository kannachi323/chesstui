#ifndef UTIL_H
#define UTIL_H

#include <stddef.h>

char *trim(char *s);
int is_ignorable_input(const char *s);
void normalize_command(char *dst, size_t dstsz, const char *src);

#endif
