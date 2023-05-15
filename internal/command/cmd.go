package command

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	stdLog "log"

	"github.com/berops/claudie/internal/utils"
	"github.com/rs/zerolog/log"
)

type Cmd struct {
	Command        string
	Options        []string
	Dir            string
	Stdout         io.Writer
	Stderr         io.Writer
	CommandTimeout int
}

// Wrapper struct holds data for the wrapper around stdout & stderr.
type Wrapper struct {
	logger *stdLog.Logger
	buf    *bytes.Buffer
	prefix string
	// Adds color to stdout & stderr if terminal supports it.
	useColours bool
	logType    int
}

const (
	STDOUT     = 0
	STDERR     = 1
	colorOkay  = "\x1b[32m"
	colorFail  = "\x1b[31m"
	colorReset = "\x1b[0m"
	maxBackoff = 5 * 60 // max backoff time [5 min]
)

// RetryCommand retries the given command, with exponential backoff, maxing at 5 min, for numOfRetries times.
// Returns error if all retries fail, nil otherwise.
func (c *Cmd) RetryCommand(numOfRetries int) error {
	var err error

	// Have a cmd that is safe for printing.
	printSafeCmd := c.sanitisedCmd()

	for i := 1; i <= numOfRetries; i++ {
		backoff := getNewBackoff(i)
		log.Info().Msgf("Next retry in %ds...", backoff)
		time.Sleep(time.Duration(backoff) * time.Second)

		if err = c.execute(i, numOfRetries); err == nil {
			log.Info().Msgf("The %s was successful on %d retry", printSafeCmd, i)

			return nil
		}

		log.Warn().Msgf("Error encountered while executing %s : %v", printSafeCmd, err)
	}

	log.Error().Msgf("Command %s was not successful after %d retries", printSafeCmd, numOfRetries)

	return err
}

// RetryCommandWithOutput retries the given command, with exponential backoff, maxing at 5 min, for numOfRetries times.
// returns (nil, error) if all retries fail, (output, nil) otherwise.
func (c *Cmd) RetryCommandWithOutput(numOfRetries int) ([]byte, error) {
	var err error
	var out []byte

	// Have a cmd that is safe for printing.
	printSafeCmd := c.sanitisedCmd()

	for i := 1; i <= numOfRetries; i++ {
		backoff := getNewBackoff(i)
		log.Info().Msgf("Next retry in %ds...", backoff)
		time.Sleep(time.Duration(backoff) * time.Second)

		if out, err = c.executeWithOutput(i, numOfRetries); err == nil {
			log.Info().Msgf("The %s was successful after %d retry", printSafeCmd, i)

			return out, nil
		}

		log.Warn().Msgf("Error encountered while executing %s : %v", printSafeCmd, err)
	}

	log.Error().Msgf("Command %s was not successful after %d retries", printSafeCmd, numOfRetries)

	return out, err
}

// execute executes the cmd with context canceled after commandTimeout seconds.
// Returns error if unsuccessful, nil otherwise.
func (c *Cmd) execute(i, numOfRetries int) error {
	cmd, cancel := c.buildCmd()
	if cancel != nil {
		defer cancel()
	}

	// Have a cmd that is safe for printing.
	printSafeCmd := c.sanitisedCmd()

	log.Warn().Msgf("Retrying command %s... (%d/%d)", printSafeCmd, i, numOfRetries)

	return cmd.Run()
}

// executeWithOutput executes the cmd with context canceled after commandTimeout seconds.
// Returns error, nil if unsuccessful, nil, output otherwise.
func (c *Cmd) executeWithOutput(i, numOfRetries int) ([]byte, error) {
	cmd, cancel := c.buildCmd()
	if cancel != nil {
		defer cancel()
	}

	// Have a cmd that is safe for printing.
	printSafeCmd := c.sanitisedCmd()

	log.Warn().Msgf("Retrying command %s... (%d/%d)", printSafeCmd, i, numOfRetries)

	return cmd.CombinedOutput()
}

// buildCmd prepares a exec.Cmd datastructure with context.
func (c *Cmd) buildCmd() (*exec.Cmd, context.CancelFunc) {
	var cmd *exec.Cmd
	var cancelFun context.CancelFunc = nil
	if c.CommandTimeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.CommandTimeout)*time.Second)
		cmd = exec.CommandContext(ctx, "bash", "-c", strings.Join(append([]string{c.Command}, c.Options...), " "))
		cancelFun = cancel
	} else {
		cmd = exec.Command("bash", "-c", strings.Join(append([]string{c.Command}, c.Options...), " "))
	}
	cmd.Dir = c.Dir
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	return cmd, cancelFun
}

func (c *Cmd) sanitisedCmd() string {
	if c.Command == "" {
		return c.Command
	}

	// sanitise any kubeconfigs found.
	printSafeCmd := utils.SanitiseKubeconfig(c.Command)
	// sanitise any URIs with passwords.
	printSafeCmd = utils.SanitiseURI(printSafeCmd)

	return printSafeCmd
}

// getNewBackoff returns a new backoff 5 * (2 ^ iteration), with the hard limit set at maxBackoff.
func getNewBackoff(iteration int) int {
	backoff := 5 * (math.Pow(2, float64(iteration)))
	if backoff > maxBackoff {
		// set hard max for exponential backoff
		return maxBackoff
	}
	return int(backoff)
}

// GetStdOut returns an io.Writer for exec with the defined prefix.
// Cannot be used with cmd.CombinedOutput().
func GetStdOut(prefix string) Wrapper {
	return getWrapper(prefix, STDOUT)
}

// GetStdErr returns an io.Writer for exec with the defined prefix.
// Cannot be used with cmd.CombinedOutput().
func GetStdErr(prefix string) Wrapper {
	return getWrapper(prefix, STDERR)
}

func getWrapper(prefix string, logType int) Wrapper {
	w := Wrapper{logger: stdLog.New(os.Stderr, "", 0), buf: bytes.NewBuffer([]byte("")), prefix: prefix, logType: logType}

	//check if console supports colors, if so, set flag to true
	w.useColours = strings.HasPrefix(os.Getenv("TERM"), "xterm")

	return w
}

// Write is implementation of the function from io.Writer interface.
func (w Wrapper) Write(p []byte) (n int, err error) {
	if n, err = w.buf.Write(p); err != nil {
		return n, err
	}
	err = w.outputLines()
	return len(p), err
}

// outputLines will output strings from current buffer.
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

// printWithPrefix will append a colour if supported and outputs the line with prefix.
func (w *Wrapper) printWithPrefix(str string) {
	if len(str) < 1 {
		return
	}
	if w.useColours {
		if w.logType == STDOUT {
			str = colorOkay + w.prefix + "\t" + colorReset + " " + str
		} else {
			str = colorFail + w.prefix + "\t" + colorReset + " " + str
		}
	} else {
		str = w.prefix + "\t" + str
	}
	w.logger.Print(str)
}
