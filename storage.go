package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS words (
    id               INTEGER PRIMARY KEY,
    term             TEXT NOT NULL UNIQUE,
    context          TEXT NOT NULL,
    source           TEXT NOT NULL DEFAULT 'mistake',
    created_at       DATETIME NOT NULL,
    next_review_date DATETIME NOT NULL,
    interval         INTEGER NOT NULL DEFAULT 1,
    times_seen       INTEGER NOT NULL DEFAULT 0,
    times_correct    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS curriculum_progress (
    week         INTEGER PRIMARY KEY,
    status       TEXT NOT NULL DEFAULT 'in_progress',
    started_at   DATETIME NOT NULL,
    completed_at DATETIME
);`

// migrate old problem_words table to new words table if needed
const migration = `
INSERT OR IGNORE INTO words (id, term, context, source, created_at, next_review_date, interval, times_seen, times_correct)
SELECT id, term, problem_sentence, 'mistake', created_at, next_review_date, interval, times_seen, times_correct
FROM problem_words;
DROP TABLE IF EXISTS problem_words;`

func openDB() *sql.DB {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(homeDir, ".goodfriendstutor", "words.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	// migrate old table if it exists
	var oldTable string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='problem_words'`).Scan(&oldTable)
	if err == nil {
		if _, err := db.Exec(migration); err != nil {
			log.Printf("warning: migration from problem_words failed: %v", err)
		}
	}

	return db
}

func storeTerm(db *sql.DB, term, context, source string) error {
	now := time.Now()
	nextReview := now // immediately available for review

	// upsert: if term already exists, append the new context (separated by newline) and reset review schedule
	_, err := db.Exec(`
		INSERT INTO words (term, context, source, created_at, next_review_date)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(term) DO UPDATE SET
			context = context || char(10) || excluded.context,
			next_review_date = excluded.next_review_date,
			interval = 1
	`, term, context, source, now, nextReview)
	return err
}

func storeTaughtTerms(db *sql.DB, words []TaughtTerm) error {
	for _, w := range words {
		if err := storeTerm(db, w.Term, w.Definition, "taught"); err != nil {
			return err
		}
	}
	return nil
}

type DueTerm struct {
	Term    string `json:"term"`
	Context string `json:"context"`
	Source  string `json:"source"`
}

func getDueTerms(db *sql.DB) ([]DueTerm, error) {
	rows, err := db.Query(`
		SELECT term, context, source FROM words
		WHERE next_review_date <= ?
		ORDER BY next_review_date ASC
		LIMIT 5
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var terms []DueTerm
	for rows.Next() {
		var t DueTerm
		if err := rows.Scan(&t.Term, &t.Context, &t.Source); err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	return terms, nil
}

func recordResult(db *sql.DB, term string, passed bool) error {
	// Read current interval first, then compute the new one in Go
	// to avoid SQL evaluation order issues between interval and next_review_date.
	var currentInterval int
	if err := db.QueryRow(`SELECT interval FROM words WHERE term = ?`, term).Scan(&currentInterval); err != nil {
		return err
	}

	var newInterval int
	var correctInc int
	if passed {
		newInterval = currentInterval * 2
		correctInc = 1
	} else {
		newInterval = 1
		correctInc = 0
	}

	_, err := db.Exec(`
		UPDATE words SET
			times_seen = times_seen + 1,
			times_correct = times_correct + ?,
			interval = ?,
			next_review_date = datetime('now', '+' || ? || ' days')
		WHERE term = ?
	`, correctInc, newInterval, newInterval, term)
	return err
}

// getCurrentWeek returns the current in-progress week, or 0 if none started.
func getCurrentWeek(db *sql.DB) int {
	var week int
	err := db.QueryRow(`
		SELECT week FROM curriculum_progress
		WHERE status = 'in_progress'
		ORDER BY week DESC LIMIT 1
	`).Scan(&week)
	if err != nil {
		return 0
	}
	return week
}

// startWeek marks a week as in-progress.
func startWeek(db *sql.DB, week int) error {
	_, err := db.Exec(`
		INSERT INTO curriculum_progress (week, status, started_at)
		VALUES (?, 'in_progress', datetime('now'))
		ON CONFLICT(week) DO NOTHING
	`, week)
	return err
}

// completeWeek marks the current week as completed and starts the next one.
func completeWeek(db *sql.DB, week int) error {
	_, err := db.Exec(`
		UPDATE curriculum_progress
		SET status = 'completed', completed_at = datetime('now')
		WHERE week = ?
	`, week)
	return err
}

func dataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(homeDir, ".goodfriendstutor")
}

func notesDir() string {
	return filepath.Join(dataDir(), "notes")
}

func loadGeneralNotes() string {
	path := filepath.Join(notesDir(), "general", "session_notes.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// cap to last 20 lines to avoid context bloat
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.Join(lines, "\n")
}

func appendGeneralNote(note string) error {
	dir := filepath.Join(notesDir(), "general")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "session_notes.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(note + "\n")
	return err
}

func loadCurriculumNote(week int) string {
	path := filepath.Join(notesDir(), "curriculum", fmt.Sprintf("week_%d.txt", week))
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func appendCurriculumNote(week int, note string) error {
	dir := filepath.Join(notesDir(), "curriculum")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("week_%d.txt", week))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(note + "\n")
	return err
}

func profilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(homeDir, ".goodfriendstutor", "profile.txt")
}

func loadUserFacts() []string {
	f, err := os.Open(profilePath())
	if err != nil {
		return nil // file doesn't exist yet, that's fine
	}
	defer f.Close()

	var facts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			facts = append(facts, line)
		}
	}
	return facts
}

func appendUserFact(fact string) error {
	f, err := os.OpenFile(profilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fact + "\n")
	return err
}
