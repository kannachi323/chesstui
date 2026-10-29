cmake_minimum_required(VERSION 3.15)
project(chess-cli C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

add_compile_options(-Wall -Wextra -pedantic)

# --- Dependencies ---
# These must be set before add_subdirectory so they pre-empt curl's option() calls
set(BUILD_CURL_EXE  OFF CACHE BOOL "" FORCE)
set(BUILD_TESTING   OFF CACHE BOOL "" FORCE)
set(HTTP_ONLY       ON  CACHE BOOL "" FORCE)
set(CURL_USE_LIBPSL OFF CACHE BOOL "" FORCE)
add_subdirectory(vendor)


# --- Sources ---
set(SOURCES
    src/main.c
    src/core/chess.c
    src/core/chess_stats.c
    src/ui/cli.c
    src/ui/terminal.c
    src/puzzle/puzzle.c
    src/util/util.c
    src/net/http_client.c
    src/net/lichess.c
    src/net/account.c
)

add_executable(chess-cli ${SOURCES})

target_include_directories(chess-cli PRIVATE
    src/core
    src/ui
    src/puzzle
    src/util
    src/net
)

target_link_libraries(chess-cli PRIVATE CURL::libcurl)

# --- Convenience targets ---
add_custom_target(fetch-puzzles
    COMMAND echo "Downloading full Lichess puzzle database..."
    COMMAND curl -L -o lichess_db_puzzle.csv.zst
        https://database.lichess.org/lichess_db_puzzle.csv.zst
    COMMAND zstd -d --rm lichess_db_puzzle.csv.zst -o puzzles.csv
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    VERBATIM
)

add_custom_target(split-puzzles
    COMMAND ${CMAKE_SOURCE_DIR}/scripts/split-puzzles.sh puzzles.csv packs
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    VERBATIM
)
