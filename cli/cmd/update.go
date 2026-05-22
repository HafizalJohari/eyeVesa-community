package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var updateRepo string

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update eyevesa to the latest version",
	Long: `Pull the latest source and rebuild the eyevesa binary.

By default, it finds the repo by walking up from the binary's location
looking for the go.mod file. Use --repo to point to a specific clone.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&updateRepo, "repo", "r", "", "path to eyeVesa repo (auto-detected from binary)")
}

func runUpdate() error {
	repoPath := updateRepo
	if repoPath == "" {
		bin, err := os.Executable()
		if err == nil {
			repoPath = findRepoRoot(filepath.Dir(bin))
		}
	}
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err == nil {
			repoPath = findRepoRoot(cwd)
		}
	}
	if repoPath == "" {
		for _, guess := range []string{
			filepath.Join(os.Getenv("HOME"), "eyeVesa"),
			filepath.Join(os.Getenv("HOME"), "src", "eyeVesa"),
			filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "hafizaljohari", "eyeVesa"),
		} {
			if r := findRepoRoot(guess); r != "" {
				repoPath = r
				break
			}
		}
	}
	if repoPath == "" {
		return fmt.Errorf("cannot find eyeVesa repo root. Clone it and use --repo /path/to/eyeVesa")
	}

	repoPath, _ = filepath.Abs(repoPath)
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err != nil {
		return fmt.Errorf("no go.mod found at %s. Provide the correct repo path with --repo", repoPath)
	}

	fmt.Printf("  repo: %s\n", repoPath)

	pullCmd := exec.Command("git", "pull", "--ff-only")
	pullCmd.Dir = repoPath
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		fmt.Println("  git pull failed, continuing with local build...")
	}

	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	commit := strings.TrimSpace(string(out))
	if err != nil {
		commit = "unknown"
	}

	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get binary path: %w", err)
	}

	ldflags := fmt.Sprintf("-X github.com/HafizalJohari/eyeVesa-community/cli/cmd.version=%s", commit)
	if runtime.GOOS == "windows" {
		ldflags = fmt.Sprintf("-X github.com/HafizalJohari/eyeVesa-community/cli/cmd.version=%s", commit)
	}

	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", bin, ".")
	build.Dir = repoPath
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	fmt.Println("  building...")
	if err := build.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("  ✓ eyevesa updated to %s\n", commit)
	return nil
}

func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
