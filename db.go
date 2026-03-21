package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
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

	// upsert: if term already exists, append the new sentence and reset next_review_date
	_, err := db.Exec(`
		INSERT INTO problem_words (term, problem_sentence, created_at, next_review_date)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(term) DO UPDATE SET
			problem_sentence = excluded.problem_sentence,
			next_review_date = excluded.next_review_date,
			interval = 1
	`, term, problemSentence, now, nextReview)
	return err
}

func getDueTerms(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT term FROM problem_words
		WHERE next_review_date <= ?
		ORDER BY next_review_date ASC
		LIMIT 5
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var terms []string
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

func recordResult(db *sql.DB, term string, passed bool) error {
	var intervalMultiplier int
	if passed {
		intervalMultiplier = 2
	} else {
		intervalMultiplier = 1
	}

	_, err := db.Exec(`
		UPDATE problem_words SET
			times_seen = times_seen + 1,
			times_correct = times_correct + CASE WHEN ? THEN 1 ELSE 0 END,
			interval = CASE WHEN ? THEN interval * ? ELSE 1 END,
			next_review_date = datetime('now', '+' || (CASE WHEN ? THEN interval * ? ELSE 1 END) || ' days')
		WHERE term = ?
	`, passed, passed, intervalMultiplier, passed, intervalMultiplier, term)
	return err
}
