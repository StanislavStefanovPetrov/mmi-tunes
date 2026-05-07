package url

import "fmt"

// Canonical returns the canonical YouTube watch URL for a given input.
// All inputs that point to the same video produce the same output:
//
//	https://www.youtube.com/watch?v=<videoID>
//
// Returns an error if the input is not a recognizable YouTube URL.
func Canonical(raw string) (string, error) {
	id, err := ExtractVideoID(raw)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", id), nil
}
