# LLM API Relay - Makefile
# 用于管理多二进制Go项目的构建和开发

# 变量定义
BIN_DIR := bin
MAIN_BINARY := llm-api-relay
TEST_BINARY := relay-test
RUNNER_BINARY := test-runner

# 默认目标
.DEFAULT_GOAL := help

# 列出所有可用的目标
.PHONY: help
help:
	@echo "LLM API Relay 多二进制项目管理"
	@echo ""
	@echo "可用命令:"
	@echo "  build          - 构建所有二进制文件"
	@echo "  build-main     - 构建主服务二进制"
	@echo "  build-test     - 构建测试工具二进制"
	@echo "  build-runner   - 构建测试运行器二进制"
	@echo "  clean          - 清理所有构建产物"
	@echo "  test           - 运行所有测试"
	@echo "  test-unit      - 运行单元测试"
	@echo "  test-integration - 运行集成测试"
	@echo "  test-coverage  - 运行测试并生成覆盖率报告"
	@echo "  test-race      - 运行竞态条件检测"
	@echo "  test-bench     - 运行性能测试"
	@echo "  test-all       - 运行完整测试套件（包含覆盖率和竞态检测）"
	@echo "  lint           - 代码规范检查"
	@echo "  fmt            - 格式化代码"
	@echo "  vet            - 代码静态分析"
	@echo "  run            - 运行主服务"
	@echo "  run-test       - 运行测试工具"
	@echo "  install        - 安装依赖"
	@echo "  deps           - 更新依赖"
	@echo "  all            - 完整构建和测试流程"
	@echo ""
	@echo "使用示例:"
	@echo "  make build          # 构建所有二进制"
	@echo "  make test           # 运行所有测试"
	@echo "  make test-coverage  # 运行测试并查看覆盖率"
	@echo "  make test-race      # 运行竞态条件检测"
	@echo "  make test-all       # 运行完整测试套件"
	@echo "  make all            # 完整构建和测试"

# 创建二进制目录
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# 构建主服务二进制
.PHONY: build-main
build-main: $(BIN_DIR)
	@echo "构建主服务二进制: $(MAIN_BINARY)"
	go build -o $(BIN_DIR)/$(MAIN_BINARY) ./main.go
	@echo "✓ 主服务二进制构建完成: $(BIN_DIR)/$(MAIN_BINARY)"

# 构建测试工具二进制
.PHONY: build-test
build-test: $(BIN_DIR)
	@echo "构建测试工具二进制: $(TEST_BINARY)"
	go build -o $(BIN_DIR)/$(TEST_BINARY) -tags="relay-test" ./cmd/relay-test.go
	@echo "✓ 测试工具二进制构建完成: $(BIN_DIR)/$(TEST_BINARY)"

# 构建测试运行器二进制
.PHONY: build-runner
build-runner: $(BIN_DIR)
	@echo "构建测试运行器二进制: $(RUNNER_BINARY)"
	go build -o $(BIN_DIR)/$(RUNNER_BINARY) -tags="test-runner" ./cmd/test-runner.go
	@echo "✓ 测试运行器二进制构建完成: $(BIN_DIR)/$(RUNNER_BINARY)"

# 构建所有二进制文件
.PHONY: build
build: build-main build-test build-runner
	@echo ""
	@echo "所有二进制文件构建完成!"
	@echo "生成的文件:"
	@ls -la $(BIN_DIR)/ 2>/dev/null || echo "检查生成的文件"

# 清理构建产物
.PHONY: clean
clean:
	@echo "清理构建产物..."
	rm -rf $(BIN_DIR)
	@echo "✓ 构建产物清理完成"

# 运行所有测试
.PHONY: test
test: test-unit test-integration
	@echo ""
	@echo "✓ 所有测试运行完成!"

# 运行单元测试
.PHONY: test-unit
test-unit:
	@echo "运行单元测试..."
	go test -v .  # 测试主包
	go test -v ./toolcallfix/...  # 测试 toolcallfix 包

# 运行集成测试
.PHONY: test-integration
test-integration:
	@echo "运行集成测试..."
	go test -v -run "TestToolCallFixIntegration" .

# 运行带覆盖率的测试
.PHONY: test-coverage
test-coverage:
	@echo "运行测试并生成覆盖率报告..."
	go test -coverprofile=coverage.out . ./toolcallfix
	go tool cover -func=coverage.out
	@echo ""
	@echo "生成HTML覆盖率报告: coverage.html"
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ 覆盖率报告生成完成"

# 运行竞态条件检测
.PHONY: test-race
test-race:
	@echo "运行竞态条件检测..."
	go test -race -run "^Test[^I]" .  # 排除集成测试
	go test -race ./toolcallfix/...

# 运行性能测试
.PHONY: test-bench
test-bench:
	@echo "运行性能测试..."
	go test -bench=. -benchmem . ./toolcallfix

# 运行所有测试（完整版）
.PHONY: test-all
test-all: test test-coverage test-race
	@echo ""
	@echo "🎉 完整测试套件运行完成!"

# 代码规范检查
.PHONY: lint
lint:
	@echo "代码规范检查..."
	go vet ./...
	go vet ./cmd/...
	go vet ./toolcallfix/...

# 格式化代码
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	go fmt ./...
	go fmt ./cmd/...
	go fmt ./toolcallfix/...

# 代码静态分析
.PHONY: vet
vet:
	@echo "代码静态分析..."
	go vet ./...
	go vet ./cmd/...
	go vet ./toolcallfix/...

# 安装依赖
.PHONY: install
install:
	@echo "安装依赖..."
	go mod download
	go mod tidy

# 更新依赖
.PHONY: deps
deps:
	@echo "更新依赖..."
	go get -u ./...
	go mod tidy

# 运行主服务
.PHONY: run
run:
	@echo "运行主服务..."
	go run ./main.go --config config.jsonc

# 运行测试工具
.PHONY: run-test
run-test:
	@echo "运行测试工具..."
	go run ./cmd/relay-test.go

# 完整构建和测试流程
.PHONY: all
all: clean install fmt vet test build
	@echo ""
	@echo "🎉 完整构建和测试流程完成!"

# 开发模式：实时重新构建和运行
.PHONY: dev
dev:
	@echo "开发模式：监控文件变化..."
	@echo "请手动运行: make build && make run"
	@echo "或: make test && make build-test && make run-test"

# 安装到系统（需要sudo权限）
.PHONY: install-system
install-system: build
	@echo "安装二进制文件到系统..."
	sudo cp $(BIN_DIR)/$(MAIN_BINARY) /usr/local/bin/
	sudo cp $(BIN_DIR)/$(TEST_BINARY) /usr/local/bin/
	sudo cp $(BIN_DIR)/$(RUNNER_BINARY) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(MAIN_BINARY)
	sudo chmod +x /usr/local/bin/$(TEST_BINARY)
	sudo chmod +x /usr/local/bin/$(RUNNER_BINARY)
	@echo "✓ 二进制文件已安装到系统"

# 从系统卸载
.PHONY: uninstall-system
uninstall-system:
	@echo "从系统卸载二进制文件..."
	sudo rm -f /usr/local/bin/$(MAIN_BINARY)
	sudo rm -f /usr/local/bin/$(TEST_BINARY)
	sudo rm -f /usr/local/bin/$(RUNNER_BINARY)
	@echo "✓ 二进制文件已从系统卸载"

# 显示版本信息
.PHONY: version
version:
	@echo "Go版本: $$(go version)"
	@echo "项目信息:"
	@cat go.mod | head -3