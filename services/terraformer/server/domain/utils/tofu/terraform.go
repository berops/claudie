package tofu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

// maxTfCommandRetryCount is the maximum amount a Tofu command can be repeated until
// it succeeds. If after "maxTfCommandRetryCount" retries the commands still fails an error should be
// returned containing the reason.
const maxTfCommandRetryCount = 3

// Parallelism is the number of resource to be work on in parallel during the apply/destroy commands.
var parallelism = envs.GetOrDefaultInt("TERRAFORMER_TOFU_PARALLELISM", 40)

type Terraform struct {
	// Directory represents the directory of .tf files
	Directory string
	// CacheDir represents the directory for caching terraform plugins
	// It will be defined via env TF_PLUGIN_CACHE_DIR
	CacheDir string

	Stdout io.Writer
	Stderr io.Writer

	// Parallelism is the number of resources to be worked on in parallel by tofu.
	Parallelism int

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (t *Terraform) ProvidersLock() error {
	absCache, err := filepath.Abs(t.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute cache dir: %w", err)
	}

	if _, err := os.Stat(absCache); os.IsNotExist(err) {
		if err := fileutils.CreateDirectory(absCache); err != nil {
			return fmt.Errorf("failed to create cache directory %s : %w", absCache, err)
		}
	}

	args := []string{
		"providers",
		"lock",
		fmt.Sprintf("-fs-mirror=%v", absCache),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory

	stderrBuf := &bytes.Buffer{}
	cmd.Stderr = stderrBuf

	if err := cmd.Run(); err != nil {
		// In case that cache mirror does not contain the required providers
		// continue and fetch them from opentofu registry.
		if strings.Contains(stderrBuf.String(), "Could not retrieve providers for locking") {
			log.Info().Msg("OpenTofu failed to fetch the requested providers from mirror. Proceed to get providers from opentofu registry.")
		} else {
			return fmt.Errorf("failed to execute cmd: %s: %s", cmd, stderrBuf.String())
		}
	}
	return nil
}

func (t *Terraform) Init() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu init process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	absCache, err := filepath.Abs(t.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute cache dir: %w", err)
	}

	//nolint
	cmd := exec.Command("tofu", "init")
	cmd.Env = append(os.Environ(), "TF_PLUGIN_CACHE_DIR="+absCache)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "tofu init",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Apply() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu apply process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = parallelism
	}

	args := []string{
		"apply",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Destroy() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu destroy process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) DestroyTarget(targets []string) error {
	if len(targets) == 0 {
		return nil
	}

	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu destroy target process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	for _, resource := range targets {
		args = append(args, fmt.Sprintf("--target=%s", resource))
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		// NOTE: the maxTfCommandRetryCount * 2 is crucial here. Some resources may have a kind of
		// "lock" on a resource that cannot be immediately deleted and a timeout is needed, for example
		// this is the case with azures NIC which have a reservation for 180.
		if err := retryCmd.RetryCommand(maxTfCommandRetryCount * 2); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) StateList() ([]string, error) {
	//nolint
	cmd := exec.Command("tofu", "state", "list")
	cmd.Dir = t.Directory
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)
		retryCmd := comm.Cmd{
			Command: "tofu state list",
			Dir:     t.Directory,
		}

		out, err = retryCmd.RetryCommandWithOutput(maxTfCommandRetryCount)
		if err != nil {
			return nil, fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
		// fallthrough
	}

	r := bytes.Split(out, []byte("\n"))
	var resources []string
	for _, b := range r {
		if r := strings.TrimSpace(string(b)); r != "" {
			resources = append(resources, strings.TrimSpace(string(b)))
		}
	}

	return resources, nil
}

func (t *Terraform) Output(resourceName string) (string, error) {
	//nolint
	cmd := exec.Command("tofu", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)
		cmd := fmt.Sprintf("tofu output -json %s", resourceName)
		retryCmd := comm.Cmd{
			Command: cmd,
			Dir:     t.Directory,
		}

		out, err = retryCmd.RetryCommandWithOutput(maxTfCommandRetryCount)
		if err != nil {
			return "", fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
		// fallthrough
	}
	return string(out), nil
}
