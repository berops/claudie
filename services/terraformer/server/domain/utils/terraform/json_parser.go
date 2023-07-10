package terraform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type (
	// Taken from https://github.com/hashicorp/terraform/blob/2622e89cfb7eb62045fbb35956345eb4ad169704/internal/command/views/json/diagnostic.go#L34
	Diagnostic struct {
		Severity string `json:"severity"`
		Summary  string `json:"summary"`
		Detail   string `json:"detail"`
		Address  string `json:"address,omitempty"`
	}

	// Taken from https://github.com/hashicorp/terraform/blob/2622e89cfb7eb62045fbb35956345eb4ad169704/internal/command/jsonformat/renderer.go#L25
	JSONLogType string

	JSONLog struct {
		Level      string      `json:"@level"`
		Message    string      `json:"@message"`
		Type       JSONLogType `json:"type"`
		Diagnostic *Diagnostic `json:"diagnostic"`
	}

	JSONLogs []JSONLog
)

const (
	LogApplyComplete     JSONLogType = "apply_complete"
	LogApplyErrored      JSONLogType = "apply_errored"
	LogApplyStart        JSONLogType = "apply_start"
	LogChangeSummary     JSONLogType = "change_summary"
	LogDiagnostic        JSONLogType = "diagnostic"
	LogPlannedChange     JSONLogType = "planned_change"
	LogProvisionComplete JSONLogType = "provision_complete"
	LogProvisionErrored  JSONLogType = "provision_errored"
	LogProvisionProgress JSONLogType = "provision_progress"
	LogProvisionStart    JSONLogType = "provision_start"
	LogOutputs           JSONLogType = "outputs"
	LogRefreshComplete   JSONLogType = "refresh_complete"
	LogRefreshStart      JSONLogType = "refresh_start"
	LogResourceDrift     JSONLogType = "resource_drift"
	LogVersion           JSONLogType = "version"
)

const (
	DiagnosticSeverityUnknown = "unknown"
	DiagnosticSeverityError   = "error"
	DiagnosticSeverityWarning = "warning"
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
	if l.Type == LogApplyErrored {
		b.WriteString(l.Message)
	}
	// formated as:
	// 	hcloud_server.test-jxt0chy-hetzner-asg2dpg-1: Creation errored after 0s
	// 	hcloud_server.test-jxt0chy-hetzner-c05ep09-1: Creation errored after 1s
	// 	diagnostic:
	// 		severity: error
	// 		summary: server type cpx89 not found
	// 		address hcloud_server.test-jxt0chy-hetzner-asg2dpg-1
	//
	// 	diagnostic:
	// 		severity: error
	// 		summary: server type cpx89 not found
	// 		address hcloud_server.test-jxt0chy-hetzner-c05ep09-1
	if l.Type == LogDiagnostic {
		if l.Diagnostic != nil {
			b.WriteString("diagnostic:\n")
			b.WriteString(fmt.Sprintf("\tseverity: %s\n", l.Diagnostic.Severity))
			b.WriteString(fmt.Sprintf("\tsummary: %s\n", l.Diagnostic.Summary))
			if l.Diagnostic.Detail != "" {
				b.WriteString(fmt.Sprintf("\tdetail: %s\n", l.Diagnostic.Detail))
			}
			if l.Diagnostic.Address != "" {
				b.WriteString(fmt.Sprintf("\taddress: %s\n", l.Diagnostic.Address))
			}
		} else {
			b.WriteString(l.Message)
		}
	}

	return b.String()
}

// parseJSONLog parses a single line into the JSONLog struct.
func parseJSONLog(line []byte) (JSONLog, error) {
	s := JSONLog{}
	if err := json.Unmarshal(line, &s); err != nil {
		return JSONLog{}, err
	}

	return s, nil
}

func collectErrors(reader io.Reader) (JSONLogs, error) {
	s := bufio.NewScanner(reader)
	s.Split(bufio.ScanLines)

	var logs []JSONLog
	for s.Scan() {
		l, err := parseJSONLog([]byte(s.Text()))
		if err != nil {
			return nil, err
		}

		isErrLog := l.Type == LogApplyErrored
		isErrLog = isErrLog || ((l.Type == LogDiagnostic) && (l.Level == DiagnosticSeverityError))

		if isErrLog {
			logs = append(logs, l)
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
