# a2s-go Scanner/Probe Architecture

## 1. Document Goal

本文定义 `a2s-go` 中 `scanner/probe` 这一层的职责边界、执行模型和默认行为。

这一层的目标不是发现服务器地址，而是把地址列表或 discovery 流稳定地转成批量查询结果：

- `A2S_INFO`
- `A2S_PLAYER`
- `A2S_RULES`

## 2. Layering

三层职责固定如下：

- `a2s`: 查询单个已知服务器
- `master`: 从 master server 发现地址
- `scanner`: 消费地址并发执行批量 probe

因此 `scanner` 是 `master` 和 `a2s` 之间的调度层，不是新的协议层。

## 3. Public `scanner` Package

职责：

- 暴露批量 probe 的公开入口
- 控制 worker pool 并发上限
- 统一 batch 级输入校验与错误模型
- 把底层 `a2s.Client` 结果汇流成流式或聚合输出

约束：

- 不向外暴露连接池
- 不向外暴露 worker 内部状态
- 默认不去重、不重试、不做代理

## 4. Input Model

当前支持两种输入：

- 固定 `[]master.ServerAddr`
- `<-chan master.Result`

设计原因：

- 地址切片适合离线、独立扫描
- discovery 结果流适合直接串联 `master.Stream`
- 两种入口共同覆盖“已有地址批量探测”和“边发现边探测”两类场景

当前版本不内建去重，如需去重，由调用方在进入扫描器前自行处理。

## 5. Execution Model

执行流程固定为：

```text
ProbeX(ctx, req)
  -> 校验输入形状
  -> 启动固定数量 workers
  -> intake goroutine 按输入源投递地址
  -> worker 为每个地址创建临时 a2s.Client
  -> 调用 QueryInfo / QueryPlayers / QueryRules
  -> 关闭该地址对应连接
  -> 按完成顺序输出 Result / PlayersResult / RulesResult
```

关键约束：

- `scanner.Client` 不持有长期 UDP 连接
- 每个地址只执行一次目标查询
- 一个 worker 同时只处理一个地址
- 并发上限完全由 worker 数控制
- `Probe` / `Collect` 保留为 `Info` 兼容入口，`ProbeInfo` / `CollectInfo` 语义等价

## 6. Error Propagation

错误分两类：

- discovery 侧错误：输入流返回 `master.Result.Err`
- probe 侧错误：单目标 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES` 失败

统一策略：

- 两类错误都通过结果流输出
- 单个错误不打断整批扫描
- `scanner.Error` 通过 `Unwrap()` 保留底层 `*master.Error` 或 `*a2s.Error`

## 7. Context and Shutdown

`context` 规则固定如下：

- `ctx` 取消后停止继续 intake 新地址
- 已经开始执行的 probe 继续使用同一个 `ctx`
- 不额外制造批次级哨兵错误
- 当 intake 结束且 worker 全部退出后关闭结果 channel

## 8. Not In Scope

以下能力当前明确不进入这一层：

- 地址去重
- 自动重试
- 代理与代理池
- 连接池
- metrics / profiling
- 批量结果导出格式
