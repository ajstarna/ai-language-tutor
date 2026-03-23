package main

import (
	"bufio"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS problem_words (
    id               INTEGER PRIMARY KEY,
    term             TEXT NOT NULL UNIQUE,
    problem_sentence TEXT NOT NULL,
    created_at       DATETIME NOT NULL,
    next_review_date DATETIME NOT NULL,
    interval         INTEGER NOT NULL DEFAULT 1,
    times_seen       INTEGER NOT NULL DEFAULT 0,
    times_correct    INTEGER NOT NULL DEFAULT 0
);`

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

	return db
}

func storeTerm(db *sql.DB, term, problemSentence string) error {
	now := time.Now()
	nextReview := now  // immediately available for review

	// upsert: if term already exists, append the new sentence (separated by newline) and reset review schedule
	_, err := db.Exec(`
		INSERT INTO problem_words (term, problem_sentence, created_at, next_review_date)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(term) DO UPDATE SET
			problem_sentence = problem_sentence || char(10) || excluded.problem_sentence,
			next_review_date = excluded.next_review_date,
			interval = 1
	`, term, problemSentence, now, nextReview)
	return err
}

type DueTerm struct {
	Term            string `json:"term"`
	ProblemSentence string `json:"problem_sentence"`
}

func getDueTerms(db *sql.DB) ([]DueTerm, error) {
	rows, err := db.Query(`
		SELECT term, problem_sentence FROM problem_words
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
		if err := rows.Scan(&t.Term, &t.ProblemSentence); err != nil {
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
	if err := db.QueryRow(`SELECT interval FROM problem_words WHERE term = ?`, term).Scan(&currentInterval); err != nil {
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
		UPDATE problem_words SET
			times_seen = times_seen + 1,
			times_correct = times_correct + ?,
			interval = ?,
			next_review_date = datetime('now', '+' || ? || ' days')
		WHERE term = ?
	`, correctInc, newInterval, newInterval, term)
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
