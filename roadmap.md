# a2s-go Roadmap

## 当前状态

`a2s-go` 已经完成三层主能力：

- `a2s`：单服 `A2S_INFO / A2S_PLAYER / A2S_RULES`
- `master`：Valve master 分页发现
- `scanner`：批量探测、并发 worker、`master.Stream` 对接

从“可发布、可用的轻量 A2S SDK”来看，当前完成度大约在 `80%~85%`。
从“长期稳定跑真实服务器和大规模扫描”的角度看，完成度大约在 `65%~75%`。

## 2026-04-16 更新

本轮已完成 roadmap 里的两个 P1：

- 已修复 split packet 后续分包来源校验不完整的问题
- 已修复 `A2S_INFO` 二次 challenge 刷新时 token 继续追加的问题
- 已补两组回归测试，覆盖上述两个高优先级场景

这意味着 Phase 1 里最关键的协议正确性问题已经落地，不再只是待办。

## 已确认的问题与状态

### P1: split packet 后续分包来源校验不完整

问题背景：

- `scanner` 复用 worker UDP socket 时，首包会校验来源地址
- 但进入 multi-packet 组包后，旧实现没有继续绑定同一个远端来源

影响：

- 不同目标服务器的分包在共享 socket 场景下可能串包
- 恶意或异常来源包可能被错误拼进当前响应

状态：`已完成`

处理结果：

- multi-packet 后续读包现在会继续绑定预期来源地址
- 对 `NewClientWithConn(...)` 的共享 socket 场景补了回归测试

### P1: `A2S_INFO` 二次 challenge 刷新处理不完整

问题背景：

- `A2S_INFO` 第一次收到 challenge 时旧实现可以工作
- 如果服务端再次下发 challenge，旧实现会继续追加 token，而不是覆盖旧 token

影响：

- 长生命周期 client 在部分服务端上可能出现协议头异常
- 与真实网络中的 challenge 刷新行为兼容性不足

状态：`已完成`

处理结果：

- `A2S_INFO` 请求现在会覆盖旧 challenge token，而不是无限追加
- 已补连续 challenge 回归测试

### P2: scanner 高并发下 UDP 读取分配偏多

当前 `Receive / ReceiveFrom` 每次都会按 `maxPacketSize` 分配缓冲，再复制实际负载。

这不会先造成错误，但在高并发批量扫描时会增加：

- 短生命周期内存分配
- GC 压力
- 高频探测时的吞吐损耗

状态：`待处理`

### P3: internal 层测试还不够直接

虽然公开层测试已经覆盖了不少主路径，但 `internal/protocol`、`internal/multipacket`、`internal/transport` 目前仍主要依赖外层间接覆盖。

这会导致协议边界、异常包、极端分包场景的回归信号还不够强。

状态：`待处理`

### P3: scanner 静态地址输入还可以继续打磨

`scanner` 之前只接受：

- `[]master.ServerAddr`
- `master.Stream()` 风格的 `Discovery`

现在已经新增了 `scanner.Request{Addresses: []string{...}}`，静态扫地址不再需要手写 `master.ServerAddr`。

但这一块还有继续优化空间：

- README 示例继续补充
- 更明确的输入约束说明
- 评估是否补 `ParseAddresses(...)` 一类显式 helper

状态：`部分完成`

## 接下来的主线

### Phase 1

目标：协议正确性和网络隔离。

状态：`已完成`

已完成项：

1. 修复 multi-packet 后续分包来源校验
2. 修复 `A2S_INFO` challenge 刷新覆盖逻辑
3. 为这两个问题补回归测试

### Phase 2

目标：补 scanner 的易用性和 API 打磨。

状态：`进行中`

下一步：

1. 稳定 `scanner.Request.Addresses`
2. 评估是否补 `ParseAddresses(...)` 一类 helper
3. 明确 `Addresses / Servers / Discovery` 三种输入模式的边界

### Phase 3

目标：补性能和测试基线。

状态：`待开始`

下一步：

1. 为 scanner 增加 benchmark
2. 评估 buffer 复用或 `sync.Pool`
3. 补 internal 层单测和 fuzz 样例

### Phase 4

目标：补真实世界验证。

状态：`待开始`

下一步：

1. 增加真实服务器手动回归样例
2. 验证老游戏、异常 challenge、压缩包、分包兼容性
3. 为 release 建立更清晰的质量门槛

## 一句话结论

`a2s-go` 现在已经不只是一个 demo 了，Phase 1 的两个高优先级协议问题也已经完成；接下来最值得投入的方向，是 scanner 易用性、性能基线和真实服务器回归。
