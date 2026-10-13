# go-dwg 逆向分析与补全台账

本目录记录 `go-dwg` 纯 Go 解析库的重建过程：**观察 / 推断 / 未知**三态分离，
每条结论标注来源（Rust 参考实现、ODA SDK 静态分析、公开规范）与置信度。

## 依据

- **参考实现**：`../ref/dwg-rs-main`（Apache-2.0，Rust）——提供约 90% 的
  容器层与实体层信息，作为骨架与对拍基准。
- **RAE 分析对象**：`/Users/xuning/Work/go-dgn/ODA-Parse-DWGs-main/`
  （ODA Teigha 25.4 Windows x64 运行时 + 完整头文件 + DWGReadWriteOperator
  示例源码）。用 RAE 静态解析导出表、字符串、反编译，补齐 Rust 未覆盖或
  存疑的字段布局。
- **上游分析**：`/Users/xuning/Work/go-dgn/analysis/`（`01-oda-inventory/`、
  `02-dwg/`、`evidence/`）。本仓库不重复该分析，只引用其结论 ID。

## 文档

- [`status.md`](status.md) — 端口进度、对拍结果、逐层验收。
- [`gap-fill-plan.md`](gap-fill-plan.md) — RAE 补全缺失信息的执行计划。

## 约定

- 对拍以「逐字节一致」为强证据：section 反解字节、对象列表字段、句柄映射。
- 任何偏离 Rust 实现的判断必须显式标注为推断，并给出 RAE/规范证据。
- ODA 为专有授权：分析仅用于理解**文件格式布局与编解码算法**，Go 实现不复刻
  其源码与命名结构。
