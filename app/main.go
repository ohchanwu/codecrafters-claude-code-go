package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

type ToolArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	messages := []openai.ChatCompletionMessageParamUnion{
		{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfString: openai.String(prompt),
				},
			},
		},
	}

	tools := []openai.ChatCompletionToolUnionParam{
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
	}

	for {
		resp, err := client.Chat.Completions.New(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model:    "anthropic/claude-haiku-4.5",
				Messages: messages,
				Tools:    tools,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		messages = append(messages, resp.Choices[0].Message.ToParam())

		// if resp.Choices[0].FinishReason == "stop" {
		// 	fmt.Println("Agent loop stopped.")
		// 	break
		// }

		// If a tool call is in the response, assume it's a "Read" tool call,
		// execute it, and append the result to the messages array.
		// Otherwise, exit the loop and print the response's content to stdout.
		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			toolCalls := resp.Choices[0].Message.ToolCalls

			for _, toolCall := range toolCalls {
				toolName := toolCall.Function.Name
				toolArgsJSON := toolCall.Function.Arguments
				var parsedToolArgs ToolArgs
				if err := json.Unmarshal([]byte(toolArgsJSON), &parsedToolArgs); err != nil {
					log.Fatal("error unmarshaling json tool arguments, perhaps the agent hallucinated:", err)
				}

				var toolCallResult string
				switch toolName {
				case "Read":
					toolCallResult, err = Read(parsedToolArgs.FilePath)
					if err != nil {
						log.Fatal(err)
					}
				case "Write":
					err = Write(parsedToolArgs.FilePath, parsedToolArgs.Content)
					if err != nil {
						log.Fatal(err)
					}
					toolCallResult = "Write successful"
				default:
					log.Fatal("unrecognized tool call: ", toolName)
				}
				messages = append(messages, openai.ChatCompletionMessageParamUnion{
					OfTool: &openai.ChatCompletionToolMessageParam{
						ToolCallID: toolCall.ID,
						Content: openai.ChatCompletionToolMessageParamContentUnion{
							OfString: openai.String(toolCallResult),
						},
					},
				})
			}

		} else {
			fmt.Print(resp.Choices[0].Message.Content)
			break
		}
	}

	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")
}

// Tool intended to be called by an agent.
// Reads the file at the specified path and returns its contents.
func Read(path string) (content string, err error) {
	if path == "" {
		return "", errors.New("read tool requires a filepath")
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
		return errors.New("write tool requires a filepath")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0o644))
	if err != nil {
		return fmt.Errorf("error creating file descriptor: %w", err)
	}
	if _, err = f.Write([]byte(content)); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}
	return nil
}
