package prompt

import (
	"os/exec"
	"strings"
)

// GetGitStatus 获取 Git 状态信息
func GetGitStatus(workingDir string) *GitStatusInfo {
	// 检查是否是 git 仓库
	if !IsGitRepo(workingDir) {
		return nil
	}

	status := &GitStatusInfo{}

	// 获取当前分支
	if branch, err := runGitCommand(workingDir, "branch", "--show-current"); err == nil {
		status.Branch = strings.TrimSpace(branch)
	}

	// 获取主分支
	status.MainBranch = detectMainBranch(workingDir)

	// 获取状态
	if output, err := runGitCommand(workingDir, "status", "--porcelain"); err == nil {
		status.IsClean = len(strings.TrimSpace(output)) == 0
		parseGitStatus(output, status)
	}

	// 获取最近提交
	if output, err := runGitCommand(workingDir, "log", "--oneline", "-5"); err == nil {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line != "" {
				status.RecentCommits = append(status.RecentCommits, line)
			}
		}
	}

	return status
}

// IsGitRepo 检查目录是否是 git 仓库
func IsGitRepo(workingDir string) bool {
	_, err := runGitCommand(workingDir, "rev-parse", "--git-dir")
	return err == nil
}

// runGitCommand 运行 git 命令
func runGitCommand(workingDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workingDir
	output, err := cmd.Output()
	return string(output), err
}

// detectMainBranch 检测主分支名称
func detectMainBranch(workingDir string) string {
	// 尝试常见的主分支名称
	candidates := []string{"main", "master"}

	for _, branch := range candidates {
		if _, err := runGitCommand(workingDir, "rev-parse", "--verify", branch); err == nil {
			return branch
		}
	}

	return "main"
}

// parseGitStatus 解析 git status --porcelain 输出
func parseGitStatus(output string, status *GitStatusInfo) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		x := line[0]
		y := line[1]
		file := strings.TrimSpace(line[3:])

		// 未跟踪文件
		if x == '?' && y == '?' {
			status.Untracked = append(status.Untracked, file)
			continue
		}

		// 已暂存
		if x != ' ' && x != '?' {
			status.Staged = append(status.Staged, file)
		}

		// 已修改但未暂存
		if y != ' ' && y != '?' {
			status.Unstaged = append(status.Unstaged, file)
		}
	}
}
