package kubeone

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"strings"
)

type (
	JSONLog struct {
		Level   string `json:"level"`
		Message string `json:"msg"`
		Time    string `json:"time"`
	}

	JSONLogs []JSONLog
)

const (
	LogLevelInfo  = "info"
	LogLevelWarn  = "warning"
	LogLevelError = "error"
)

func (l JSONLogs) prettyPrint() string {
	b := new(strings.Builder)
	for _, v := range l {
		b.WriteString("\n")
		b.WriteString(v.prettyPrint())
	}
	return b.String()
}

func (l JSONLog) prettyPrint() string {
	b := new(strings.Builder)

	// formated as:
	// 	severity: error
	// 	time: 2023-07-12T09:43:45+02:00
	// 	summary: ssh: installing kubeadm\nssh: remote command exited without exit status or exit...
	if l.Level == LogLevelError {
		b.WriteString(fmt.Sprintf("severity: %s\n", l.Level))
		b.WriteString(fmt.Sprintf("time: %s\n", l.Time))
		b.WriteString(fmt.Sprintf("summary: %s\n", l.Message))
	}

	return b.String()
}

func parseJSONLog(line []byte) (JSONLog, error) {
	s := JSONLog{}
	if err := json.Unmarshal(line, &s); err != nil {
		return JSONLog{}, err
	}
	return s, nil
}

// collectErrors collects error messages from a continuous stream of logs.
// it will only extract message that are valid json and contain an error.
func collectErrors(reader io.Reader) (JSONLogs, error) {
	s := bufio.NewScanner(reader)
	s.Split(bufio.ScanLines)

	var logs []JSONLog
	for s.Scan() {
		// the output may contain logs that are not JSON.
		txt := s.Text()
		if !gjson.Valid(txt) {
			continue
		}

		l, err := parseJSONLog([]byte(txt))
		if err != nil {
			return nil, err
		}

		if l.Level == LogLevelError {
			logs = append(logs, l)
		}
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse error logs: %w", err)
	}

	return logs, nil
}
