# a2s-go Scanner/Probe API Contract

## 1. Public Package

- package: `scanner`

`scanner` 作为一级公开子包存在，与根包 `a2s` 和子包 `master` 并列。

## 2. Constructor

```go
func NewClient(opts ...Option) (*Client, error)
```

约束：

- `Client` 负责批量调度 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES`
- `Client` 自身不持有长期 UDP 连接
- `NewClient` 只保存并校验并发、超时和最大包长配置

## 3. Public Types

```go
type Request struct {
    Servers   []master.ServerAddr
    Discovery <-chan master.Result
}

type Result struct {
    Server master.ServerAddr
    Info   *a2s.Info
    Err    error
}

type PlayersResult struct {
    Server  master.ServerAddr
    Players *a2s.Players
    Err     error
}

type RulesResult struct {
    Server master.ServerAddr
    Rules  *a2s.Rules
    Err    error
}

type ErrorCode string
type Error struct {
    Code    ErrorCode
    Op      string
    Server  master.ServerAddr
    Message string
    Err     error
}
```

约束：

- `Request` 必须且只能设置一种输入源
- `Servers` 用于固定地址列表
- `Discovery` 用于直接消费 `master.Stream` 风格的发现结果流
- `Result.Info`、`PlayersResult.Players`、`RulesResult.Rules` 只在成功时非空
- `Err` 非空时统一为 `*scanner.Error`，并通过 `Unwrap()` 保留底层 `*a2s.Error` 或 `*master.Error`

## 4. Public Methods

```go
func (c *Client) Probe(ctx context.Context, req Request) (<-chan Result, error)
func (c *Client) Collect(ctx context.Context, req Request) ([]Result, error)

func (c *Client) ProbeInfo(ctx context.Context, req Request) (<-chan Result, error)
func (c *Client) CollectInfo(ctx context.Context, req Request) ([]Result, error)

func (c *Client) ProbePlayers(ctx context.Context, req Request) (<-chan PlayersResult, error)
func (c *Client) CollectPlayers(ctx context.Context, req Request) ([]PlayersResult, error)

func (c *Client) ProbeRules(ctx context.Context, req Request) (<-chan RulesResult, error)
func (c *Client) CollectRules(ctx context.Context, req Request) ([]RulesResult, error)
```

行为：

- `ProbeInfo`、`ProbePlayers`、`ProbeRules` 是主接口，按完成顺序流式输出结果
- `CollectInfo`、`CollectPlayers`、`CollectRules` 是聚合便利层，返回完成顺序切片
- `Probe` / `Collect` 保留为 `Info` 路径的兼容别名
- 不承诺保持输入顺序
- 单个目标失败不终止整批扫描
- discovery 流里出现错误时，转成一次 `discovery` 类型结果继续输出

## 5. Options

```go
func WithConcurrency(n int) Option
func WithTimeout(d time.Duration) Option
func WithMaxPacketSize(size int) Option
```

默认值：

- 并发数：`32`
- 默认超时：`3s`
- `maxPacketSize`：与根包 `a2s` 默认值保持一致

校验：

- `WithConcurrency(n)` 要求 `n > 0`
- `WithTimeout(d)` 要求 `d > 0`
- `WithMaxPacketSize(size)` 要求大于协议安全下限

## 6. Error Codes

- `input`
- `concurrency`
- `timeout`
- `packet_size`
- `discovery`
- `probe`

含义：

- `input`: 请求形状非法
- `concurrency`: 并发配置非法
- `timeout`: 单目标探测超时或被 `context` 截止
- `packet_size`: 包大小配置非法
- `discovery`: discovery 输入流本身返回错误
- `probe`: 单目标 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES` 失败

## 7. Scope

当前版本固定如下：

- 支持批量 `A2S_INFO`
- 支持批量 `A2S_PLAYER`
- 支持批量 `A2S_RULES`
- 不做地址去重
- 不做自动重试
- 不做代理
- 不做连接池
- 不做 metrics/profiling
