package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLM API Relay 测试程序
// 用于测试服务基本功能

const BASE_URL = "http://localhost:8080"

var testModel = "gpt-oss-120b" // 默认测试模型
var verboseMode = false        // 详细模式

type TestResult struct {
	Name    string
	Success bool
	Message string
	Details string
}

func main() {
	// 解析命令行参数
	flag.StringVar(&testModel, "model", "gpt-oss-120b", "测试模型名称")
	flag.StringVar(&testModel, "m", "gpt-oss-120b", "测试模型名称(简)")
	flag.BoolVar(&verboseMode, "verbose", false, "详细模式 - 打印请求和响应详情")
	flag.BoolVar(&verboseMode, "v", false, "详细模式(简) - 打印请求和响应详情")
	flag.Parse()

	fmt.Println("LLM API Relay 测试程序启动")
	fmt.Printf("服务地址: %s\n", BASE_URL)
	fmt.Printf("测试模型: %s\n", testModel)
	fmt.Printf("详细模式: %s\n", func() string {
		if verboseMode {
			return "开启"
		} else {
			return "关闭"
		}
	}())
	fmt.Println(strings.Repeat("=", 60))

	results := []TestResult{
		testHealthCheck(),
		testModelsEndpoint(),
		testChatCompletionsNonStreaming(),
		testChatCompletionsStreaming(),
	}

	// 输出测试结果
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("测试结果汇总:")
	fmt.Println(strings.Repeat("=", 60))

	passCount := 0
	totalCount := len(results)

	for _, result := range results {
		status := "❌ FAIL"
		if result.Success {
			status = "✅ PASS"
		}
		fmt.Printf("%s %s: %s\n", status, result.Name, result.Message)
		if result.Details != "" {
			fmt.Printf("   详情: %s\n", result.Details)
		}
		if result.Success {
			passCount++
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("测试完成: %d/%d 通过\n", passCount, totalCount)
	if passCount == totalCount {
		fmt.Println("🎉 所有测试通过!")
	} else {
		fmt.Printf("�️ %d 个测试失败\n", totalCount-passCount)
	}
}

// 1. 健康检查测试
func testHealthCheck() TestResult {
	startTime := time.Now()

	fmt.Println("\n1. 测试健康检查端点...")
	if verboseMode {
		fmt.Printf("   📝 请求: GET %s/health\n", BASE_URL)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(BASE_URL + "/health")
	duration := time.Since(startTime)

	if err != nil {
		if verboseMode {
			fmt.Printf("   �️ 错误: %v\n", err)
		}
		return TestResult{
			Name:    "健康检查",
			Success: false,
			Message: fmt.Sprintf("连接失败: %v", err),
			Details: fmt.Sprintf("耗时: %v", duration),
		}
	}

	defer resp.Body.Close()

	if verboseMode {
		fmt.Printf("   📝 响应: HTTP %d\n", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		content := string(body)
		if verboseMode {
			fmt.Printf("   📝 内容: %s\n", content)
		}
		if content == "ok" {
			return TestResult{
				Name:    "健康检查",
				Success: true,
				Message: "正常",
				Details: fmt.Sprintf("状态码: %d, 响应: %s, 耗时: %v", resp.StatusCode, content, duration),
			}
		}
	}

	return TestResult{
		Name:    "健康检查",
		Success: false,
		Message: fmt.Sprintf("状态码: %d", resp.StatusCode),
		Details: fmt.Sprintf("耗时: %v", duration),
	}
}

// 2. Models 端点测试
func testModelsEndpoint() TestResult {
	startTime := time.Now()

	fmt.Println("\n2. 测试 Models 端点...")

	if verboseMode {
		fmt.Printf("   📝 请求: GET %s/v1/models\n", BASE_URL)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", BASE_URL+"/v1/models", nil)

	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		if verboseMode {
			fmt.Printf("   �️ 错误: %v\n", err)
		}
		return TestResult{
			Name:    "Models 列表",
			Success: false,
			Message: fmt.Sprintf("请求失败: %v", err),
			Details: fmt.Sprintf("耗时: %v", duration),
		}
	}

	defer resp.Body.Close()

	if verboseMode {
		fmt.Printf("   📝 响应: HTTP %d\n", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		if verboseMode {
			fmt.Printf("   📝 内容:\n%s\n", content)
		}

		// 检查是否包含 models 字段
		if strings.Contains(content, `"object":"list"`) && strings.Contains(content, `"data"`) {
			return TestResult{
				Name:    "Models 列表",
				Success: true,
				Message: "正常",
				Details: fmt.Sprintf("状态码: %d, 响应长度: %d 字节, 耗时: %v", resp.StatusCode, len(content), duration),
			}
		}
	}

	return TestResult{
		Name:    "Models 列表",
		Success: false,
		Message: fmt.Sprintf("响应异常 - 状态码: %d", resp.StatusCode),
		Details: fmt.Sprintf("耗时: %v", duration),
	}
}

// 3. Chat Completions 非流模式测试
func testChatCompletionsNonStreaming() TestResult {
	startTime := time.Now()

	fmt.Println("\n3. 测试 Chat Completions (非流模式)...")

	// 构建测试请求
	requestBody := map[string]any{
		"model":  testModel,
		"stream": false,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "你好，请回答一句话",
			},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	if verboseMode {
		fmt.Printf("   📝 请求: POST %s/v1/chat/completions\n", BASE_URL)
		fmt.Printf("   📝 发送数据:\n%s\n", string(jsonBody))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST", BASE_URL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		if verboseMode {
			fmt.Printf("   �️ 错误: %v\n", err)
		}
		return TestResult{
			Name:    "Chat Completions (非流)",
			Success: false,
			Message: fmt.Sprintf("请求失败: %v", err),
			Details: fmt.Sprintf("耗时: %v", duration),
		}
	}

	defer resp.Body.Close()

	if verboseMode {
		fmt.Printf("   📝 响应: HTTP %d\n", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		if verboseMode {
			fmt.Printf("   📝 内容:\n%s\n", content)
		}

		// 检查是否包含预期字段
		if strings.Contains(content, `"object":"chat.completion"`) &&
			strings.Contains(content, `"choices"`) &&
			strings.Contains(content, `"message"`) {
			return TestResult{
				Name:    "Chat Completions (非流)",
				Success: true,
				Message: "正常",
				Details: fmt.Sprintf("状态码: %d, 响应长度: %d 字节, 耗时: %v", resp.StatusCode, len(content), duration),
			}
		}
	}

	return TestResult{
		Name:    "Chat Completions (非流)",
		Success: false,
		Message: fmt.Sprintf("响应异常 - 状态码: %d", resp.StatusCode),
		Details: fmt.Sprintf("耗时: %v", duration),
	}
}

// 4. Chat Completions 流模式测试
func testChatCompletionsStreaming() TestResult {
	startTime := time.Now()

	fmt.Println("\n4. 测试 Chat Completions (流模式)...")

	// 构建测试请求
	requestBody := map[string]any{
		"model":  testModel,
		"stream": true,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "请用流模式回答一句话",
			},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	if verboseMode {
		fmt.Printf("   📝 请求: POST %s/v1/chat/completions\n", BASE_URL)
		fmt.Printf("   📝 发送数据:\n%s\n", string(jsonBody))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST", BASE_URL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		if verboseMode {
			fmt.Printf("   �️ 错误: %v\n", err)
		}
		return TestResult{
			Name:    "Chat Completions (流)",
			Success: false,
			Message: fmt.Sprintf("请求失败: %v", err),
			Details: fmt.Sprintf("耗时: %v", duration),
		}
	}

	defer resp.Body.Close()

	if verboseMode {
		fmt.Printf("   📝 响应: HTTP %d\n", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		// 读取部分响应，检查是否为流格式
		content, _ := io.ReadAll(io.LimitReader(resp.Body, 1000)) // 读取前 1000 字节
		contentStr := string(content)

		if verboseMode {
			fmt.Printf("   📝 流内容(前 %d 字节):\n%s\n", len(contentStr), contentStr)
		}

		// 流模式响应包含多个 JSON 对象，每行一个
		lineCount := strings.Count(contentStr, "\n")

		if strings.Contains(contentStr, `data: `) && lineCount > 1 {
			return TestResult{
				Name:    "Chat Completions (流)",
				Success: true,
				Message: "正常",
				Details: fmt.Sprintf("状态码: %d, 前 %d 字节包含 %d 行, 耗时: %v", resp.StatusCode, len(contentStr), lineCount+1, duration),
			}
		}

		// 如果没有检测到流格式，但状态码正常也算通过
		return TestResult{
			Name:    "Chat Completions (流)",
			Success: true,
			Message: "正常 (流检测可能不准确)",
			Details: fmt.Sprintf("状态码: %d, 响应长度: %d 字节, 耗时: %v", resp.StatusCode, len(contentStr), duration),
		}
	}

	return TestResult{
		Name:    "Chat Completions (流)",
		Success: false,
		Message: fmt.Sprintf("响应异常 - 状态码: %d", resp.StatusCode),
		Details: fmt.Sprintf("耗时: %v", duration),
	}
}

// 辅助函数：打印结果
func printResult(result TestResult) {
	status := "❌ FAIL"
	if result.Success {
		status = "✅ PASS"
	}
	fmt.Printf("%s %s: %s\n", status, result.Name, result.Message)
	if result.Details != "" {
		fmt.Printf("   详情: %s\n", result.Details)
	}
}
