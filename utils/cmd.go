package utils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	stdLog "log"

	"github.com/rs/zerolog/log"
)

type Cmd struct {
	Command string
	Dir     string
	Stdout  io.Writer
	Stderr  io.Writer
}

// Wrapper struct holds data for the wrapper around stdout & stderr
type Wrapper struct {
	logger *stdLog.Logger
	buf    *bytes.Buffer
	prefix string
	// Adds color to stdout & stderr if terminal supports it
	useColours bool
	logType    int
}

const (
	STDOUT     = 0
	STDERR     = 1
	colorOkay  = "\x1b[32m"
	colorFail  = "\x1b[31m"
	colorReset = "\x1b[0m"
)

// function RetryCommand will retry the given command, every 5 sec until either successful or reached numOfRetries
// returns error if all retries fail, nil otherwise
func (c Cmd) RetryCommand(numOfRetries int) error {
	var err error
	for i := 1; i <= numOfRetries; i++ {
		log.Warn().Msgf("Retrying command %s... (%d/%d)", c.Command, i, numOfRetries)
		cmd := exec.Command("bash", "-c", c.Command)
		cmd.Dir = c.Dir
		cmd.Stdout = c.Stdout
		cmd.Stderr = c.Stderr
		err = cmd.Run()
		if err == nil {
			log.Info().Msgf("The %s was successful on %d retry", c.Command, i)
			return nil
		}
		log.Warn().Msgf("Error encountered while executing %s : %v", c.Command, err)
		backoff := 5 * i
		log.Info().Msgf("Next retry in %ds...", backoff)
		time.Sleep(time.Duration(backoff) * time.Second)
	}
	log.Error().Msgf("Command %s was not successful after %d retries", c.Command, numOfRetries)
	return err
}

// function RetryCommandWithOutput will retry the given command, every 5 sec until either successful or reached numOfRetries
// returns (nil, error) if all retries fail, (output, nil) otherwise
func (c Cmd) RetryCommandWithOutput(numOfRetries int) ([]byte, error) {
	var err error
	for i := 1; i <= numOfRetries; i++ {
		log.Warn().Msgf("Retrying command %s... (%d/%d)", c.Command, i, numOfRetries)
		cmd := exec.Command("bash", "-c", c.Command)
		cmd.Dir = c.Dir
		cmd.Stdout = c.Stdout
		cmd.Stderr = c.Stderr
		out, err := cmd.CombinedOutput()
		if err == nil {
			log.Info().Msgf("The %s was successful after %d retry", c.Command, i)
			return out, nil
		}
		log.Warn().Msgf("Error encountered while executing %s : %v", c.Command, err)
		backoff := 5 * i
		log.Info().Msgf("Next retry in %ds...", backoff)
		time.Sleep(time.Duration(backoff) * time.Second)
	}
	log.Error().Msgf("Command %s was not successful after %d retries", c.Command, numOfRetries)
	return nil, err
}

// function GetStdOut returns an io.Writer for exec with the defined prefix
// Cannot be used with cmd.CombinedOutput()
func GetStdOut(prefix string) Wrapper {
	return getWrapper(prefix, STDOUT)
}

// function GetStdErr returns an io.Writer for exec with the defined prefix
// Cannot be used with cmd.CombinedOutput()
func GetStdErr(prefix string) Wrapper {
	return getWrapper(prefix, STDERR)
}

func getWrapper(prefix string, logType int) Wrapper {
	w := Wrapper{logger: stdLog.New(os.Stderr, "", 0), buf: bytes.NewBuffer([]byte("")), prefix: prefix, logType: logType}

	//check if console supports colors, if so, set flag to true
	w.useColours = strings.HasPrefix(os.Getenv("TERM"), "xterm")

	return w
}

// function Write is implementation of the function from io.Writer interface
func (w Wrapper) Write(p []byte) (n int, err error) {
	if n, err = w.buf.Write(p); err != nil {
		return n, err
	}
	err = w.outputLines()
	return len(p), err
}

// function outputLines will output strings from current buffer
func (w *Wrapper) outputLines() error {
	for {
		line, err := w.buf.ReadString('\n')
		//if EOF, break while loop
		if err == io.EOF {
			break
		}
		//if other err, break and return err
		if err != nil {
			return err
		}
		// print line
		if len(line) > 0 {
			//if whole line, print out with wrapper prefix
			if strings.HasSuffix(line, "\n") {
				w.printWithPrefix(line)
			} else {
				// if no new line, just append to the current buffer
				if _, err := w.buf.WriteString(line); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// function printWithPrefix will append a colour if supported and outputs the line with prefix
func (w *Wrapper) printWithPrefix(str string) {
	if len(str) < 1 {
		return
	}
	if w.useColours {
		if w.logType == STDOUT {
			str = colorOkay + w.prefix + colorReset + " " + str
		} else {
			str = colorFail + w.prefix + colorReset + " " + str
		}
	} else {
		str = w.prefix + str
	}
	w.logger.Print(str)
}
