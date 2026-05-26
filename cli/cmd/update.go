package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	addOperateCommand(updateCmd)
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
	cliPath := repoPath
	if _, err := os.Stat(filepath.Join(cliPath, "go.mod")); err != nil {
		cliPath = filepath.Join(repoPath, "cli")
	}
	if _, err := os.Stat(filepath.Join(cliPath, "go.mod")); err != nil {
		return fmt.Errorf("cannot find CLI module (go.mod) under %s. Provide --repo pointing to the eyeVesa repository", repoPath)
	}
	gitRoot, err := gitOutput(cliPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("cannot find git repository for %s: %w", cliPath, err)
	}
	repoPath = strings.TrimSpace(gitRoot)

	fmt.Printf("  repo: %s\n", repoPath)
	fmt.Printf("  cli:  %s\n", cliPath)

	if err := updateDefaultBranch(repoPath); err != nil {
		fmt.Printf("  git update failed: %v\n", err)
		fmt.Println("  continuing with local build...")
	}

	out, err := gitOutput(repoPath, "rev-parse", "--short", "HEAD")
	commit := strings.TrimSpace(out)
	if err != nil {
		commit = "unknown"
	}

	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get binary path: %w", err)
	}

	ldflags := fmt.Sprintf("-X github.com/hafizaljohari/eyeVesa/cli/cmd.version=%s", commit)

	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", bin, ".")
	build.Dir = cliPath
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	fmt.Println("  building...")
	if err := build.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("  ✓ eyevesa updated to %s\n", commit)
	return nil
}

func updateDefaultBranch(repoPath string) error {
	if err := gitRun(repoPath, "fetch", "--prune", "origin"); err != nil {
		return err
	}

	defaultBranch, err := detectDefaultBranch(repoPath)
	if err != nil {
		return err
	}

	status, err := gitOutput(repoPath, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("working tree has local changes; commit or stash before updating")
	}

	current, _ := gitOutput(repoPath, "branch", "--show-current")
	current = strings.TrimSpace(current)
	if current != defaultBranch {
		if _, err := gitOutput(repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+defaultBranch); err == nil {
			if err := gitRun(repoPath, "switch", defaultBranch); err != nil {
				return err
			}
		} else if err := gitRun(repoPath, "switch", "--create", defaultBranch, "--track", "origin/"+defaultBranch); err != nil {
			return err
		}
	}

	return gitRun(repoPath, "merge", "--ff-only", "origin/"+defaultBranch)
}

func detectDefaultBranch(repoPath string) (string, error) {
	headRef, err := gitOutput(repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		if branch := strings.TrimPrefix(strings.TrimSpace(headRef), "origin/"); branch != "" {
			return branch, nil
		}
	}
	for _, branch := range []string{"main", "master"} {
		if _, err := gitOutput(repoPath, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch); err == nil {
			return branch, nil
		}
	}
	return "", fmt.Errorf("could not determine origin default branch")
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
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
