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

	for {
		resp, err := client.Chat.Completions.New(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model:    "anthropic/claude-haiku-4.5",
				Messages: messages,
				Tools: []openai.ChatCompletionToolUnionParam{
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
				},
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

		if resp.Choices[0].FinishReason == "stop" {
			fmt.Println("Agent loop stopped.")
			break
		}

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
				if toolName == "Read" {
					toolCallResult, err = Read(parsedToolArgs.FilePath)
					if err != nil {
						log.Fatal(err)
					}
				} else {
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
		return "", errors.New("model did not return a filepath")
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("error reading file:", err)
	}
	return string(bs), nil
}
