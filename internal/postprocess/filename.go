// Package postprocess contains MP3 post-processing — filename sanitization,
// cover-art resizing, and (optionally) M3U playlist generation.
package postprocess

import (
	"strings"
	"unicode"
)

// MaxFilenameRunes is the upper bound on output filename length, in runes,
// before the extension is appended. 80 chars matches yt-dlp's "%(title).80s"
// default and stays well under FAT32's 255-byte limit even with multi-byte
// characters that survive transliteration.
const MaxFilenameRunes = 80

// cyrillicMap holds a Bulgarian/Russian Latin transliteration. We bias
// Bulgarian conventions for ambiguous letters (ъ→a, я→ya).
var cyrillicMap = map[rune]string{
	'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D", 'Е': "E", 'Ж': "Zh",
	'З': "Z", 'И': "I", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M", 'Н': "N",
	'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U", 'Ф': "F",
	'Х': "H", 'Ц': "Ts", 'Ч': "Ch", 'Ш': "Sh", 'Щ': "Sht", 'Ъ': "A",
	'Ь': "", 'Ю': "Yu", 'Я': "Ya",
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sht", 'ъ': "a",
	'ь': "", 'ю': "yu", 'я': "ya",
	// Russian-specific
	'Ё': "Yo", 'ё': "yo", 'Ы': "Y", 'ы': "y", 'Э': "E", 'э': "e",
	// Ukrainian
	'Є': "Ye", 'є': "ye", 'І': "I", 'і': "i", 'Ї': "Yi", 'ї': "yi", 'Ґ': "G", 'ґ': "g",
}

// Forbidden on FAT32 (and most filesystems) plus chars that make filenames
// unfriendly across Finder/Terminal/Audi MMI.
var forbiddenChars = map[rune]bool{
	'\\': true, '/': true, ':': true, '*': true, '?': true, '"': true,
	'<': true, '>': true, '|': true,
}

// SanitizeFilename converts a raw video title into a FAT32-safe filename
// suitable for Audi MMI:
//
//   - Cyrillic letters are transliterated to Latin
//   - Forbidden chars (\ / : * ? " < > |) are replaced with "-"
//   - Control chars are dropped
//   - Trailing dots/spaces are trimmed (FAT32 quirk)
//   - Whitespace is collapsed to single spaces
//   - Output is truncated to MaxFilenameRunes runes
//   - If the result is empty, fallback is returned (typically the video ID)
//
// The returned string contains no extension; callers append ".mp3".
func SanitizeFilename(title, fallback string) string {
	var b strings.Builder
	b.Grow(len(title))

	for _, r := range title {
		if mapped, ok := cyrillicMap[r]; ok {
			b.WriteString(mapped)
			continue
		}
		if forbiddenChars[r] {
			b.WriteRune('-')
			continue
		}
		// Whitespace control chars (tab, newline, CR, etc.) become spaces
		// so that strings.Fields can collapse them below. Other control
		// chars are dropped entirely.
		if unicode.IsControl(r) {
			if unicode.IsSpace(r) {
				b.WriteRune(' ')
			}
			continue
		}
		// Keep ASCII letters/digits/punctuation that are FAT32-safe;
		// drop everything else (CJK, Arabic, emoji, etc).
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		// Lossless fallback: drop. If everything was non-ASCII and not
		// in our transliteration map, the final result will be empty
		// and we'll use `fallback`.
	}

	out := b.String()

	// Collapse runs of whitespace to single spaces.
	out = strings.Join(strings.Fields(out), " ")

	// Trim FAT32-unfriendly trailing dots/spaces.
	out = strings.TrimRight(out, ". ")
	out = strings.TrimLeft(out, ". ")

	// Truncate to MaxFilenameRunes runes (not bytes).
	if runes := []rune(out); len(runes) > MaxFilenameRunes {
		out = strings.TrimRight(string(runes[:MaxFilenameRunes]), ". ")
	}

	if out == "" {
		return fallback
	}
	return out
}
