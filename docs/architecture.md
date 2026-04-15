# a2s-go 架构设计

## 1. 文档目标

本文档定义 `a2s-go` 首版的架构边界、模块划分、数据流和关键实现策略。

它面向实现者，目标是让后续编码阶段不再需要对首版架构做二次决策。

## 2. 设计目标

首版只追求四件事：

- 小而稳
- 类型安全
- 调用链简单
- 不把兼容历史问题的特殊开关过早暴露成公共 API

这意味着：

- 优先保证 `A2S_INFO`、`A2S_PLAYER`、`A2S_RULES` 可稳定工作
- 优先把协议复杂度封装在内部
- 不在首版引入批量查询、缓存、监控、日志等外围能力

## 3. 架构分层

### 3.1 顶层包 `a2s`

职责：

- 对外暴露 `Client`
- 对外暴露 `Option`
- 对外暴露 typed 返回类型
- 对外暴露统一错误类型
- 承载 `NewClient`、`QueryInfo`、`QueryPlayers`、`QueryRules`、`Close`

约束：

- 不暴露协议状态
- 不暴露 challenge token
- 不暴露 UDP 包解析细节
- 不暴露多包拼装内部类型

### 3.2 `internal/protocol`

职责：

- 协议常量
- 请求包构建
- 响应头识别
- 字段读取与解码
- `A2S_INFO`、`A2S_PLAYER`、`A2S_RULES` 的 payload 解析

约束：

- 只关注协议格式，不负责网络 IO
- 不直接处理连接和超时

### 3.3 `internal/transport`

职责：

- UDP 连接建立与关闭
- 请求发送与响应接收
- deadline 设置
- 与 `context` 协调取消和超时

约束：

- 不解析协议 payload
- 不做业务级错误包装以外的协议判断

### 3.4 `internal/challenge`

职责：

- challenge 请求与响应处理
- `A2S_PLAYER` / `A2S_RULES` 的 challenge 流程
- `A2S_INFO` 的 challenge fallback

约束：

- challenge 对调用方透明
- 不向上层暴露 token

### 3.5 `internal/multipacket`

职责：

- split packet 解析
- 多包响应拼装
- 重复包、越界包、缺包判断
- bzip2 压缩包解压与 checksum 校验

约束：

- 只负责“把多包还原成单个完整 payload”
- 不负责具体查询类型的字段解码

### 3.6 `internal/errors`

职责：

- 内部错误构造
- 错误码与底层错误映射

约束：

- 对外统一通过 `*a2s.Error` 暴露

## 4. 核心数据流

单次查询统一遵循下面的流程：

```text
QueryX(ctx)
  -> 构造请求包
  -> 发送 UDP 请求
  -> 接收响应
  -> 识别单包 / 多包
  -> 如需 challenge，则先完成 challenge 交换
  -> 如为多包，则完成拼装与解压
  -> 解析 payload
  -> 返回 typed 结果
```

三类查询都走统一的大框架，但在请求构造、challenge 流程和 payload 解码上各自实现。

## 5. Challenge 处理策略

首版 challenge 策略固定如下：

- `A2S_PLAYER` 必须支持 challenge
- `A2S_RULES` 必须支持 challenge
- `A2S_INFO` 按官方兼容流程实现 challenge fallback
- challenge 全程由库内部自动处理
- 用户不需要也不能手工提供 challenge token

这样做的原因是：

- challenge 是协议细节，不应成为公共 API 的一部分
- 对调用方而言，查询成功与否比 challenge 中间状态更重要

## 6. 多包处理策略

首版多包策略固定如下：

- 必须支持 split packet
- 必须支持多包按编号拼装
- 必须检测重复包
- 必须检测包编号越界
- 必须支持 bzip2 压缩包解压
- 必须支持 checksum 校验

如果出现以下情况，必须返回统一错误模型中的多包相关错误：

- 重复包
- 越界包
- 拼装后解压失败
- checksum 不匹配

## 7. 连接策略

首版连接模型固定如下：

- 一个 `Client` 绑定一个目标地址
- 一个 `Client` 默认复用一个 UDP 连接
- `Close()` 负责释放底层连接
- 不设计连接池
- 不设计多地址 client manager

地址规则：

- 地址不带端口时，默认补 `27015`

## 8. 上下文与超时

首版上下文和超时策略固定如下：

- 所有查询方法都必须接收 `context.Context`
- `Option` 允许设置默认 timeout
- 每次查询时，`context` 优先级高于默认 timeout
- 如果 `context` 已取消或超时，返回可识别的 timeout/cancel 类错误

这意味着实现上需要：

- 在发送和接收前设置读写 deadline
- 在可能阻塞的等待点优先检查 `ctx.Done()`

## 9. 错误模型

对外只暴露统一错误类型：

```go
type Error struct {
    Code string
    Op   string
    Addr string
    Err  error
}
```

错误码按来源分类，不要求调用方解析错误字符串。

首版错误关注以下几类：

- 地址错误
- 连接错误
- 写入错误
- 读取错误
- 超时错误
- 包头错误
- challenge 错误
- 多包错误
- 解码错误
- 不支持的响应错误

## 10. 首版边界

以下能力明确不进入首版：

- batch query manager
- 池化 client manager
- logger
- metrics
- callback
- retry policy
- rate limit
- 缓存
- 面向具体游戏的 helper

这些能力后续如果要做，也必须建立在当前核心查询链路已经稳定的前提下。

## 11. 对参考实现的使用原则

`rumblefrog/go-a2s` 可以作为参考，但不作为设计约束。

首版实现原则是：

- 参考其成功经验
- 不复制其公开 API
- 不把其历史兼容开关直接照搬进首版公共接口

尤其以下内容首版不直接暴露：

- `PreOrangeBox`
- `SetAppID`
- 其他偏兼容性或特定游戏行为的选项

## 12. 一句话结论

`a2s-go` 首版架构的核心原则是：

> 只提供单地址、单 client、三类核心查询，把 challenge、多包、压缩包和 UDP 细节全部内聚到内部，让外部只看到清晰稳定的 typed API。
