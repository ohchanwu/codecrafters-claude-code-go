package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

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
				// var parsedToolArgs ToolArgsWrapper
				// if err := json.Unmarshal([]byte(toolArgsJSON), &parsedToolArgs); err != nil {
				// 	log.Fatal("error unmarshaling json tool arguments, perhaps the agent hallucinated:", err)
				// }

				var toolCallResult string
				switch toolName {
				case "Read":
					parsedToolArgs, err := parseToolArgs[ReadToolArgs](toolArgsJSON)
					if err != nil {
						log.Fatal(err)
					}
					toolCallResult, err = Read(parsedToolArgs.FilePath)
					if err != nil {
						log.Fatal(err)
					}
				case "Write":
					parsedToolArgs, err := parseToolArgs[WriteToolArgs](toolArgsJSON)
					if err != nil {
						log.Fatal(err)
					}
					err = Write(parsedToolArgs.FilePath, parsedToolArgs.Content)
					if err != nil {
						log.Fatal(err)
					}
					toolCallResult = "Write successful"
				case "Bash":
					parsedToolArgs, err := parseToolArgs[BashToolArgs](toolArgsJSON)
					if err != nil {
						log.Fatal(err)
					}
					toolCallResult, err = Bash(parsedToolArgs.Command)
					if err != nil {
						log.Fatal(err)
					}
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
