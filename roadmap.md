# a2s-go Roadmap

## 当前状态

`a2s-go` 已经完成三层主能力：

- `a2s`：单服 `A2S_INFO / A2S_PLAYER / A2S_RULES`
- `master`：Valve master 分页发现
- `scanner`：批量探测、并发 worker、`master.Stream` 对接

从“可发布、可用的轻量 A2S SDK”来看，当前完成度大约在 `85%` 左右。
从“长期稳定跑真实服务器和大规模扫描”的角度看，完成度大约在 `80%~85%`。

## 2026-04-16 更新

本轮已经完成 Phase 1、Phase 2、Phase 3，并推进了 Phase 4：

- Phase 1：协议正确性和网络隔离
- Phase 2：scanner 易用性和输入 API 打磨
- Phase 3：性能和测试基线
- Phase 4：真实世界验证工具和 release 门槛

其中 Phase 4 本轮完成了这些事情：

1. 增加真实服务器手动回归示例 `examples/live-regression`
2. 把单服探测和 scanner 批量探测放进同一条手动验证路径
3. 新增 `release-checklist.md`，明确 release 前的自动门槛和手动检查项

## 已完成的问题

### P1: split packet 后续分包来源校验不完整

问题背景：

- `scanner` 复用 worker UDP socket 时，首包会校验来源地址
- 旧实现进入 multi-packet 组包后，没有继续绑定同一个远端来源

影响：

- 共享 socket 场景下可能出现分包串包
- 异常来源包可能被错误拼进当前响应

状态：`已完成`

处理结果：

- multi-packet 后续读包现在会继续绑定预期来源地址
- 已补共享 socket 场景的回归测试

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

### P2: scanner 静态地址输入不够顺手

问题背景：

- `scanner` 之前只接受 `[]master.ServerAddr` 或 `Discovery`
- 对“我已经有一批 `host:port` 字符串，只想直接扫”的场景不够友好

状态：`已完成`

处理结果：

- 已支持 `scanner.Request{Addresses: []string{...}}`
- 已新增 `scanner.ParseAddress(...)` / `scanner.ParseAddresses(...)`
- 已明确三种输入模式的边界和约束

### P2: scanner 高并发下 UDP 读取分配偏多

问题背景：

- 旧实现的 `Receive / ReceiveFrom` 每次都会按 `maxPacketSize` 分配大缓冲
- 高频本地扫描下这会放大短生命周期分配和 GC 压力

状态：`已完成`

处理结果：

- 已为 UDP 读取路径引入 buffer pool
- Phase 3 benchmark 可以作为后续继续优化的基线

## 仍待处理的问题

### P3: internal 层测试覆盖还不够系统

虽然现在已经补了 `internal/transport` 和 `internal/protocol` 的直接测试，但整体 internal 层仍然不是“系统性覆盖”。

还可以继续补的方向：

- `internal/multipacket` 的更细粒度单测
- 更有针对性的 fuzz corpus
- benchmark 结果的长期追踪基线

状态：`进行中`

### P4: 真实服务器兼容矩阵还需要持续积累

虽然现在已经有了 live regression 示例和 release checklist，但真实世界兼容矩阵本身仍需要长期积累。

还可以继续补的方向：

- 老游戏或非主流服务端样本库
- challenge / split / compressed 真实样本记录
- release 历史里的实测记录沉淀

状态：`进行中`

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

状态：`已完成`

已完成项：

1. 稳定 `scanner.Request.Addresses`
2. 新增 `ParseAddress(...)` / `ParseAddresses(...)` helper
3. 明确 `Addresses / Servers / Discovery` 三种输入模式的边界

### Phase 3

目标：补性能和测试基线。

状态：`已完成（首轮）`

已完成项：

1. 为 scanner 增加 benchmark
2. 通过 buffer pool 降低 UDP 读取路径重复分配
3. 补 internal 层单测和 fuzz 样例

后续仍可继续：

1. 为 `internal/multipacket` 增加更细 benchmark
2. 评估更进一步的零拷贝或结果对象复用
3. 建立 benchmark 结果记录和回归门槛

### Phase 4

目标：补真实世界验证。

状态：`进行中`

已完成项：

1. 增加真实服务器手动回归示例
2. 为 release 建立更清晰的质量门槛

下一步：

1. 用真实服务器目标积累兼容矩阵
2. 验证老游戏、异常 challenge、压缩包、分包兼容性
3. 把 live regression 结果沉淀到 release 历史

## 一句话结论

`a2s-go` 现在已经有了真实服务器手动回归工具和 release 检查单；接下来最值得投入的方向，是把这些工具跑在更丰富的真实服务端样本上，并持续沉淀兼容矩阵。
