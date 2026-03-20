package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nevalla/k8s-pager/kube"
)

const maxToolRounds = 10

type DiagnosisRequest struct {
	Cluster      string
	Namespace    string
	ResourceKind string
	ResourceName string
	Reason       string
	EventNote    string
}

type TroubleshootingAgent struct {
	llm      LLMClient
	executor *kube.Executor
}

func NewTroubleshootingAgent(llm LLMClient, executor *kube.Executor) *TroubleshootingAgent {
	return &TroubleshootingAgent{
		llm:      llm,
		executor: executor,
	}
}

func (a *TroubleshootingAgent) Diagnose(ctx context.Context, req DiagnosisRequest) (string, error) {
	system := "You are a Kubernetes diagnostics agent. Your job is to investigate issues and identify the root cause. " +
		"Do NOT suggest fixes or remediation steps. Only report what is wrong and why. " +
		"Be concise. Format your response using Slack mrkdwn syntax (*bold*, _italic_, `code`). " +
		"Do NOT wrap your response in code fences (```)."

	messages := []Message{
		{
			Role: "user",
			Text: fmt.Sprintf(
				"A Kubernetes %s is experiencing issues. Please diagnose the problem.\n\n"+
					"Cluster: %s\nNamespace: %s\n%s: %s\nEvent Reason: %s\nEvent Message: %s\n\n"+
					"Use the available tools to investigate and gather relevant information. "+
					"Identify the root cause. Do not suggest fixes.",
				req.ResourceKind, req.Cluster, req.Namespace, req.ResourceKind, req.ResourceName, req.Reason, req.EventNote,
			),
		},
	}

	for round := range maxToolRounds {
		resp, err := a.llm.Send(ctx, system, messages)
		if err != nil {
			return "", fmt.Errorf("LLM call (round %d): %w", round, err)
		}

		// Build assistant message
		assistantMsg := Message{Role: "assistant", Text: resp.Text, ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMsg)

		if resp.Done || len(resp.ToolCalls) == 0 {
			return resp.Text, nil
		}

		// Execute tools and build results
		var results []ToolResult
		for _, tc := range resp.ToolCalls {
			slog.Info("executing tool", "tool", tc.Name, "round", round)
			result, err := executeTool(ctx, a.executor, tc.Name, tc.Input)
			isError := false
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				isError = true
			}
			results = append(results, ToolResult{
				ToolCallID: tc.ID,
				Content:    result,
				IsError:    isError,
			})
		}
		messages = append(messages, Message{Role: "user", ToolResults: results})
	}

	return "Diagnosis timed out after maximum tool rounds.", nil
}
