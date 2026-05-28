package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

// Split the monolithic "ToolArgs" struct type into modular types.
// Maybe not necessary for a simple implementation like this with 3 tools,
// but if more tools are added (15+), I believe this will prove prudent.
type ReadToolArgs struct {
	FilePath string `json:"file_path"`
}

type WriteToolArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type BashToolArgs struct {
	Command string `json:"command"`
}

type ToolArgsInterface interface {
	ReadToolArgs | WriteToolArgs | BashToolArgs
}

func parseToolArgs[T ToolArgsInterface](
	toolArgsJSON string,
) (parsedToolArgs *T, err error) {
	if err := json.Unmarshal([]byte(toolArgsJSON), &parsedToolArgs); err != nil {
		return nil, fmt.Errorf("error unmarshaling json tool arguments, perhaps the agent hallucinated: %w", err)
	}
	return parsedToolArgs, nil
}

var tools = []openai.ChatCompletionToolUnionParam{
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name: "Read",
				Description: param.Opt[string]{
					Value: "Read and return the contents of a file",
				},
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "The path to the file to be read",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name: "Write",
				Description: param.Opt[string]{
					Value: "Write content to a file",
				},
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "The content to write to the file",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name: "Bash",
				Description: param.Opt[string]{
					Value: "Execute a shell command",
				},
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	},
}

// Tool intended to be called by an agent.
// Reads the file at the specified path and returns its contents.
func Read(path string) (content string, err error) {
	if path == "" {
		return "", errors.New("Read tool requires a filepath")
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	return string(bs), nil
}

// Tool intended to be called by an agent.
// Writes the content to the file at the specified path.
// Creates the file if it doesn't exist; truncates it if it does.
func Write(path string, content string) error {
	if path == "" {
		return errors.New("Write tool requires a filepath")
	}
	if content == "" {
		return errors.New("Write tool requires content")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0o644))
	if err != nil {
		return fmt.Errorf("error creating file descriptor: %w", err)
	}
	if _, err = f.Write([]byte(content)); err != nil {
		return fmt.Errorf("error writing to file at %s: %w", path, err)
	}
	return nil
}

func Bash(command string) (result string, err error) {
	if command == "" {
		return "", errors.New("Bash tool requires a command")
	}
	split := strings.Split(command, " ")
	path, args := split[0], split[1:]
	cmd := exec.Command(path, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error executing command \"%s\": %w", command, err)
	}
	return string(out), nil
}
