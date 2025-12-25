package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	var verbose bool
	var listTests bool
	var runSpecificTest string

	flag.BoolVar(&verbose, "v", false, "verbose mode - print test details")
	flag.BoolVar(&verbose, "verbose", false, "verbose mode - print test details")
	flag.BoolVar(&listTests, "list", false, "list all available tests")
	flag.StringVar(&runSpecificTest, "run", "", "run specific test by name pattern")
	flag.Parse()

	if verbose {
		fmt.Println("LLM API Relay 单元测试运行器")
		fmt.Println("=" + strings.Repeat("=", 50))
	}

	// 如果只是列出测试
	if listTests {
		fmt.Println("可用的单元测试:")
		fmt.Println("- TestStripJSONC: JSONC注释解析功能测试")
		fmt.Println("- TestLoadConfigJSONC: 配置文件加载功能测试")
		fmt.Println("- TestFindRule: 模型规则查找功能测试")
		fmt.Println("- TestGetString: 字符串值获取功能测试")
		fmt.Println("- TestApplyRules: 模型规则应用功能测试")
		fmt.Println("- TestCopyHeaders: HTTP头部复制功能测试")
		fmt.Println("- TestProxyPassthrough: HTTP代理透传功能测试")
		fmt.Println("- TestShouldEnableToolCallFix: toolcallfix启用判断功能测试")
		fmt.Println("- TestConfigWithToolCallFix: toolcallfix配置解析测试")
		fmt.Println("- TestConfigWithoutToolCallFix: 向后兼容性测试")
		fmt.Println("- TestProxyWithJSONPatchWithToolCallFix: 集成功能测试")
		return
	}

	// 运行测试
	fmt.Println("运行 LLM API Relay 单元测试...")

	if verbose {
		fmt.Printf("详细模式: 开启\n")
	}

	// 构建go test命令
	args := []string{"test"}
	if verbose {
		args = append(args, "-v")
	}
	if runSpecificTest != "" {
		args = append(args, "-run", runSpecificTest)
	}

	// 添加要测试的文件
	args = append(args,
		"../main_test.go",
		"../main.go",
		"../toolcallfix_integration_test.go",
		"../toolcallfix/transform_test.go",
		"../toolcallfix/transform.go",
	)

	// 设置工作目录为项目根目录
	cmd := exec.Command("go", args...)
	cmd.Dir = ".."

	if verbose {
		fmt.Printf("执行命令: go %s\n", strings.Join(args, " "))
		fmt.Printf("工作目录: %s\n", cmd.Dir)
	}

	// 设置标准输出和错误输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 运行测试
	if err := cmd.Run(); err != nil {
		if verbose {
			fmt.Printf("测试执行失败: %v\n", err)
		}
		os.Exit(1)
	}

	if verbose {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("测试执行完成!")
		fmt.Println(strings.Repeat("=", 60))
	}
}
