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
    language         TEXT NOT NULL DEFAULT '',
    term             TEXT NOT NULL,
    context          TEXT NOT NULL,
    source           TEXT NOT NULL DEFAULT 'mistake',
    created_at       DATETIME NOT NULL,
    next_review_date DATETIME NOT NULL,
    interval         INTEGER NOT NULL DEFAULT 1,
    times_seen       INTEGER NOT NULL DEFAULT 0,
    times_correct    INTEGER NOT NULL DEFAULT 0,
    UNIQUE(language, term)
);

CREATE TABLE IF NOT EXISTS curriculum_progress (
    language     TEXT NOT NULL DEFAULT '',
    week         INTEGER NOT NULL,
    status       TEXT NOT NULL DEFAULT 'in_progress',
    started_at   DATETIME NOT NULL,
    completed_at DATETIME,
    PRIMARY KEY (language, week)
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

	// migrate old problem_words table if it exists
	var oldTable string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='problem_words'`).Scan(&oldTable)
	if err == nil {
		if _, err := db.Exec(migration); err != nil {
			log.Printf("warning: migration from problem_words failed: %v", err)
		}
	}

	return db
}

func storeMistake(db *sql.DB, lang, term, problemSentence string) error {
	now := time.Now()
	nextReview := now

	// upsert: append the new context and reset review schedule (they got it wrong again)
	_, err := db.Exec(`
		INSERT INTO words (language, term, context, source, created_at, next_review_date)
		VALUES (?, ?, ?, 'mistake', ?, ?)
		ON CONFLICT(language, term) DO UPDATE SET
			context = context || char(10) || excluded.context,
			next_review_date = excluded.next_review_date,
			interval = 1
	`, lang, term, problemSentence, now, nextReview)
	return err
}

func storeTaughtTerms(db *sql.DB, lang string, words []TaughtTerm) error {
	now := time.Now()
	for _, w := range words {
		// insert only if not already tracked — don't reset progress on existing terms
		_, err := db.Exec(`
			INSERT INTO words (language, term, context, source, created_at, next_review_date)
			VALUES (?, ?, ?, 'taught', ?, ?)
			ON CONFLICT(language, term) DO NOTHING
		`, lang, w.Term, w.Definition, now, now)
		if err != nil {
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

func getDueTerms(db *sql.DB, lang string) ([]DueTerm, error) {
	rows, err := db.Query(`
		SELECT term, context, source FROM words
		WHERE language = ? AND next_review_date <= ?
		ORDER BY next_review_date ASC
		LIMIT 5
	`, lang, time.Now())
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

func recordResult(db *sql.DB, lang, term string, passed bool) error {
	// Check if the term already exists (case-insensitive).
	var currentInterval int
	var dbTerm string
	err := db.QueryRow(`SELECT term, interval FROM words WHERE language = ? AND term = ? COLLATE NOCASE`, lang, term).Scan(&dbTerm, &currentInterval)

	if err == sql.ErrNoRows {
		// Term not in DB yet — create it from the quiz result.
		newInterval := 1
		if passed {
			newInterval = 2
		}
		_, err := db.Exec(`
			INSERT INTO words (language, term, context, source, created_at, next_review_date, interval, times_seen, times_correct)
			VALUES (?, ?, 'quizzed', 'taught', datetime('now'), datetime('now', '+' || ? || ' days'), ?, 1, ?)
		`, lang, term, newInterval, newInterval, boolToInt(passed))
		return err
	}
	if err != nil {
		return err
	}

	// Term exists — update it normally.
	term = dbTerm // use the exact DB spelling
	var newInterval int
	var correctInc int
	if passed {
		newInterval = currentInterval * 2
		correctInc = 1
	} else {
		newInterval = 1
		correctInc = 0
	}

	_, err = db.Exec(`
		UPDATE words SET
			times_seen = times_seen + 1,
			times_correct = times_correct + ?,
			interval = ?,
			next_review_date = datetime('now', '+' || ? || ' days')
		WHERE language = ? AND term = ?
	`, correctInc, newInterval, newInterval, lang, term)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// getCurrentWeek returns the current in-progress week, or 0 if none started.
func getCurrentWeek(db *sql.DB, lang string) int {
	var week int
	err := db.QueryRow(`
		SELECT week FROM curriculum_progress
		WHERE language = ? AND status = 'in_progress'
		ORDER BY week DESC LIMIT 1
	`, lang).Scan(&week)
	if err != nil {
		return 0
	}
	return week
}

// startWeek marks a week as in-progress.
func startWeek(db *sql.DB, lang string, week int) error {
	_, err := db.Exec(`
		INSERT INTO curriculum_progress (language, week, status, started_at)
		VALUES (?, ?, 'in_progress', datetime('now'))
		ON CONFLICT(language, week) DO NOTHING
	`, lang, week)
	return err
}

// completeWeek marks the current week as completed and starts the next one.
func completeWeek(db *sql.DB, lang string, week int) error {
	_, err := db.Exec(`
		UPDATE curriculum_progress
		SET status = 'completed', completed_at = datetime('now')
		WHERE language = ? AND week = ?
	`, lang, week)
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

func loadCurriculumNote(lang string, week int) string {
	path := filepath.Join(notesDir(), "curriculum", lang, fmt.Sprintf("week_%d.txt", week))
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func appendCurriculumNote(lang string, week int, note string) error {
	dir := filepath.Join(notesDir(), "curriculum", lang)
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

type WeekProgress struct {
	TotalTerms   int `json:"total_terms"`
	StoredTerms  int `json:"stored_terms"`
	LearnedTerms int `json:"learned_terms"` // interval >= 4 (passed at least twice)
}

// getWeekProgress checks how many of the given vocab terms are in the DB and how many are "learned".
func getWeekProgress(db *sql.DB, lang string, vocabTerms []string) WeekProgress {
	progress := WeekProgress{TotalTerms: len(vocabTerms)}
	for _, term := range vocabTerms {
		var interval int
		err := db.QueryRow(`SELECT interval FROM words WHERE language = ? AND term = ?`, lang, term).Scan(&interval)
		if err != nil {
			continue // not in DB yet
		}
		progress.StoredTerms++
		if interval >= 4 {
			progress.LearnedTerms++
		}
	}
	return progress
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
