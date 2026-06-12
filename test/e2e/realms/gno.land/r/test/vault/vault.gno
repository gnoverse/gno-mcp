package vault

import "strconv"

var notes = map[address]string{}

// Set stores a note for a user.
func Set(cur realm, user address, note string) {
	notes[user] = note
}

// Clear removes a user's note. Only the note's owner may clear it.
func Clear(cur realm, user address) {
	if cur.Previous().Address() != user {
		panic("not your note")
	}
	delete(notes, user)
}

// Get returns the note stored for a user, or "" if none.
func Get(user address) string {
	return notes[user]
}

// Render returns a markdown summary of how many notes are stored.
func Render(path string) string {
	return "# Vault\n\nnotes stored: " + strconv.Itoa(len(notes))
}
