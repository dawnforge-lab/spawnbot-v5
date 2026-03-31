package struggles

import (
	"bufio"
	"encoding/json"
	"os"
)

// MaxLogBytes is the maximum bytes to read from the struggle log (100KB).
const MaxLogBytes = 100 * 1024

// ReadLog reads all signals from the struggle log file.
// Returns empty slice if file does not exist.
func ReadLog(path string) ([]Signal, error) {
	return ReadLogCapped(path, 0)
}

// ReadLogCapped reads signals from the log, only reading the last capBytes of the file.
// If capBytes is 0, reads the entire file.
func ReadLogCapped(path string, capBytes int64) ([]Signal, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	if capBytes > 0 {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if info.Size() > capBytes {
			if _, err := f.Seek(-capBytes, 2); err != nil {
				return nil, err
			}
			// Skip partial first line after seeking
			scanner := bufio.NewScanner(f)
			if scanner.Scan() {
				// discard partial line
			}
			return scanSignals(scanner)
		}
	}

	scanner := bufio.NewScanner(f)
	return scanSignals(scanner)
}

// ReadLogContent reads the raw content of the log file as a string.
// Returns empty string if file does not exist.
// Caps at capBytes if the file is larger.
func ReadLogContent(path string, capBytes int64) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if capBytes > 0 && int64(len(data)) > capBytes {
		data = data[int64(len(data))-capBytes:]
	}
	return string(data), nil
}

// TruncateLog clears the struggle log file.
func TruncateLog(path string) error {
	return os.WriteFile(path, nil, 0o644)
}

func scanSignals(scanner *bufio.Scanner) ([]Signal, error) {
	var signals []Signal
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sig Signal
		if err := json.Unmarshal(line, &sig); err != nil {
			continue // skip malformed lines
		}
		signals = append(signals, sig)
	}
	return signals, scanner.Err()
}
