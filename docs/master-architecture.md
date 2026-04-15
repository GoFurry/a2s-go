# a2s-go Master/Discovery 架构设计

## 1. 文档目标

本文档定义 `a2s-go` 下一阶段中 `master/discovery` 能力的架构边界、模块职责、数据流和默认行为。

这一层的目标不是“查询单台服务器”，而是“从 Valve Master Server 持续拿到服务器地址列表”，为后续批量探测打基础。

本文档只覆盖：

- `master/discovery`：拉取服务器地址

本文档明确不覆盖：

- `scanner/probe`：对大量服务器做高并发探测
- 多协议聚合发现
- 持久化缓存
- 分布式任务调度

## 2. 设计目标

这一层只追求四件事：

- 稳定拿到服务器地址列表
- 调用链足够简单
- 对 Master Query 协议细节做内聚封装
- 为下一阶段 `scanner/probe` 预留自然扩展点

这意味着：

- discovery 层只负责“发现地址”
- 不在 discovery 层混入 `QueryInfo`、`QueryPlayers`、`QueryRules`
- 不在首版 discovery 层引入高并发扫描调度器

## 3. 分层结构

### 3.1 顶层包 `master`

职责：

- 对外暴露 `Client`
- 对外暴露 `Option`
- 对外暴露 filter、region、cursor、result 等类型
- 对外暴露统一错误类型
- 承载 `NewClient`、`Query`、`Stream`、`Close`

约束：

- `Client` 只负责 Master Server Query
- 不向外暴露底层 UDP 包格式
- 不向外暴露游标字节级细节

### 3.2 `internal/masterprotocol`

职责：

- Master Server Query 请求构造
- Master 响应头解析
- 服务器地址列表解码
- 终止游标判断

约束：

- 只关心 Master Query 协议格式
- 不负责网络 I/O
- 不负责重试和分页策略

### 3.3 `internal/transport`

职责：

- UDP 连接建立和关闭
- 单次请求发送与响应接收
- deadline 和 `context` 协调

约束：

- 不解析 Master 响应语义
- 不维护游标推进逻辑

### 3.4 `internal/errors`

职责：

- discovery 层内部错误构造
- 协议错误、地址错误、超时错误的分类

## 4. 核心数据流

单次 discovery 查询统一遵循下面的流程：

```text
Query(ctx, req)
  -> 校验 region / filter / cursor
  -> 构造 master query 请求包
  -> 发送 UDP 请求
  -> 接收响应
  -> 校验 master 响应头
  -> 解码 server addresses
  -> 读取 next cursor
  -> 返回 page 结果
```

如果调用的是流式接口，则数据流为：

```text
Stream(ctx, req)
  -> 从起始 cursor 开始
  -> 循环 Query page
  -> 每页发出 addresses
  -> 游标推进
  -> 遇到终止 cursor 停止
```

## 5. Client 模型

`master.Client` 与当前 `a2s.Client` 一样，维持“小而明确”的模型：

- 一个 `Client` 默认复用一个 UDP 连接
- 一个 `Client` 面向一个 master server 地址
- 一个 `Client` 只承载 discovery 行为

默认不做：

- 多 master server 自动故障切换
- 客户端连接池
- 背景刷新 goroutine

## 6. 查询模型

discovery 层只提供两种查询方式：

### 6.1 单页查询

适用场景：

- 手工翻页
- 调试协议
- 调用方自己控制分页推进

语义：

- 输入一个 `Request`
- 返回一个 `Page`
- `Page` 包含本页地址和下一页游标

### 6.2 连续流式查询

适用场景：

- 一次性遍历某个 region 下的服务器
- 后续接 `scanner/probe`

语义：

- 输入一个 `Request`
- 内部自动推进 cursor
- 逐页产出结果
- 支持 `context` 取消

## 7. 过滤模型

discovery 首版只暴露一个原始 filter 字符串入口，不在首版强行设计复杂 DSL。

原因：

- Valve master filter 本身就是字符串协议
- 不同游戏和服务端生态会使用不同过滤条件
- 过早把 filter 做成结构化 DSL，后续很容易反复破坏兼容性

因此首版只承诺：

- 提供 `Filter string`
- 提供若干常见 helper 作为纯函数字符串构造器是可以考虑的
- 不在公开契约里把 filter 语法“类型系统化”

## 8. Region 模型

region 应作为明确的公开类型，而不是裸 `byte`。

原因：

- 这是协议里稳定的有限枚举
- 用枚举比 magic number 清楚得多

首版 region 只承诺覆盖常用区域常量，并保留 `RegionCustom(byte)` 兜底能力。

## 9. Cursor 模型

cursor 是 discovery 分页的核心状态。

设计原则：

- 对调用方可见，但不暴露协议字节细节
- 调用方可以保存并继续使用
- 默认支持从“起始 cursor”开始
- 支持判断“是否终止”

不建议把 cursor 设计成普通字符串，因为它本质上是“地址边界状态”而不是人类可读 ID。

## 10. 错误模型

discovery 层继续沿用“单一公开错误类型”的思路。

建议新增一套面向 master query 的错误码，例如：

- `address`
- `dial`
- `write`
- `read`
- `timeout`
- `packet_header`
- `decode`
- `filter`
- `cursor`
- `region`

调用方仍然应使用稳定错误码，而不是匹配错误字符串。

## 11. 与 `a2s.Client` 的关系

`master.Client` 和现有 `a2s.Client` 是并列关系，不是嵌套关系。

即：

- `a2s.Client`：查询单台已知服务器
- `master.Client`：从 master server 发现服务器地址

后续 `scanner/probe` 才是把两者串起来的调度层。

## 12. 首版边界

本阶段明确不进入以下能力：

- 批量高并发探测
- worker pool
- rate limit
- 自动重试矩阵
- 结果去重数据库
- 持久化断点续扫
- 多 master server 冗余容错

## 13. 对下一阶段的影响

当前 `master/discovery` 的输出必须天然适配下一阶段 `scanner/probe`。

因此这里要保证：

- 输出地址类型稳定
- `Stream` 接口可以被扫描器直接消费
- `Page` 结果包含明确 `NextCursor`
- 错误模型可以区分“发现失败”和“后续探测失败”

## 14. 一句话结论

`master/discovery` 的定位是：

> 只负责稳定地从 Valve Master Server 获取服务器地址列表，不负责对这些地址做高并发探测；它是 `a2s-go` 从“单服查询”走向“可发现服务器”的第一层基础能力。
