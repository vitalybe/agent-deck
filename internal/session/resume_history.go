package session

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// ReadTranscriptMeta reads the head of a Claude transcript (.jsonl) and returns
// the session ID and working directory recorded inside it.
//
// The encoded directory name under ~/.claude/projects is lossy (slashes and
// dots both collapse to "-", so it cannot be reversed unambiguously), but every
// turn in the transcript records the exact absolute "cwd". Reading it from the
// body is therefore the only reliable way to map a transcript back to a folder.
func ReadTranscriptMeta(filePath string) (sessionID, cwd string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(io.LimitReader(f, 256*1024))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record claudeJSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if sessionID == "" && record.SessionID != "" {
			sessionID = record.SessionID
		}
		if cwd == "" && record.CWD != "" {
			cwd = record.CWD
		}
		if sessionID != "" && cwd != "" {
			break
		}
	}
	return sessionID, cwd, scanner.Err()
}
