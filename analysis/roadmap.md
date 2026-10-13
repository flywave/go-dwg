# 路线图（两条路线，按优先级）

目标：纯 Go DWG 解析库。方法：**先参照 Rust 逐项对拍一致（路线 A）**，再用
**RAE + ODA SDK 补 Rust 覆盖不到的部分（路线 B）**。

判定口径：
- **差异可验证**＝语料（合成 6 + ODA R2004/R2013）里存在该类型记录，且 Go 与
  预言机逐字段一致。
- **仅向量可验证**＝语料无该记录，以 Rust 自带测试向量在 Go 侧复刻。
- **Rust 缺口**＝Rust 对该记录报 error/unhandled；移植后与 Rust 行为一致但
  无成功解码，须靠路线 B。

---

## 路线 A — 移植 Rust（对拍可验证）

### A0 已完成（差异可验证）

容器层（版本/头部/section map/LZ77/CRC/RS/R2007）· 元数据 · handle/class map ·
对象 walker · 实体 LINE/CIRCLE/ARC/POINT/BLOCK/ENDBLK/SEQEND ·
对象 DICTIONARY/CONTROL/PLACEHOLDER/XRECORD ·
表项 APPID/LAYER/LTYPE/STYLE/BLOCK_HEADER/DIMSTYLE ·
自定义类分发 + DICTIONARYVAR/SCALE/ACDBDICTIONARYWDFLT/VISUALSTYLE/MLEADERSTYLE。

### A1 剩余差异可验证项（按价值/成本）

| 序 | 目标 | 语料记录 | 预估 | 状态 |
|---|---|---|---|---|
| A1.1 | SECTIONVIEWSTYLE / DETAILVIEWSTYLE | 各 1 | 中 | 待做 |
| A1.2 | VIEW / UCS / VPORT 表项 | VPORT 1 | 中 | 待做 |
| A1.3 | 固定码 LAYOUT(0x52) / MLINESTYLE | 0（语料用 custom LAYOUT） | 中 | 低收益 |

### A2 高价值但语料无记录（仅向量可验证）

TEXT / MTEXT / INSERT / LWPOLYLINE / DIMENSION / HATCH / SPLINE / ELLIPSE /
3DFACE —— 真实图纸最常用。需先建 **Go BitWriter** + 预言机「按字节解码」模式，
以 Rust 测试向量对拍。**这是路线 A 的战略主线**（完整性），但短期内不提升
当前语料对拍数。

### A3 Rust 缺口（转入路线 B）

`3DSOLID/BODY/REGION` 在本 ODA R2013 语料上与 Rust **同为 error**
（delta 64 / 位数不足；Rust 仅在其私有 R2018 样例验证过）→ 无成功解码，
属路线 B。

---

## 路线 B — RAE + ODA SDK 补 Rust 缺口（主推进）

证据分层：E1 `ODA/include/*.h`（类字段/枚举）；E2 REA/Ghidra 反编译
`TD_DbEntities.tx`/`TD_DbCore.dll` 的 `dwgInFields`；E3 ODS v5.4.1 + 公开实现。

### B1 高优先级（当前语料缺口，可直接提升对拍数）

| 序 | 目标 | 语料记录 | 依据 |
|---|---|---|---|
| B1.1 | ✅ **已完成** `3DSOLID/REGION/BODY` §20.4.41（R2013 修订字段 8 字节） | 4 | vtable 解析 + trace |
| B1.2 | `LAYOUT`（custom 类 502，Rust 不派发） | 3 | E1 `DbLayout.h` + E2 |
| B1.3 | `MATERIAL`（Rust 仅解析前缀） | 7 | E1 `DbMaterial.h` + E2 |
| B1.4 | `SORTENTSTABLE` / `LIGHT` / `SUN` / `SOLID_BACKGROUND` | 各 1 | E1/E2 |

### B2 中优先级

- 实体 `TEXT/MTEXT/INSERT/LWPOLYLINE/DIMENSION/HATCH` 的字段序以 E2 复核
  （Rust 已实现，RAE 用于交叉验证 Rust 存疑处）。
- 类型化 `XRECORD`（code,value）链（Rust 保持 opaque，§19.6.5 / §3.5）。

### B3 待补样例

- 真实 **R2007 (AC1021)** 样例 → R2007 容器 section 字节端到端对拍
  （当前仅向量级）。
- 真实 **R2018** 样例 → 复核 R2010+ 多字节 MC 等。

---

## 执行顺序（主推进）

1. **B1.1** 3DSOLID 修复（先做，直接 +4 且暴露真实缺口）。
2. **A1.1/A1.2** SECTION/DETAILVIEWSTYLE、VIEW/UCS/VPORT（+3~4）。
3. **B1.2/B1.3** LAYOUT/MATERIAL（+10）。
4. **A2** 标准实体批量（BitWriter + 字节级对拍）—— 战略完整性主线。
5. **B3** 补 R2007/R2018 真实样例。

依赖：B1 需要 REA 打开 ODA ELF 模块（已验证可用：
`ODAToolkit/bin/lnxX64_11dll/TD_DbEntities.tx`）。

---

## 路线 B 侦察记录（E2，`TD_DbEntities.tx`，ELF x86-64，未 strip）

`sha256=2ac2f87de0ef75ffc1b1096c3901956c847ef5852b5e6e85d4adf5123eb26af8`

用 `nm` + `objdump` 定位（无需 Ghidra 全量分析）：

| 符号 | 地址 | 说明 |
|---|---|---|
| `OdDb3dSolid::dwgInFields(OdDbDwgFiler*)` | `0x89b68a` (~192B) | 先 `OdDbEntity::dwgInFields`（通用实体），再 `OdDbModelerGeometryImpl::dwgInFields`，然后若 filer 版本 `>0x19` 读一个 int64 到 `impl+0x1f8` |
| `OdDbModelerGeometryImpl::dwgInFields` | `0x8b71a4` (~4572B) | **§20.4.41 完整字段序**（envelope + wireframe + data-store） |
| `OdDbModelerGeometryImpl::dwgOutFields` | `0x8b8380` | 写侧，可交叉印证字段序 |
| `OdDbAcisIO::readAcisData(OdDbDwgFiler*, ...)` | `0x8b622a` | ACIS SAT/SAB 读取（含 data-store 变体） |

结论：
- 3DSOLID 的 §20.4.41 布局在 **`OdDbModelerGeometryImpl::dwgInFields`**；
  字段序需反编译该函数（4.5KB，含大量 filer 虚调用，`call *偏移(%rax)`，偏移即
  `OdDbDwgFiler` 虚表方法序号 → 由 E1 `DbFiler.h` 的 rd* 声明序映射）。
- `OdDbModelerGeometryImpl::dwgInFields` 全量 Ghidra 分析在请求窗口内未完成
  （模块 21MB）。**下一次**：`open_binary` 后按符号地址单独 `analyze_function`，
  或先 `objdump` 提取 filer 虚表偏移序列 + E1 `DbFiler.h` 映射，重建字段序。

## 当前对齐状态（截至本轮）

- 路线 A 已对拍：AC1018 = 40，AC1027 = 157（`entity_values` 投影）。
- `3DSOLID/BODY/REGION` 已移植但本 R2013 语料上与 Rust **同为 error**
  （delta 64 / 位数不足）→ 记为路线 B 缺口（B1.1）。

