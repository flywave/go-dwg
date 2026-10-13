# RAE 补全计划

目标：用 RAE 静态分析 ODA SDK（`/Users/xuning/Work/go-dgn/ODA-Parse-DWGs-main/`）
补齐 Rust 参考实现缺失或存疑的信息。**容器层已对拍一致，此处只针对缺口。**

## 证据分层（沿用 `go-dgn/analysis` 约定）

- **E1 头文件源码**（`ODA/include/`）：类字段、枚举、版本条件 — 观察级。
  例：`DbLine.h`、`DbEntity.h`、`DbObject.h`、`DbFiler.h`、`DwgClassDef.h`。
- **E2 二进制静态分析**（REA/Ghidra，Windows x64 PE，纯静态）：
  导出表 / RTTI / 字符串 / 反编译 — 观察/推断级。
  主目标模块：
  - `TD_DbIO.tx`（863 导出）：`OdDwgR18FileController/Header/Section`、
    `OdDwgR21FileSplitStream`、`OdDwgR24FileLoader` —— R18/R21/R24 容器代际。
  - `TD_DbCore.dll`（10132 导出）：DWG 对象模型核心 + 版本魔数。
  - `TD_DbEntities.tx`（8070 导出）：实体 `dwgInFields` 家族 —— 实体字段布局主源。
- **E3 公开规范**：Open Design Specification for .dwg files v5.4.1（Rust 即据此
  实现）+ ODA 示例源码（`DWGReadWriteOperator/`）交叉印证 — 推断级。

以上均已在 `go-dgn/analysis/01-oda-inventory/{headers,modules}.md` 与
`go-dgn/analysis/evidence/` 落盘；本仓库引用其结论 ID，不重复盘点。

## 任务优先级

### P0 — R2007（AC1021）容器（纯移植，非 RAE 依赖）

Rust `r2007.rs` + `reed_solomon.rs` + `r21_lz.rs` 已实现该层，直接增量移植到
`dwg/r2007.go`、`dwg/reed_solomon.go`、`dwg/r21_lz.go`，再用构造样例 / 真实
AC1021 文件对拍。**不需要 RAE**——Rust 已覆盖。

### P1 — 逐实体字段解码（移植为主，RAE 兜底）

按 Rust `src/entities/*.rs` + `common_entity.rs` + `string_stream.rs` 增量移植：
LINE / CIRCLE / ARC / POINT 起步，逐步到 LWPOLYLINE / TEXT / MTEXT / INSERT /
DIMENSION 族 / HATCH / MLEADER / SPLINE / 3DFACE / VIEWPORT / 表项。

对拍方式：扩展预言机输出 `decoded_entities` 的**逐字段值**（Rust 已能解码），
Go 侧逐字段比对。Rust 覆盖不足的类型（其 `STATUS.md` 列出的 HATCH 边界树、
MLEADER 引线、完整 75 字段 DIMSTYLE、XRECORD、实体图遍历）转到 P2。

### P2 — RAE 补 Rust 未覆盖处

对 P1 中 Rust 报 `errored`/`unhandled` 或字段可疑的实体，用 RAE：

1. **定位实现**：由 `go-dgn/analysis/01-oda-inventory/modules.md` 的
   `Corresponding C++ library` 注解确定类所在二进制（多为 `TD_DbEntities.tx`）。
2. **提取字段序**：
   - E1：`ODA/include/Db<Class>.h` 的 `dwgInFields(OdDbDwgFiler*)` 声明顺序；
   - E2：REA 对 `TD_DbEntities.tx` 导出表 / RTTI / 反编译，核对
     `dwgInFields` 内的 `filer->rd...()` 调用序列与版本分支；
   - E3：与 ODS v5.4.1 对应章节比对。
3. **边界自校验**：ODS §19.1 的字符串流 / `RL` object-data-size 边界是每条实体
   解码器的自检锚点（Rust 已用此）。RAE 补全时同样以「数据字段恰好落在边界」
   为验收。
4. **落地**：新字段写入 Go 解码器，构造/真实样例对拍通过后方可合入。

### P3 — 写路径（可选）

Rust `file_writer.rs` 为实验性写路径；如需要，按同法移植并用
`DwgFromBytes(to_bytes(f))` 自洽 + 外部 CAD 验收（Rust 亦未自动化该验收）。

## 验收标准

| 层 | 验收 | 依据 |
|---|---|---|
| R2007 容器 | AC1021 文件 section 反解字节与 Rust 一致 | 对拍 |
| 实体解码 | 逐字段值与 Rust 预言机一致；边界自检通过 | 对拍 |
| RAE 补全项 | 字段序有 E1/E2/E3 证据链；对拍通过 | 证据 + 对拍 |

## 未知与风险

- ODA 二进制为 Windows x64 PE，**本机不可运行**，RAE 仅做静态分析。
- 专有授权：分析只用于理解格式布局/算法，Go 实现不复刻 ODA 源码与命名。
- Rust 仅为「约 90%」基准，其自身 `STATUS.md` 承认的缺口即本计划 P2 的输入。

## B1.1 — 3DSOLID §20.4.41 侦察结果（本轮）

对 `ODAToolkit/bin/lnxX64_11dll/TD_DbEntities.tx`
（ELF x86-64，未 strip，`sha256=2ac2f87de0ef75ffc1b1096c3901956c847ef5852b5e6e85d4adf5123eb26af8`）
用 `nm` + `objdump` 精确符号定位：

| 符号 | 地址 | 备注 |
|---|---|---|
| `OdDb3dSolid::dwgInFields` | `0x89b68a` | 先调 `OdDbEntity::dwgInFields`，再 `OdDbModelerGeometryImpl::dwgInFields`；filer 版本 `>0x19` 时补读一个 int64 |
| `OdDbModelerGeometryImpl::dwgInFields` | `0x8b71a4` | **§20.4.41 字段序所在**，~4572 B，含 42 处 filer 虚调用（`addq $off,%rax; callq *(%rax)`） |
| `OdDbModelerGeometryImpl::dwgOutFields` | `0x8b8380` | 写侧，可交叉印证 |
| `OdDbAcisIO::readAcisData` | `0x8b622a` | SAT/SAB 读取（含 data-store 变体） |

**经验证据（对拍语料 `OpenRL_PointLight.dwg` AC1027 的 4 条 3DSOLID）**：
`common.BinaryChain=1`（`has AcDs binary data` 置位）；`in_data_store=true` 分支下：

| handle | 结果 |
|---|---|
| `0x201` | 解码成功但游标越过边界 **+64 bit**（dataEnd=3093，落 3157） |
| `0x205` | +64 bit |
| `0x209` | +64 bit |
| `0x1fd` | 位数耗尽（`wanted 8 bits, 0 remain`） |

即 3/4 条恰好**多读 64 bit**（= 1 个 RD 或 8 字节）——与 Rust 在本语料**同为 error**，
印证 Rust 模块文档自述的开放问题（“data-store 块是否与 AcDs 位绑定、是否 R2013+
统一变更，语料无法判定”）。Rust 仅在其私有 R2018 样例验证过该布局。

**当前阻塞**：`OdDbModelerGeometryImpl::dwgInFields` 全量 Ghidra 分析在交互窗口内
未完成（模块 21 MB，C++ 异常清理使纯 `objdump` 手工还原不可靠）。

**下一步（专一会话）**：
1. `open_binary` 该模块后**按符号地址单独 `analyze_function 0x8b71a4`**（或延长分析预算）；
2. 或 `objdump` 提取 42 处 filer 虚表偏移序列，配合 E1 `DbFiler.h` 的 `OdDbDwgFiler`
   虚函数声明序（vtable 偏移→rdInt16/rdInt32/rdDouble/rdString…）重建字段序；
3. 用 `dwgOutFields`（`0x8b8380`）交叉印证，再在 Go 侧改 `modeler.go` 使 4 条记录
   delta=0，并纳入对拍。

### B1.3 — MATERIAL 侦察（`libTD_DbCore.so`）——**深层嵌套，暂缓**

符号（ELF x64 未 strip）：
| 符号 | 地址 | 大小 | 备注 |
|---|---|---|---|
| `OdDbMaterial::dwgInFields` | `0xa071fa` | 166 B | 委托 Impl |
| `OdDbMaterialImpl::dwgInFields` | `0xa0b840` | 884 B | 顶层 |
| `OdDbMaterialImpl::rdColor` | `0xa09a42` | 182 B | `RC method, BD factor, [method==1] BL color`（**条件字段**） |
| `OdDbMaterialImpl::rdMap` | `0xa0b0c2` | 304 B | `BD blend, rdMapper, RC source, [source∈{1,2}] TV file, [..] rdTexture` |
| `OdDbMaterialImpl::rdMapper` | `0xa0ae9c` | 288 B | `RC proj, RC uTiling, RC vTiling, RC autoTransform, [..] rdMatrix3d` |
| `OdDbMaterialImpl::rdMatrix3d` | （嵌套） | — | 矩阵 |
| `OdDbMaterialImpl::rdTexture` | `0xa09bb4` | 1724 B | 贴图 |
| `OdDbMaterialImpl::rdAdvMaterial` | `0xa192ea` | 476 B | `B BD×4 B BS BD B BS BS` |

顶层 filer 调用序列（vtable 偏移法）：
```
TV name, TV description,
rdColor, rdColor, rdMap, rdColor, rdMap, BD, rdMap, BD, rdMap, rdMap, BD, rdMap,
BD×4, BL×3, rdAdvMaterial
```
结论：MATERIAL 是**递归嵌套**记录（rdMap→rdMapper→rdMatrix3d；另有 rdTexture），带多个
条件字段（依 method/source）。忠实重建需约 5 个嵌套 helper 逐一反编译——这正是参考
实现明确「故意不派发」的原因。**已回退**此半成品解码（Go 与 Rust 对 MATERIAL 同为
Unhandled），留待专一会话。侦察结论与地址已完整记录。

### B1.4 — 其余自定义类

| 类 | 语料对象 | 符号（模块） | 结构 |
|---|---|---|---|
| SOLID_BACKGROUND | 0（仅类表 num=1） | `OdDbSolidBackground::dwgInFields` `SCENEOE.tx:0x6fcca` | **单个 `rdInt32`**（已实现 `dwg/solid_background.go`；本语料无对象） |
| SUN | 1 (`CUSTOM(528)`) | `OdDbSun::dwgInFields` `SCENEOE.tx:0x8f542` | ✅ **已实现** `dwg/sun.go`（`BL, B, CmColor, BD, B, Date(BL,BL), B, BL, BS, RC`），Go 解码 Rust 未处理 |
| LIGHT | 1 (`CUSTOM(525)`) | `OdDbLightImpl::dwgInFields` `SCENEOE.tx:0x5df98` | ✅ **已实现** `dwg/light.go`。`LIGHT extends OdDbEntity` → 走**实体公共前导**（§19.4.1）而非对象前缀；体 `BL,TV,BL,B,CmColor,B,BD,3d×2,BL,B,BD×4,B,BL,BS,RC`，末尾 `[filerType()!=0]` 门控块在文件读路径（`kFileFiler=0`）**不出现**。Go 解码 Rust 未处理的 1 条，`name="Pointlight1"`，`delta=0` |
| SECTION/DETAILVIEWSTYLE | 2 | `TD_DbEntities.tx`：`OdDbSection::dwgInFields` `0xa830ee`、`OdDbSectionManager` `0xd1a038`、`OdDbSectionSettings` `0xa92efa`、`OdDbDetailViewStyle` `0xd8b98e`、`OdDbSectionViewStyle` `0xd905b2`（另有 `AcModelDocObj.tx` `OdDbSectionSymbol` `0x2992f0`） | 同法 |
| LAYOUT | 3 | `OdDbLayout::dwgInFields` `libTD_DbCore.so:0xa2f040`、基类 `OdDbPlotSettings::dwgInFields` `0x9e265c` + `OdDbPlotSettingsImpl::dwgInFields` `0x9e2a5e` | 见 B1.2 侦察 |

### B1.2 — LAYOUT / PLOTSETTINGS ✅ **已收口（移植 + 真机验证）**

**关键发现**：Rust 参考**已实现** LAYOUT 与 PLOTSETTINGS（`src/objects/acad_layout.rs`、
`acad_plot_settings.rs`，含完整单测），但在这两份真实 ODA 语料上（AC1018 / AC1027）
全部落入 Undecoded（`entity_values` 无 LAYOUT）。因此这项是**移植 + 真机收口**，
而非从零逆向。

**移植**：`dwg/plot_settings.go`（共享块 `readPlotSettingsFields` + `AcadPlotSettings`）
与 `dwg/layout.go`（`decodeLayoutObject`）。复用既有 `openObjectStream`
（`data`/`strings`/`hasDSBinaryData`/`readTV`/`finish`）。字段序：

- LAYOUT：`[hasDS] 2×RC ds_block` → **plot-settings 块** → `TV layout_name`,
  `BL tab_order`, `BS flags`, `3BD ucs_origin`, `2RD limmin`, `2RD limmax`,
  `3BD insertion_point`, `3BD ucs_x_axis`, `3BD ucs_y_axis`, `BD elevation`,
  `BS ucs_ortho_view_type`, `3BD extent_min`, `3BD extent_max`, `BL viewport_count`。
  注：`limmin/limmax` 是 **RD（原始 64 位）**，其余点是 **BD**；`viewport_count` 为
  **BL**（非规范所印 RL，Rust 已实测）。
- PLOTSETTINGS 块：`TV×2` → `BS flags` → `BD×6`（4 边距 + 纸宽高）→ `TV paper_size`
  → `2BD origin` → `BS×3` → `2BD window_min/max` → `BD×2` → `TV style_sheet` → `BS`
  → `BD scale_factor` → `2BD image_origin` → `BS×3`（R2004+ shade）。
- 版本门控同 Rust：R2004/R2010/R2013/R2018 支持；R2007 返回 Unsupported（定位不到
  字符串流）；R13/R14/R2000 未定。

**真机结果**：AC1018 与 AC1027 **各 3 条 LAYOUT 全部收口**（`finish` 边界校验通过、
无 ERROR；Go 投影 40→43 / 157→166）。独立佐证：`paper_width×paper_height` 与字符串流
里的 `paper_size` 完全吻合（`ISO_A4_(210.00_x_297.00_MM)`→210×297、
`ANSI_A_(8.50_x_11.00_Inches)`→215.9×279.4）；Model 记录 `lim=(0,0)..(12,9)`、
`ucsX=(1,0,0)`、`ucsY=(0,1,0)`、`ds_block=[0x2C,0x0B]`、scale=0.386863 —— 与 Rust 文档
实测值逐项一致。`dwg/classdispatch.go` 另挂 PLOTSETTINGS 自定义类分发（本语料无对象）。

嵌套子记录（已定位，待逐个反编译）：
- `OdCmColor::dwgIn` `libTD_DbCore.so:0x828ba0`（198 B，含 `dwgInAsTrueColor` 分支 +
  `fixColorIndex`——颜色方法条件字段）；
- `OdDbDate::dwgIn` `libTD_DbCore.so:0x792bee`（112 B：`rdInt32, rdInt32`）；
- `OdDbLightImpl::rdPhotometricData` `SCENEOE.tx:0x62716`（1152 B，多版本分支，待做）。

结论：SUN 已实现（`dwg/sun.go`，复用 §2.11 CMC + `OdDbDate::dwgIn`）；LIGHT
**已收口**（见下）；MATERIAL 同前暂缓。所有地址/字段骨架已记录，均可按同一
vtable 方法续接。

**LIGHT 收口（根因）**：`OdDbLight::dwgInFields`（公开入口 `SCENEOE.tx:0x5df3a`）
调用的是 `OdDbEntity::dwgInFields` 而**非** `OdDbObject::dwgInFields` —— LIGHT 是
**实体**。此前用对象公共前缀（`openObjectStream`）读，字段游标整个错位；name 只因
取自**字符串流**（与数据游标无关）而偶合正确。改用实体路径（`BodyCursor` +
`ReadCommonEntityData` + `entityDataEnd` 边界校验）后：

- 顶层 `BL,TV,BL,B,CmColor,B,BD,3d×2,BL,B,BD×4,B,BL,BS,RC` 全部对齐；
- 之前残余的 ~349 bit 正是**实体公共前导**（entity mode、颜色、线型、线宽…）被对象
  前缀漏掉的量；
- 末段 `if(filerType()!=0){B; if(B) rdPhotometricData}` 在文件读路径
  （filer 类型 `kFileFiler=0`）**整块不存在** → 少读门控 `B` 即 `delta=1` → 去掉后
  `delta=0`，`dataEnd=837` 精确收口。

`rdPhotometricData`（`SCENEOE.tx:0x62716`）为其它 filer（copy/undo）路径，读文件时
不触发；已在 `dwg/light.go:readLightPhotometricData` 保留实现备查但不在文件路径调用。
**已回退 → 重新以实体路径实现**，对拍全绿（LIGHT 计入 Route-B 额外项）。

## B1 完成度

- ✅ B1.1 3DSOLID/REGION/BODY（已修复，见下，Go 解码 Rust 拒绝的 4 条）。
- ✅ B1.2 LAYOUT / PLOTSETTINGS（已收口，Go 每语料解码 3 条，见下）。
- ✅ B1.4 SUN + **LIGHT**（已实现，Go 各解码 Rust 拒绝的 1 条）。
- ⏳ B1.3 MATERIAL（深层嵌套 `rdMap→rdMapper`）、B1.4 其余（SECTION/DETAILVIEWSTYLE）：已完成**符号级侦察**，待嵌套子记录反编译。

**关键教训（可复用）**：自定义类的**基类**决定公共数据前导——`OdDbEntity` 子类
（如 LIGHT）必须走实体前导，`OdDbObject` 子类（如 SUN）才走对象前缀；公开
`<Class>::dwgInFields` 的第一句 `call` 目标即为判定依据。

### B1.1 结果：已修复（vtable 解析法）

Ghidra 21MB 全量分析超时，改用**「重定位解析 vtable → 偏移映射方法名」**法（无需
反编译器）：

1. `nm`/`llvm-readelf -r` 取 `_ZTV13OdR12DwgFiler`（`OdR12DwgFiler : OdFlatFiler`）
   的虚表槽，relocation 目标解析为符号名；
2. 得到完整偏移→方法表（`0xa8=rdBool`、`0xb0=rdString`、`0xd8=rdInt32`、
   `0xf0=rdDouble`、`0x100=rdDbHandle`、`0x130=rdPoint3d`…）；
3. 在 `OdDbModelerGeometryImpl::dwgInFields` 里抽取 filer 虚调用偏移序列，
   结合逐字段游标追踪（Go 侧临时 trace）**定位到 R2013 data-store 尾部**：
   `envelope → wireframe/isolines → acis_empty2 → R2007+ BL → data-store marker`
   之后，修订字段 **R2013 = 8 字节**、R2018 = 16 字节。

**修复**：`dwg/modeler.go` 的 `revisionBytes(version)`（R2018→16，其余→8）。4 条
3DSOLID 全部 `delta=0`；Go `Decoded` 157→161、`Errored` 4→0——**解码了 Rust 拒绝
的 4 条记录**。对拍测试改为「预言机结果须为 Go 结果的子序列，且 Go 多出的项只能是
modeler 类型」，以容纳此路线 B 改进。

**可复用方法**（后续 MAT/LAYOUT/… 同类任务）：`_ZTV<Class>` + `.rela.dyn`
→ 偏移→`rd*` 方法；Go 侧逐字段游标 trace 定界。

### B0 — 路线 A 补遗：预言机漏投影的已实现类型（**已完成**）

在转 Route B 之前，用一次性 Rust 探针（遍历 `all_objects` 打印参考 `DecodedEntity`
变体）核实「Rust 已实现、但预言机未投影、Go 未移植」的类型。本语料命中三类：

| 类型 | Rust 源 | Go 源 | 证据 |
|---|---|---|---|
| VPORT | `tables/vport.rs` | `dwg/table_vport.go` | AC1018/AC1027 各 1 条逐位一致 |
| HATCH | `entities/hatch.rs` | `dwg/hatch.go` | AC1027 1 条规范签名逐字段一致 |
| SECTIONVIEWSTYLE | `objects/acad_section_view_style.rs` | `dwg/section_view_style.go` | AC1027 1 条全字段一致 |

要点：预言机 `entity_value_json` 的 `_ => return None` 回退会静默丢弃未投影类型，
两侧投影须**同时**补齐（否则 Go 多出项会被判定为非 Route-B 额外项而失败）。HATCH
用两侧一致的规范签名串比对（浮点取位型）；VPORT/SECTIONVIEWSTYLE 用命名/有序字段。

**教训**：对拍覆盖率不仅取决于 Go 缺什么，也取决于**预言机投影了什么**；定期用
探针核对「参考能解码」全集，避免把 Route A 缺口误当已覆盖。
