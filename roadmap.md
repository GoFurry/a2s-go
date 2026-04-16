# a2s-go Roadmap

## 当前状态

`a2s-go` 已经完成了三层主能力：

- `a2s`：单服 `A2S_INFO / A2S_PLAYER / A2S_RULES`
- `master`：Valve master 分页发现
- `scanner`：批量探测、并发 worker、`master.Stream` 对接

从“可发布、可用的轻量 A2S SDK”来看，当前完成度大约在 `80%~85%`。
从“长期稳定跑真实服务器和大规模扫描”的角度看，完成度大约在 `60%~70%`。

## 已确认的问题

### P1: split packet 后续分包来源校验不完整

当前 `scanner` 复用 worker UDP socket 时，首包会校验来源地址，但进入 multi-packet 组包后，后续分包读取没有继续绑定同一个远端来源。

这会带来两个风险：

- 不同目标服务器的分包在共享 socket 场景下发生串包
- 恶意或异常来源包被错误拼进当前响应

优先级：`高`

### P1: `A2S_INFO` 二次 challenge 刷新处理不完整

`A2S_INFO` 第一次收到 challenge 时当前实现可以工作，但如果服务端后续重新下发 challenge，当前请求构造会继续追加 token，而不是替换已有 token。

这会导致：

- 长生命周期 client 在部分服务端上出现协议头异常
- 与真实网络中的 challenge 刷新行为兼容性不足

优先级：`高`

### P2: scanner 高并发下 UDP 读取分配偏多

当前 UDP 读取路径每次 `Receive / ReceiveFrom` 都会按 `maxPacketSize` 分配缓冲，再复制实际负载。

这不会先造成错误，但在高并发批量扫描时会增加：

- 短生命周期内存分配
- GC 压力
- 高频探测时的吞吐损耗

优先级：`中`

### P3: internal 层测试还不够直接

虽然公开层测试已经覆盖了很多主路径，但 `internal/protocol`、`internal/multipacket`、`internal/transport` 目前主要还是被外层间接覆盖。

这会导致协议边界、异常包、极端分包场景的回归信号还不够强。

优先级：`中`

### P3: scanner 的静态地址输入之前不够顺手

`scanner` 最初只接受：

- `[]master.ServerAddr`
- `master.Stream()` 风格的 `Discovery`

这对和 `master` 对接是自然的，但对“我已经有一批 `host:port` 字符串，只想直接扫”的场景不够友好。

这个问题现在已经通过新增 `scanner.Request{Addresses: []string{...}}` 入口缓解，但还需要继续完善：

- README 示例
- 更明确的输入约束说明
- 地址解析与错误提示测试

优先级：`中`

## 接下来的主线

### Phase 1

先补协议正确性和网络隔离：

1. 修复 multi-packet 后续分包来源校验
2. 修复 `A2S_INFO` challenge 刷新覆盖逻辑
3. 为这两个问题补回归测试

### Phase 2

补 scanner 的易用性和 API 打磨：

1. 稳定 `scanner.Request.Addresses`
2. 评估是否补 `ParseAddresses(...)` 一类显式 helper
3. 明确 `Addresses / Servers / Discovery` 三种输入模式的边界

### Phase 3

补性能和测试基线：

1. 为 scanner 增加 benchmark
2. 评估 buffer 复用或 `sync.Pool`
3. 补 internal 层单测和 fuzz 样例

### Phase 4

补真实世界验证：

1. 增加真实服务器手动回归样例
2. 验证老游戏、异常 challenge、压缩包、分包兼容性
3. 为 release 建立更清晰的质量门槛

## 一句话结论

`a2s-go` 现在已经不只是一个 demo 了，但距离“放心长期跑在真实服务器和大规模扫描上”的状态，还需要优先补协议边界、scanner 可用性和性能基线。
