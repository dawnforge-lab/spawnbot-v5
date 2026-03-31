package struggles

import (
	"bufio"
	"encoding/json"
	"os"
)

// ReadLog reads all signals from the struggle log file.
// Returns empty slice if file does not exist.
func ReadLog(path string) ([]Signal, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var signals []Signal
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sig Signal
		if err := json.Unmarshal(line, &sig); err != nil {
			continue
		}
		signals = append(signals, sig)
	}
	return signals, scanner.Err()
}
