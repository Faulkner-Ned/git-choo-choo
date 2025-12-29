package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type CommitInfo struct {
	Hash          string
	Message       string
	Modifications string
}

// Precompiled regex
var (
	insertionsRegex = regexp.MustCompile(`(\d+)\s+insertions?\(\+\)`)
	deletionsRegex  = regexp.MustCompile(`(\d+)\s+deletions?\(-\)`)
)

// parseLineChanges converts Git shortstat lines to "+X -Y" format
func parseLineChanges(line string) string {
	if line == "" {
		return ""
	}

	parts := strings.Split(line, ",")
	result := strings.TrimSpace(parts[0])

	if match := insertionsRegex.FindStringSubmatch(line); len(match) == 2 {
		result += ", +" + match[1]
	}

	if match := deletionsRegex.FindStringSubmatch(line); len(match) == 2 {
		result += " -" + match[1]
	}

	return result
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get current branch: %v\n%s", err, output.String())
	}

	return strings.TrimSpace(output.String()), nil
}

func validateBranchAndRemote(branch, remote string) error {
	// Check local branch existence
	cmdLocal := exec.Command("git", "rev-parse", "--verify", branch)
	var outputLocal bytes.Buffer
	cmdLocal.Stdout = &outputLocal
	cmdLocal.Stderr = &outputLocal
	if err := cmdLocal.Run(); err != nil {
		return fmt.Errorf("local branch '%s' does not exist", branch)
	}

	// Check remote branch existence
	ref := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	cmdRemote := exec.Command("git", "show-ref", "--verify", "--quiet", ref)
	if err := cmdRemote.Run(); err != nil {
		return fmt.Errorf("remote branch '%s/%s' does not exist", remote, branch)
	}

	return nil
}

func getUnpushedCommitDetails(branch, remote string) ([]CommitInfo, error) {
	if err := validateBranchAndRemote(branch, remote); err != nil {
		return nil, fmt.Errorf("failed to validate branch and remote: %w", err)
	}

	cmd := exec.Command("git", "log", fmt.Sprintf("%s/%s..%s", remote, branch, branch),
		"--shortstat", "--pretty=format:---%n%H%n%s")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w\n%s", err, output.String())
	}

	gitOutput := strings.TrimSpace(output.String())
	if gitOutput == "" {
		return nil, nil // no unpushed commits
	}

	blocks := strings.Split(gitOutput, "---\n")
	commits := make([]CommitInfo, 0, len(blocks))

	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue // skip malformed entries
		}

		commit := CommitInfo{
			Hash:    strings.TrimSpace(lines[0]),
			Message: strings.TrimSpace(lines[1]),
		}

		if len(lines) > 2 {
			commit.Modifications = parseLineChanges(strings.TrimSpace(lines[2]))
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

func reverseCommitOrder(commits []CommitInfo) {
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
}

func getCommitInformation(branch, remote string) ([]CommitInfo, error) {
	commits, err := getUnpushedCommitDetails(branch, remote)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, fmt.Errorf("no unpushed commits found on branch '%s'", branch)
	}

	// Reverses the order of commit so earliest commit is the first entry in list
	// Makes sense as you build on earlier commit, so final commit would be the last attached to the train
	reverseCommitOrder(commits)
	return commits, nil
}

func pushToRemote(branch, remote string, force bool) error {
	args := []string{"push", remote, branch}

	if force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	fmt.Printf("Pushing to %s/%s...\n", remote, branch)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("push failed: %v\n%s", err, output.String())
	}

	fmt.Println("Push complete. Your code has left the station!")
	return nil
}
