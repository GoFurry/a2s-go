# a2s-go 接口契约

## 1. 模块与包名

- module：`github.com/GoFurry/a2s-go`
- package：`a2s`

首版对外只暴露一个主包，不拆二级公开包。

## 2. 公共构造接口

```go
func NewClient(addr string, opts ...Option) (*Client, error)
```

契约说明：

- `addr` 为目标服务器地址
- 地址不带端口时默认补 `27015`
- 构造阶段完成地址校验和基础 client 初始化
- `Client` 对外为不透明类型，不承诺公开字段

## 3. 公共类型

```go
type Client struct { ... }
type Option func(*clientConfig) error
```

约束：

- `Client` 不暴露内部字段
- `clientConfig` 属于内部实现细节，不导出
- `Option` 只用于首版真正必要的构造配置

## 4. 公共方法

```go
func (c *Client) QueryInfo(ctx context.Context) (*Info, error)
func (c *Client) QueryPlayers(ctx context.Context) (*Players, error)
func (c *Client) QueryRules(ctx context.Context) (*Rules, error)
func (c *Client) Close() error
```

行为契约：

- 三个查询方法都必须接收 `context.Context`
- 三个查询方法都返回 typed 结果
- `Close()` 释放底层 UDP 连接
- `Close()` 可重复调用，重复调用不应导致 panic

## 5. 首版公开 Option

```go
func WithTimeout(d time.Duration) Option
func WithMaxPacketSize(size int) Option
```

### 5.1 `WithTimeout`

契约：

- 设置 client 默认超时
- `d` 必须大于 `0`
- 每次查询时，`context` 的取消/超时优先级高于默认 timeout

### 5.2 `WithMaxPacketSize`

契约：

- 设置 UDP 响应读取缓冲上限
- `size` 必须大于协议默认最小安全值
- 用于兼容响应包偏大的服务器实现

默认建议值在实现阶段确定，但文档要求必须高于常规协议最小缓冲需求。

## 6. 不纳入首版的 Option

以下选项明确不进入首版：

- `WithLogger`
- `WithRetry`
- `WithRateLimit`
- `WithPreOrangeBox`
- `WithAppID`

原因：

- 这些能力要么不是极简核心所必需
- 要么属于历史兼容或特定游戏开关
- 过早暴露会让首版公共 API 变脏

## 7. 返回类型契约

### 7.1 `Info`

`Info` 贴近官方 `A2S_INFO` 字段，包含标准 Source 协议响应中的核心信息。

推荐形状：

```go
type Info struct {
    Protocol   uint8
    Name       string
    Map        string
    Folder     string
    Game       string
    AppID      uint16
    Players    uint8
    MaxPlayers uint8
    Bots       uint8
    ServerType byte
    Environment byte
    Visibility bool
    VAC        bool
    Version    string

    EDF        uint8
    Port       uint16
    SteamID    uint64
    Keywords   string
    GameID     uint64

    TVPort     uint16
    TVName     string
}
```

说明：

- 字段命名以 Go 风格为准
- 字段语义贴近官方协议
- 不引入首版不必要的展示型 helper 字段

### 7.2 `Players`

契约：

```go
type Players struct {
    Count   uint8
    Players []Player
}
```

推荐 `Player` 形状：

```go
type Player struct {
    Index    uint8
    Name     string
    Score    int32
    Duration float32
}
```

### 7.3 `Rules`

契约：

```go
type Rules struct {
    Count uint16
    Items map[string]string
}
```

说明：

- 首版不为 rules 生成特定游戏强类型结构
- 原样保持键值映射最稳妥

## 8. 错误契约

```go
type ErrorCode string

const (
    ErrorCodeAddress      ErrorCode = "address"
    ErrorCodeDial         ErrorCode = "dial"
    ErrorCodeWrite        ErrorCode = "write"
    ErrorCodeRead         ErrorCode = "read"
    ErrorCodeTimeout      ErrorCode = "timeout"
    ErrorCodePacketHeader ErrorCode = "packet_header"
    ErrorCodeChallenge    ErrorCode = "challenge"
    ErrorCodeMultiPacket  ErrorCode = "multi_packet"
    ErrorCodeDecode       ErrorCode = "decode"
    ErrorCodeUnsupported  ErrorCode = "unsupported"
)

type Error struct {
    Code string
    Op   string
    Addr string
    Err  error
}
```

契约说明：

- `Code` 用于稳定分类
- `Op` 表示失败阶段，如 `new_client`、`query_info`、`read_packet`、`decode_rules`
- `Addr` 记录目标地址
- `Err` 保存底层错误
- `Error` 必须实现 `error`
- `Error` 应支持 `Unwrap() error`

## 9. 行为约束

### 9.1 地址处理

- 地址不带端口时默认补 `27015`
- 非法地址在 `NewClient` 阶段返回 `address` 类错误

### 9.2 超时处理

- `WithTimeout` 必须大于 `0`
- 默认 timeout 用于查询读写 deadline
- `context` 取消或超时时，尽量返回可识别的 timeout/cancel 类错误

### 9.3 包大小处理

- `WithMaxPacketSize` 必须大于协议默认最小安全值
- 包太大或无法完整解析时，返回 `multi_packet` 或 `decode` 类错误

### 9.4 协议处理

- challenge、多包、压缩包都是内建能力
- 用户无需手工干预 challenge token
- 用户无法访问底层协议状态

## 10. 兼容性承诺

首版兼容性承诺固定如下：

- 保证方法名稳定
- 保证 `Option` 名称与含义稳定
- 保证 `Info`、`Players`、`Player`、`Rules`、`Error` 的公开语义稳定
- 不承诺内部目录结构稳定
- 不承诺内部实现细节稳定
- 不承诺与 `rumblefrog/go-a2s` 的 API 完全一致

## 11. 首版默认与假设

- 默认端口为 `27015`
- 首版仅支持 `A2S_INFO`、`A2S_PLAYER`、`A2S_RULES`
- 首版仅支持 `Source engine and above`
- 首版不接入第三方底层库
- 首版不提供兼容层

## 12. 一句话结论

首版接口契约的核心是：

> 用最小但稳定的面向对象 API，提供单地址 A2S 查询能力；协议复杂度全部内聚，公共接口只暴露真正长期值得承诺的内容。
