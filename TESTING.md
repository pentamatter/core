# 测试指南

本项目使用真实的 MongoDB、Redis 和 Meilisearch 进行集成测试。

## 前置条件

- Go 1.24+
- Docker 和 Docker Compose

## 快速开始

### 1. 启动测试服务

```bash
make test-up
```

这会启动以下服务：
- MongoDB: `localhost:27018`
- Redis: `localhost:6380`
- Meilisearch: `localhost:7701`

### 2. 运行测试

```bash
# 使用 Docker 容器运行所有测试
make test-docker

# 或者如果你本地已有服务运行在默认端口
make test-integration
```

### 3. 停止测试服务

```bash
make test-down
```

## 测试命令

| 命令 | 说明 |
|------|------|
| `make test` | 运行所有测试（需要本地服务） |
| `make test-unit` | 仅运行单元测试（无外部依赖） |
| `make test-integration` | 运行集成测试（使用默认端口） |
| `make test-docker` | 运行测试（使用 Docker 容器端口） |
| `make test-coverage` | 生成测试覆盖率报告 |
| `make test-up` | 启动测试容器 |
| `make test-down` | 停止测试容器 |
| `make clean` | 清理测试产物 |

## 环境变量

可以通过环境变量自定义测试连接：

```bash
export TEST_MONGO_URI=mongodb://localhost:27017
export TEST_REDIS_ADDR=localhost:6379
export TEST_REDIS_PASSWORD=
export TEST_MEILI_HOST=http://localhost:7700
export TEST_MEILI_KEY=
```

## 测试结构

```
internal/
├── testutil/
│   └── testutil.go      # 测试工具和环境设置
├── repository/
│   ├── mongo_test.go    # MongoDB 集成测试
│   ├── redis_test.go    # Redis 集成测试
│   └── meili_test.go    # Meilisearch 集成测试
└── service/
    ├── auth_test.go     # 认证服务测试
    ├── emoji_test.go    # Emoji 验证测试
    ├── reaction_test.go # Reaction 服务测试
    ├── session_test.go  # Session 服务测试
    ├── sync_test.go     # 同步服务测试
    └── validator_test.go # Schema 验证测试
```

## 测试隔离

每个测试运行时会：
1. 创建唯一的 MongoDB 数据库（`test_matter_<timestamp>`）
2. 创建唯一的 Meilisearch 索引（`test_entries_<timestamp>`）
3. 使用 Redis DB 15 进行测试
4. 测试结束后自动清理所有资源

## 跳过测试

如果某个服务不可用，相关测试会自动跳过而不是失败。例如：

```
=== SKIP: TestMeiliRepo_IndexAndSearch
    meili_test.go:15: Skipping test: Meilisearch not available
```

## CI/CD 集成

在 CI 环境中，可以使用 `docker-compose.test.yml` 启动服务：

```yaml
# GitHub Actions 示例
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      mongo:
        image: mongo:7
        ports:
          - 27017:27017
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
      meilisearch:
        image: getmeili/meilisearch:v1.12
        ports:
          - 7700:7700
        env:
          MEILI_ENV: development
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test -v ./...
```
