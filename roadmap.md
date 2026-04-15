# a2s-go Roadmap

## 当前状态

当前仓库已经完成的基线是：

- 单地址 `Client`
- `QueryInfo`
- `QueryPlayers`
- `QueryRules`
- challenge 自动处理
- split packet / 压缩包处理
- example、CI、自动化测试

这意味着 `a2s-go` 已经具备“稳定查询单台已知服务器”的能力。

## 下一阶段：Master/Discovery

下一阶段的目标是把 `a2s-go` 从“单服查询库”推进到“可发现服务器”。

这一阶段只做：

- `master/discovery`
- 从 Valve Master Server 拉服务器地址列表
- 单页查询与连续流式查询
- region / filter / cursor 的公开契约

这一阶段明确不做：

- 高并发扫描
- worker pool
- 结果聚合
- 去重存储
- 分布式扫描

对应文档：

- [Master/Discovery 架构设计](docs/master-architecture.md)
- [Master/Discovery 接口契约](docs/master-api-contract.md)

## 下一个阶段目标：Scanner/Probe

在 discovery 稳定后，下一阶段目标是补 `scanner/probe`，用于对大量服务器做高并发探测。

这一层的定位是：

- 消费 `master/discovery` 输出的地址流
- 控制并发度
- 调用现有 `a2s.Client` 或共享底层探测器
- 输出批量 `QueryInfo` / `QueryPlayers` / `QueryRules` 结果

这一阶段预计会解决：

- worker pool
- 并发上限
- 按需查询类型选择
- 背压
- 结果流式输出
- 批量超时与失败隔离

这一阶段暂时不在当前轮次设计公开契约，等 discovery 落地后再单独建文档。

## 更后面的方向

在 `scanner/probe` 之后，可以考虑：

- Master filter helper
- 更丰富的手动测试基线
- 真实服务器兼容性样本集
- 更细的 metrics / profiling
- 可选的扫描结果导出格式

## 一句话总结

`a2s-go` 的演进顺序建议是：

1. 单服查询稳定
2. Master/Discovery 完成
3. Scanner/Probe 完成
4. 再考虑外围工程化和批量能力增强
