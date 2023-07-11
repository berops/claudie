package utils

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
)

// collectErrors collects error messages from a json output of a single playbook run.
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
					errs = append(errs, fmt.Errorf("%s failed inside task %s due to: %s", hostName, taskName, msg))
				}
			}
		}
	}

	return errors.Join(errs...)
}
