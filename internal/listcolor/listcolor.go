package listcolor

import (
	"database/sql"
)

// Preset color palette — name to tailwind-compatible classes.
// Order defines the cycle order.
var Colors = []string{
	"",       // no color (default)
	"red",
	"orange",
	"amber",
	"green",
	"teal",
	"blue",
	"purple",
	"pink",
}

// Store manages list color preferences in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns the color for a specific list, or "" if not set.
func (s *Store) Get(email, listID string) string {
	var color string
	err := s.db.QueryRow(
		"SELECT color FROM list_colors WHERE user_email = ? AND list_id = ?",
		email, listID,
	).Scan(&color)
	if err != nil {
		return ""
	}
	return color
}

// GetAll returns a map of list_id -> color for a user.
func (s *Store) GetAll(email string) map[string]string {
	rows, err := s.db.Query(
		"SELECT list_id, color FROM list_colors WHERE user_email = ?", email,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var listID, color string
		if err := rows.Scan(&listID, &color); err == nil {
			m[listID] = color
		}
	}
	return m
}

// Set stores a color for a list.
func (s *Store) Set(email, listID, color string) error {
	if color == "" {
		_, err := s.db.Exec(
			"DELETE FROM list_colors WHERE user_email = ? AND list_id = ?",
			email, listID,
		)
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO list_colors (user_email, list_id, color) VALUES (?, ?, ?)
		 ON CONFLICT (user_email, list_id) DO UPDATE SET color = excluded.color`,
		email, listID, color,
	)
	return err
}

// CycleNext returns the next color in the palette.
func CycleNext(current string) string {
	for i, c := range Colors {
		if c == current {
			return Colors[(i+1)%len(Colors)]
		}
	}
	return Colors[1] // first real color if current not found
}

// DotClass returns the Tailwind bg class for a color dot.
func DotClass(color string) string {
	switch color {
	case "red":
		return "bg-red-500"
	case "orange":
		return "bg-orange-500"
	case "amber":
		return "bg-amber-500"
	case "green":
		return "bg-green-500"
	case "teal":
		return "bg-teal-500"
	case "blue":
		return "bg-blue-500"
	case "purple":
		return "bg-purple-500"
	case "pink":
		return "bg-pink-500"
	default:
		return "bg-gray-300 dark:bg-gray-600"
	}
}

// BadgeClasses returns Tailwind classes for the list name badge in task view.
func BadgeClasses(color string) string {
	switch color {
	case "red":
		return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
	case "orange":
		return "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400"
	case "amber":
		return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
	case "green":
		return "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
	case "teal":
		return "bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400"
	case "blue":
		return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
	case "purple":
		return "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
	case "pink":
		return "bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400"
	default:
		return "bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400"
	}
}
