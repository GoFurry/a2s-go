# a2s-go Roadmap

`a2s-go` 当前已经完成了三层基础能力：

- 根包 `a2s`：单服 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES`
- 子包 `master`：地址发现
- 子包 `scanner`：批量 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES`

这份 roadmap 只保留后续仍值得继续改进的内容。

## 近期优先级

- 补一轮真实服务器兼容性样本，覆盖更多 Source 游戏和边缘返回包
- 继续扩大异常输入样本库，把真实线上抓到的异常包沉淀成固定回归测试
- 增加更丰富的手动测试基线，特别是 fake master、fake A2S server 和 discovery + probe 串联场景

## 性能与工程化

- 评估 `scanner` 的连接复用策略，减少大批量短生命周期 `a2s.Client` 带来的 UDP dial/close 开销
- 评估 `scanner` 在大批量结果场景下的背压和内存占用，必要时补限流或统计接口
- 评估解码与多包拼装路径中的临时分配，优先优化高频热点而不是提前做复杂抽象
- 逐步补充 metrics / profiling 基线，先看真实瓶颈再决定是否深入优化

## 网络适配

- 当前仍保持直连 UDP 为主，不内建代理能力
- 如果后续为中国地区网络环境补代理支持，优先评估 `SOCKS5 UDP ASSOCIATE`、自定义 `Dialer`、`net.PacketConn` 或本地 relay
- 不计划把普通 HTTP 代理直接当作 A2S / Discovery / Probe 的通用代理方案
- 暂不内建代理池轮转，等核心链路和真实网络诉求稳定后再决定

## 更后面的方向

- Master filter helper
- 更完整的兼容性文档
- 更细的使用示例与接入指南
