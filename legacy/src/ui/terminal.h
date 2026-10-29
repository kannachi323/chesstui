#pragma once

#define TERMINAL_KEY_UNKNOWN  0
#define TERMINAL_KEY_ENTER    1
#define TERMINAL_KEY_BACKSPACE 2
#define TERMINAL_KEY_LEFT     3
#define TERMINAL_KEY_RIGHT    4
#define TERMINAL_KEY_ESCAPE   5
#define TERMINAL_KEY_UP       6
#define TERMINAL_KEY_DOWN     7

void terminal_setup(void);
void terminal_restore(void);
int terminal_enable_raw_mode(void);
void terminal_disable_raw_mode(void);
int terminal_read_key(void);
void terminal_sleep_ms(int ms);
