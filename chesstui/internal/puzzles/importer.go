package puzzles

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/klauspost/compress/zstd"
)

const importBatchSize = 1000

type BatchStore interface {
	UpsertPuzzles(context.Context, []domain.Puzzle) error
}

type Progress struct {
	Imported int
	Skipped  int
}

func ImportFile(ctx context.Context, store BatchStore, path string, notify func(Progress)) (Progress, error) {
	file, err := os.Open(path)
	if err != nil {
		return Progress{}, fmt.Errorf("open puzzle pack: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	var decoder *zstd.Decoder
	if strings.HasSuffix(strings.ToLower(path), ".zst") {
		decoder, err = zstd.NewReader(file, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return Progress{}, fmt.Errorf("open zstd puzzle pack: %w", err)
		}
		defer decoder.Close()
		reader = decoder
	}

	csvReader := csv.NewReader(reader)
	csvReader.ReuseRecord = true
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	source := filepath.Base(path)
	progress := Progress{}
	batch := make([]domain.Puzzle, 0, importBatchSize)

	for {
		if err := ctx.Err(); err != nil {
			return progress, err
		}
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return progress, fmt.Errorf("read puzzle CSV near row %d: %w", progress.Imported+progress.Skipped+1, err)
		}
		puzzle, ok := parseRecord(record, source)
		if !ok {
			progress.Skipped++
			continue
		}
		batch = append(batch, puzzle)
		if len(batch) == cap(batch) {
			if err := store.UpsertPuzzles(ctx, batch); err != nil {
				return progress, err
			}
			progress.Imported += len(batch)
			batch = batch[:0]
			if notify != nil {
				notify(progress)
			}
		}
	}
	if len(batch) > 0 {
		if err := store.UpsertPuzzles(ctx, batch); err != nil {
			return progress, err
		}
		progress.Imported += len(batch)
	}
	if notify != nil {
		notify(progress)
	}
	return progress, nil
}

func parseRecord(record []string, source string) (domain.Puzzle, bool) {
	if len(record) < 4 || strings.EqualFold(strings.TrimSpace(record[0]), "PuzzleId") {
		return domain.Puzzle{}, false
	}
	rating, err := strconv.Atoi(strings.TrimSpace(record[3]))
	if err != nil {
		return domain.Puzzle{}, false
	}
	themes := ""
	if len(record) > 7 {
		themes = record[7]
	} else if len(record) > 4 {
		themes = record[4]
	}
	puzzle := domain.Puzzle{
		ID:     strings.TrimSpace(record[0]),
		FEN:    strings.TrimSpace(record[1]),
		Moves:  strings.Fields(record[2]),
		Rating: rating,
		Themes: strings.Fields(themes),
		Source: source,
	}
	return puzzle, puzzle.ID != "" && puzzle.FEN != "" && len(puzzle.Moves) > 0
}
