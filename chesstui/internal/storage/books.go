//go:build !js

package storage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Store) SaveBook(ctx context.Context, book Book) error {
	marks := make([]string, len(book.Bookmarks))
	for i, mark := range book.Bookmarks {
		marks[i] = strconv.Itoa(mark)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO books(id, title, path, position, bookmarks, opened_at) VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET title=excluded.title, path=excluded.path, position=excluded.position,
    bookmarks=excluded.bookmarks, opened_at=excluded.opened_at`,
		book.ID, book.Title, book.Path, book.Position, strings.Join(marks, ","), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save book: %w", err)
	}
	return nil
}

// Books returns the library, most recently opened first.
func (s *Store) Books(ctx context.Context) ([]Book, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, title, path, position, bookmarks, opened_at FROM books ORDER BY opened_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()
	var books []Book
	for rows.Next() {
		var book Book
		var marks, opened string
		if err := rows.Scan(&book.ID, &book.Title, &book.Path, &book.Position, &marks, &opened); err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}
		for _, mark := range strings.Split(marks, ",") {
			if n, err := strconv.Atoi(mark); err == nil {
				book.Bookmarks = append(book.Bookmarks, n)
			}
		}
		sort.Ints(book.Bookmarks)
		book.OpenedAt, _ = time.Parse(time.RFC3339, opened)
		books = append(books, book)
	}
	return books, rows.Err()
}

func (s *Store) DeleteBook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM books WHERE id = ?", id)
	return err
}
