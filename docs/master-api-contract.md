# a2s-go Master/Discovery 接口契约

## 1. 模块与包名

- module：`github.com/GoFurry/a2s-go`
- package：`master`

首版将 discovery 能力放在一级公开子包中，推荐目录形态为：

```text
master/
  client.go
  options.go
  types.go
  errors.go
```

## 2. 公共构造接口

```go
func NewClient(opts ...Option) (*Client, error)
```

契约说明：

- `Client` 面向 Valve Master Server 查询
- 默认 master server 地址由实现内置
- 可以通过 option 覆盖 base address

## 3. 公共类型

```go
type Client struct { ... }
type Option func(*clientConfig) error
```

约束：

- `Client` 为不透明类型
- `clientConfig` 为内部实现细节，不导出

## 4. 公共方法

```go
func (c *Client) Query(ctx context.Context, req Request) (*Page, error)
func (c *Client) Stream(ctx context.Context, req Request) (<-chan Result, error)
func (c *Client) Close() error
```

行为契约：

- `Query` 只请求单页
- `Stream` 自动推进 cursor 并连续产出结果
- `Close` 释放底层 UDP 连接
- `Close` 可重复调用，不应 panic

## 5. Request 契约

```go
type Request struct {
    Region Region
    Filter string
    Cursor Cursor
}
```

字段说明：

- `Region`：目标区域
- `Filter`：master filter 原始字符串
- `Cursor`：分页起点

默认行为：

- `Cursor` 为空值时，表示从起始 cursor 开始
- `Filter` 为空时，表示不过滤

## 6. Page 契约

```go
type Page struct {
    Servers    []ServerAddr
    NextCursor Cursor
    Done       bool
}
```

字段说明：

- `Servers`：当前页服务器地址
- `NextCursor`：下一页游标
- `Done`：是否已到终止游标

行为约束：

- `Done == true` 时，`NextCursor` 仍应可被序列化保存
- `Servers` 允许为空；空页不等于错误

## 7. Result 契约

```go
type Result struct {
    Server ServerAddr
    Err    error
}
```

说明：

- `Stream` 返回的 channel 输出逐个服务器地址
- 首版 `Err` 只用于流中断前的分页错误通知
- `Server` 和 `Err` 同时存在时，以 `Err` 为准

如果实现上更倾向按页输出，也可以改为：

```go
type Result struct {
    Page *Page
    Err  error
}
```

但首版推荐“逐地址输出”，因为更方便后续对接 `scanner/probe`。

## 8. 地址类型契约

```go
type ServerAddr struct {
    IP   net.IP
    Port uint16
}
```

行为约束：

- 必须可转为 `host:port`
- 必须支持 IPv4
- 首版不承诺 IPv6

建议补充：

```go
func (a ServerAddr) String() string
```

## 9. Cursor 契约

```go
type Cursor struct { ... }
```

约束：

- `Cursor` 对外可见，但内部表示不承诺稳定
- 允许零值表示起始 cursor
- 应支持：

```go
func StartCursor() Cursor
func (c Cursor) IsZero() bool
func (c Cursor) IsTerminal() bool
func (c Cursor) String() string
```

说明：

- `String()` 用于日志、调试、持久化
- 不要求调用方理解内部协议编码

## 10. Region 契约

```go
type Region byte
```

建议至少提供：

```go
const (
    RegionUSEast      Region = 0x00
    RegionUSWest      Region = 0x01
    RegionSouthAmerica Region = 0x02
    RegionEurope      Region = 0x03
    RegionAsia        Region = 0x04
    RegionAustralia   Region = 0x05
    RegionMiddleEast  Region = 0x06
    RegionAfrica      Region = 0x07
    RegionRestOfWorld Region = 0xFF
)
```

并建议补充：

```go
func (r Region) String() string
```

## 11. 首版公开 Option

```go
func WithTimeout(d time.Duration) Option
func WithBaseAddress(addr string) Option
func WithMaxPacketSize(size int) Option
```

### 11.1 `WithTimeout`

契约：

- 设置默认超时
- `d` 必须大于 `0`

### 11.2 `WithBaseAddress`

契约：

- 覆盖默认 master server 地址
- 主要用于测试、私有网关或后续兼容扩展

### 11.3 `WithMaxPacketSize`

契约：

- 设置 UDP 接收缓冲上限
- `size` 必须大于协议最小安全值

## 12. 不纳入首版的 Option

- `WithLogger`
- `WithRetry`
- `WithRateLimit`
- `WithFallbackMasters`
- `WithResolver`

原因：

- 这些能力会显著放大公开 API 面
- 当前阶段优先保证 discovery 基础能力稳定

## 13. 错误契约

```go
type ErrorCode string

const (
    ErrorCodeAddress      ErrorCode = "address"
    ErrorCodeDial         ErrorCode = "dial"
    ErrorCodeWrite        ErrorCode = "write"
    ErrorCodeRead         ErrorCode = "read"
    ErrorCodeTimeout      ErrorCode = "timeout"
    ErrorCodePacketHeader ErrorCode = "packet_header"
    ErrorCodeDecode       ErrorCode = "decode"
    ErrorCodeFilter       ErrorCode = "filter"
    ErrorCodeCursor       ErrorCode = "cursor"
    ErrorCodeRegion       ErrorCode = "region"
)

type Error struct {
    Code string
    Op   string
    Addr string
    Err  error
}
```

行为约束：

- `Error` 必须实现 `error`
- `Error` 必须支持 `Unwrap() error`
- 调用方应通过 `Code` 做稳定判断

## 14. 行为约束

### 14.1 查询行为

- `Query` 只拉单页
- `Stream` 负责自动翻页
- `context` 为最高优先级取消条件

### 14.2 过滤行为

- `Filter` 原样传给 master 协议层
- 不在首版做语义解释和校验 DSL
- 只做最基本的空值与非法编码校验

### 14.3 分页行为

- 零值 cursor 表示从起点开始
- terminal cursor 表示遍历结束
- `Stream` 必须在 terminal cursor 处正常关闭 channel

## 15. 兼容性承诺

首版 discovery 的兼容性承诺固定如下：

- 保证 `Client`、`Request`、`Page`、`ServerAddr`、`Cursor`、`Region` 的公开语义稳定
- 保证 `Query`、`Stream`、`Close` 的方法名和核心语义稳定
- 不承诺内部 UDP 实现细节稳定
- 不承诺 cursor 的内部编码格式稳定

## 16. 一句话结论

discovery 首版接口契约的核心是：

> 用最小但稳定的公开 API 提供“按 region/filter 从 Master Server 获取服务器地址并分页推进”的能力，为后续扫描器提供干净输入，而不在这一层提前引入批量探测复杂度。
