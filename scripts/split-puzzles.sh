#!/bin/bash
# Split the full Lichess puzzle CSV into rating-based packs.
# Usage: ./split-puzzles.sh [input.csv] [output-dir]

set -e

INPUT="${1:-puzzles.csv}"
OUTDIR="${2:-packs}"

if [ ! -f "$INPUT" ]; then
    echo "File not found: $INPUT"
    echo "Usage: $0 <puzzles.csv> [output-dir]"
    exit 1
fi

mkdir -p "$OUTDIR"

HEADER=$(head -1 "$INPUT")

echo "Splitting $(( $(wc -l < "$INPUT") - 1 )) puzzles by rating..."

tail -n +2 "$INPUT" | awk -F',' -v outdir="$OUTDIR" -v header="$HEADER" '
BEGIN {
    bands[1]  = "0-1000"
    bands[2]  = "1000-1500"
    bands[3]  = "1500-2000"
    bands[4]  = "2000-2500"
    bands[5]  = "2500-plus"

    for (i = 1; i <= 5; i++) {
        f = outdir "/puzzles-" bands[i] ".csv"
        print header > f
        count[i] = 0
    }
}
{
    rating = $4 + 0
    if      (rating < 1000) idx = 1
    else if (rating < 1500) idx = 2
    else if (rating < 2000) idx = 3
    else if (rating < 2500) idx = 4
    else                    idx = 5

    f = outdir "/puzzles-" bands[idx] ".csv"
    print >> f
    count[idx]++
}
END {
    for (i = 1; i <= 5; i++)
        printf "  puzzles-%s.csv: %d puzzles\n", bands[i], count[i]
}'

echo ""
echo "Compressing packs..."
for csv in "$OUTDIR"/puzzles-*.csv; do
    zstd --rm -q "$csv"
    echo "  $(basename "$csv" .csv).csv.zst"
done

echo ""
echo "Done. Packs in $OUTDIR/"
ls -lh "$OUTDIR"
