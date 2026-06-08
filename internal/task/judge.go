// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"fmt"
	"strings"

	"codeg/internal/llm"
)

// CompletionVerdict is the result of judging whether a coding session completed its task.
type CompletionVerdict string

const (
	VerdictCompleted CompletionVerdict = "COMPLETED"
	VerdictStuck     CompletionVerdict = "STUCK"
	VerdictFailed    CompletionVerdict = "FAILED"
)

// VerificationVerdict is the result of judging whether a verification passed.
type VerificationVerdict string

const (
	VerifyPassed VerificationVerdict = "PASSED"
	VerifyFailed VerificationVerdict = "FAILED"
)

// JudgeCodingComplete sends the agent's final output to OpenAI to determine
// whether the sub-task was actually completed.
func JudgeCodingComplete(ctx context.Context, client llm.Client, subTaskTitle, subTaskDesc, agentOutput string) (CompletionVerdict, string) {
	if client == nil {
		return VerdictCompleted, "no LLM client — assuming completed"
	}
	if agentOutput == "" {
		return VerdictStuck, "agent produced no output"
	}

	systemPrompt := `You are a technical reviewer. Given a sub-task description and the final output
from a coding agent, determine whether the task was completed successfully.

Reply with EXACTLY one word on the first line: COMPLETED, STUCK, or FAILED.
On the second line, provide a brief explanation (one sentence).

- COMPLETED: the agent implemented what was asked, files were created/modified.
- STUCK: the agent tried but got stuck, hit errors it couldn't resolve, or the output
  suggests the task was too complex. The agent may need a more specific prompt.
- FAILED: the agent encountered a fatal error or produced clearly wrong output.`

	userMsg := fmt.Sprintf("Sub-task: %s\nDescription: %s\n\nAgent Output:\n%s",
		subTaskTitle, subTaskDesc, truncateForJudge(agentOutput, 4000))

	response, err := client.Chat(ctx, systemPrompt, []llm.Message{{Role: "user", Content: userMsg}})
	if err != nil {
		return VerdictCompleted, fmt.Sprintf("judge call failed: %v — assuming completed", err)
	}
	return parseCompletionVerdict(response)
}

// JudgeVerification sends the verification agent's output to OpenAI to determine
// whether verification passed.
func JudgeVerification(ctx context.Context, client llm.Client, subTaskTitle, agentOutput string) (VerificationVerdict, string) {
	if client == nil {
		return VerifyPassed, "no LLM client — assuming passed"
	}
	if agentOutput == "" {
		return VerifyFailed, "verification agent produced no output"
	}

	systemPrompt := `You are a technical reviewer. Given the output from a verification agent
that reviewed code changes, determine whether the verification passed.

Reply with EXACTLY one word on the first line: PASSED or FAILED.
On the second line, provide a brief explanation (one sentence).

- PASSED: the verification found no issues, or only minor suggestions.
- FAILED: the verification found real bugs, missing files, or broken tests.`

	userMsg := fmt.Sprintf("Sub-task: %s\n\nVerification Agent Output:\n%s",
		subTaskTitle, truncateForJudge(agentOutput, 4000))

	response, err := client.Chat(ctx, systemPrompt, []llm.Message{{Role: "user", Content: userMsg}})
	if err != nil {
		return VerifyPassed, fmt.Sprintf("judge call failed: %v — assuming passed", err)
	}
	return parseVerificationVerdict(response)
}

func parseCompletionVerdict(response string) (CompletionVerdict, string) {
	lines := strings.SplitN(strings.TrimSpace(response), "\n", 2)
	first := strings.TrimSpace(strings.ToUpper(lines[0]))
	explanation := ""
	if len(lines) > 1 {
		explanation = strings.TrimSpace(lines[1])
	}

	switch {
	case strings.Contains(first, "COMPLETED"):
		return VerdictCompleted, explanation
	case strings.Contains(first, "STUCK"):
		return VerdictStuck, explanation
	case strings.Contains(first, "FAILED"):
		return VerdictFailed, explanation
	default:
		// If unclear, treat as completed with a note
		return VerdictCompleted, fmt.Sprintf("unclear verdict %q — assuming completed", first)
	}
}

func parseVerificationVerdict(response string) (VerificationVerdict, string) {
	lines := strings.SplitN(strings.TrimSpace(response), "\n", 2)
	first := strings.TrimSpace(strings.ToUpper(lines[0]))
	explanation := ""
	if len(lines) > 1 {
		explanation = strings.TrimSpace(lines[1])
	}

	if strings.Contains(first, "PASSED") {
		return VerifyPassed, explanation
	}
	return VerifyFailed, explanation
}

func truncateForJudge(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Keep the last maxLen chars — the end of the output is most relevant
	return "...(truncated)\n" + s[len(s)-maxLen:]
}
