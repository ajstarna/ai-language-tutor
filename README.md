# Good Friends Tutor

A terminal-based AI language tutor. It uses an LLM to teach vocabulary, correct mistakes, and quiz you using spaced repetition.

## Features

- **Curriculum mode** — follows a structured weekly syllabus (currently German and French, A1→B1)
- **Conversational mode** — free-form chat with corrections
- **Vocabulary tracking** — terms you learn and mistakes you make are stored in a local SQLite database and reviewed via spaced repetition
- **Quizzes** — the tutor quizzes you on terms that are due for review
- **Topic lessons** — ask the tutor to teach you vocab for any topic
- **Notes** — the tutor keeps notes on your progress across sessions

## Setup

You need an [OpenRouter](https://openrouter.ai/) API key:

```
export OPENROUTER_API_KEY=your-key-here
```

## Usage

```
go run .
```

On first run it will ask you to pick a target language, tutor language, mode, and strictness level. Config is saved to `~/.goodfriendstutor/config.json`.

### Commands

| Command | Description |
|---------|-------------|
| `/quiz` | Start a vocabulary quiz |
| `/endquiz` | Exit quiz mode early |
| `/topic [theme]` | Learn vocab for a topic |
| `/week` | Show current curriculum week |
| `/model` | Switch the AI model |
| `/config` | Change a setting |
| `/help` | Show available commands |
| `/quit` | Exit |

## Data

Everything is stored locally in `~/.goodfriendstutor/`:

- `config.json` — your settings
- `words.db` — vocabulary and progress (SQLite)
- `profile.txt` — personal facts the tutor remembers about you
- `notes/` — session notes and per-week curriculum notes
