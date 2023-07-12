package utils

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
)

// collectErrors collects error messages from a single json output that's dumped by an ansible playbook run.
func collectErrors(reader io.Reader) error {
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, reader); err != nil {
		return err
	}

	if !gjson.ValidBytes(b.Bytes()) {
		return errors.New("failed to parse json from input")
	}

	result := gjson.ParseBytes(b.Bytes())
	plays := result.Get("plays")
	if !plays.IsArray() {
		return errors.New("unexpected input, \"plays\" is not an array")
	}

	var errs []error
	for _, play := range plays.Array() {
		result := play.Get("tasks")
		if !result.IsArray() {
			return errors.New("unexpected input,\"tasks\" is not an array")
		}

		for _, val := range result.Array() {
			taskName := val.Get("task.name")
			for hostName, host := range val.Get("hosts").Map() {
				if host.Get("failed").Bool() {
					msg := host.Get("msg")
					// formatted as:
					//        hetzner-50m17fe-1: failed
					//        task: Wait 300 seconds for target connection to become reachable/usable
					//        summary: timed out waiting for ping module test: Failed to connect to the host via ssh: ssh: connect to host 116.203.141.58 port 22: Operation timed out
					errs = append(errs, fmt.Errorf("\n\t%s: failed\n\ttask: %s\n\tsummary: %s", hostName, taskName, msg))
				}
			}
		}
	}

	return errors.Join(errs...)
}
