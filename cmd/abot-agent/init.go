package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"abot/pkg/agent"
)

func runInitWizard() error {
	fmt.Println("🤖 abot 配置向导")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	// 1. 检测 API Key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		fmt.Printf("✓ 检测到 OPENAI_API_KEY\n")
	} else {
		fmt.Print("请输入 OpenAI API Key: ")
		scanner.Scan()
		apiKey = strings.TrimSpace(scanner.Text())
		if apiKey == "" {
			return fmt.Errorf("API Key 不能为空")
		}
	}

	// 2. 选择模型
	fmt.Println("\n选择模型：")
	fmt.Println("1. gpt-4o-mini (推荐，快速且便宜)")
	fmt.Println("2. gpt-4o")
	fmt.Println("3. gpt-4-turbo")
	fmt.Print("请选择 [1-3]: ")
	scanner.Scan()
	modelChoice := strings.TrimSpace(scanner.Text())
	if modelChoice == "" {
		modelChoice = "1"
	}

	model := "gpt-4o-mini"
	switch modelChoice {
	case "2":
		model = "gpt-4o"
	case "3":
		model = "gpt-4-turbo"
	}

	// 3. Agent 名称
	fmt.Print("\nAgent 名称 [assistant]: ")
	scanner.Scan()
	agentName := strings.TrimSpace(scanner.Text())
	if agentName == "" {
		agentName = "assistant"
	}

	// 4. 会话存储
	fmt.Println("\n会话存储方式：")
	fmt.Println("1. 内存 (重启后丢失)")
	fmt.Println("2. JSONL 文件 (持久化)")
	fmt.Print("请选择 [1-2]: ")
	scanner.Scan()
	storageChoice := strings.TrimSpace(scanner.Text())
	if storageChoice == "" {
		storageChoice = "2"
	}

	sessionType := "jsonl"
	sessionDir := "data/sessions"
	if storageChoice == "1" {
		sessionType = "memory"
		sessionDir = ""
	}

	// 5. 生成配置
	cfg := &agent.Config{
		AppName: "abot",
		Providers: []agent.ProviderConfig{
			{
				Name:    "primary",
				APIBase: "https://api.openai.com/v1",
				APIKey:  apiKey,
				Model:   model,
			},
		},
		Agents: []agent.AgentDefConfig{
			{
				ID:          "default-bot",
				Name:        agentName,
				Description: "A helpful assistant",
				Model:       model,
			},
		},
		Session: agent.SessionConfig{
			Type: sessionType,
			Dir:  sessionDir,
		},
		ContextWindow: 128000,
	}

	// 6. 保存配置
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	configPath := "config.yaml"
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("\n✓ 配置已保存到 %s\n", configPath)
	fmt.Println("\n启动命令：")
	fmt.Printf("  ./abot-agent -config %s\n", configPath)

	return nil
}

func promptInt(prompt string, min, max int) int {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(prompt + ": ")
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			return min
		}
		n, err := strconv.Atoi(input)
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Printf("请输入 %d-%d 之间的数字\n", min, max)
	}
}
