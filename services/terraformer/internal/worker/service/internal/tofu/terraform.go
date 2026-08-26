package tofu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
const maxTfCommandRetryCount = 1

// Parallelism is the number of resource to be work on in parallel during the apply/destroy commands.
var parallelism = envs.GetOrDefaultInt("TERRAFORMER_TOFU_PARALLELISM", 10)

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
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu providers lock process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	absCache, err := filepath.Abs(t.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute cache dir: %w", err)
	}

	if err := fileutils.CreateDirectory(absCache); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", absCache, err)
	}

	args := []string{
		"providers",
		"lock",
		fmt.Sprintf("-fs-mirror=%v", absCache),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory

	if err := cmd.Run(); err != nil {
		return err
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
	cmd := exec.Command("tofu", "init", "-upgrade")
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("TF_PLUGIN_CACHE_DIR=%s", absCache))

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "tofu init -upgrade",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
			Env:     []string{fmt.Sprintf("TF_PLUGIN_CACHE_DIR=%s", absCache)},
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

func (t *Terraform) OutputString(resourceName string) (string, error) {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return "", fmt.Errorf("failed to prepare tofu output process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

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

func (t *Terraform) OutputAll() (map[string]any, error) {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return nil, fmt.Errorf("failed to prepare tofu output process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	//nolint
	cmd := exec.Command("tofu", "output", "-json")
	cmd.Dir = t.Directory
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)
		retryCmd := comm.Cmd{
			Command: "tofu output -json",
			Dir:     t.Directory,
		}

		out, err = retryCmd.RetryCommandWithOutput(maxTfCommandRetryCount)
		if err != nil {
			return nil, fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
		// fallthrough
	}

	var outputs map[string]struct {
		Value any `json:"value"`
	}

	d := json.NewDecoder(bytes.NewReader(out))
	d.UseNumber()

	if err := d.Decode(&outputs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tofu outputs from %s: %w", t.Directory, err)
	}

	values := make(map[string]any, len(outputs))
	for name, output := range outputs {
		values[name] = output.Value
	}
	return values, nil
}
