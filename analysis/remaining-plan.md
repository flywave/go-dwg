# 剩余未解内容 — 计划与执行台账

本文件穷举两份 ODA 真实语料（AC1018 `OdWriteEx XRef.dwg`、AC1027
`OpenRL_PointLight.dwg`）与合成语料上**仍未解码**的对象，按「参考是否已实现」
分为路线 A（增量移植 Rust）与路线 B（RAE + ODA 静态分析补 Rust 的缺口），
逐项给出证据链、验收判据与执行状态。

判据统一为：**解码字段恰好落在记录的数据流边界**（`finish`/`checkedInline`
的 `delta=0`）——这是本格式自带的、不依赖参考的可验证锚点；能进对拍的再加
「与 Rust 预言机逐字段一致」。

## 0. 侦察方法（本轮新增，穷举用）

- **参考探针**（`tools/dwgref/examples/probe_decode.rs`，已入库复用）：遍历 `all_objects`，
  对每个对象打印参考 `DecodedEntity` 变体。用于发现「Rust 已实现、但预言机
  `entity_value_json` 未投影、Go 也未移植」的**隐形路线 A 缺口**——光看对拍输出
  发现不了（例：VPORT/ELLIPSE/MLINESTYLE）。
- **符号定位**：`nm [-S] <module> | c++filt | grep dwgInFields` 取
  `Od<Class>::dwgInFields` / `Od<Class>Impl::dwgInFields` 地址与大小。
- **字段序提取（无 Ghidra）**：`objdump -d` 该地址，抽取对 filer 虚表的
  `addq $off,%rax; ...; callq *%rax` 调用序列，偏移经既有
  `_ZTV13OdR12DwgFiler` 的 `.rela.dyn` 映射表 → `rd*` 方法名。已验证偏移跨
  OdDbDwgFiler 派生类一致。

## 1. 路线 A 收尾（参考能解码、补齐 Go）— **已完成**

| 类型 | 语料 | Go 源 | 状态 |
|---|---|---|---|
| VPORT | AC1018/AC1027 各 1 | `dwg/table_vport.go` | ✅ 逐位一致 |
| HATCH | AC1027 1 | `dwg/hatch.go` | ✅ 签名逐字段一致 |
| SECTIONVIEWSTYLE | AC1027 1 | `dwg/section_view_style.go` | ✅ 全字段一致 |
| ELLIPSE | AC1018 1 | `dwg/ellipse.go` | ✅ 逐位一致 |
| MLINESTYLE | AC1018/AC1027 各 1 | `dwg/mline_style.go` | ✅ 逐字段一致 |

路线 A 在**当前语料上已穷尽**（探针核对：Rust 能解码的其余类型 Go 均已覆盖）。

## 2. 路线 B  —— 剩余清单（Rust 也未解码/报错）

| # | 类（DXF） | 语料对象 | 符号（模块） | 计划/状态 |
|---|---|---|---|---|
| B-1 | SORTENTSTABLE | AC1027 1 | `OdDbSortentsTableImpl::dwgInFields` `libTD_DbCore.so:0x96eb90`（238 B） | ✅ **完成** `dwg/sortents_table.go`：`BL count` + `count×H`，7 条句柄恰为绘制序实体集，`delta=0` |
| B-2 | MTEXT | AC1018 1 | `OdDbMTextImpl::dwgInFields` `TD_DbEntities.tx:0x8f357e` | ✅ **完成** `dwg/mtext.go`：Rust 内联解码漏读 R2004+ `background_flags`（清零为 2-bit `BL`），故 delta −2；补齐后 `delta=0`，`text="{\\fComic Sans MS…External Reference}"` |
| B-3 | PLANESURFACE | AC1027 1 | `OdDbPlaneSurface::dwgInFields` `TD_DbEntities.tx:0x9e7ec6`→`OdDbSurface::dwgInFields` `0x9c90e4` | ✅ **完成** `dwg/classdispatch.go:decodeSurfaceEntity`：实体前导 + `decodeModelerRecord` + 2×`BS`；`delta=0` |
| B-4 | DETAILVIEWSTYLE | AC1027 1 | `OdDbDetailViewStyle::dwgInFields` `TD_DbEntities.tx:0xd8b98e`（0x4ba）→ 基类 `OdDbModelDocViewStyle::dwgInFields` `0xd899ce` | ✅ **完成** `dwg/detail_view_style.go`：旧「矛盾」是 bodyStart 误算 + 句柄槽被漏标所致。主体直线无分支，基类(BS ver, TV name, B) + 派生 28 字段（BS/BL/H/CMC/BD/TV/RC 交错，6 个软指针句柄占 0 数据位），`delta=0`；StringB = 视图标签模板 `%<\AcVar ViewDetailId>%…` 佐证 |
| B-5 | MATERIAL | AC1027 7 | `OdDbMaterialImpl::dwgInFields` `libTD_DbCore.so:0xa0b840`（+ `rdColor`/`rdMap`/`rdMapper`/`rdMatrix3d`） | ✅ **完成** `dwg/material.go`：顶层 + 3 嵌套 helper，`source==2` 纹理映射明确报错；7 条 `delta=0`，名称 ByLayer/ByBlock/Global/Material1–4 |
| B-6 | TABLESTYLE | AC1018/AC1027 各 1 | `OdDbTableStyle::dwgInFields` `TD_DbEntities.tx:0xd44fd8`（2021 ELF）；25.4 PE DLL 导出 `?dwgInFields@OdDbTableStyle@@` @RVA 0x3c5360 | ✅ **完成** `dwg/tablestyle.go`：≤R2007 分支 A（TV,BS,BS,BD,BD,B,B + 3×方向块{BD,BS,CMC,CMC,B,6×{BS,B,CMC}}，R2007 另补 2BL+TV）与 R2010+ 分支 B（RC,TV,BL,BL,H + **完整 OdCellStyle**（25.4 版文件路径做全量读取：BL,BS,BL,BL,CMC,5BL,TV,2BD,BL,CMC,H,BD,BS,6BD,BL 网格数 + n×网格线）+ BL,BL,TV + count×{type,cell,key1,key2,TV}）。2021 版读法对新版记录**欠读**，以 25.4 版全量文法收口；两语料均 `delta=0`（AC1018 边距 0.18/0.25/0.18） |
| B-7 | EVALUATION_GRAPH | AC1027 4 | `OdDbEvalGraph::dwgInFields` `TD_DynBlocks.tx:0x1d8c3c` | ✅ **完成** `dwg/eval_graph.go`：`BL,BL,BL(n),n×7×BL,BL(m),m×10×BL`（节点 H 在句柄流）；4 条 `delta=0` |
| B-8 | SH_HISTORY | AC1027 4 | `OdDbShHistory::dwgInFields` `TD_3DSolidHistory.tx:0xb77e8`（Impl `0xb82bc`） | ✅ **完成** `dwg/sh_history.go`：`BL,BL,BL,B,B`（H 在句柄流，0 bit）；4 条 `delta=0` |
| B-9 | ShSphere/Cylinder/Cone/Wedge | AC1027 各 1 | `TD_3DSolidHistory.tx`：ShSphere `0xd1018`(Impl `0xd11d4`)、ShCylinder `0xb175e`(`0xb1a82`)、ShCone `0xb2368`(复用 CylinderImpl)、ShWedge `0xd50ec`(`0xd53bc`) | ✅ **完成** `dwg/sh_primitive.go`：`OdDbEvalExpr::dwgInFields`（gc=-1 路径）= `BL,BL,BL,BS` + 结尾再 `BL`（此前漏标），变体按 `OdDxfCode::_getType` 三张 .rodata 映射表分派（gc=-1 → 14 → 不读）；`ShHistoryNodeImpl` = `BL,BL,16×BD(矩阵),CMC,BL`（句柄在句柄流）；Impl 尾 Sphere `BL,BL,BD`、Cylinder/Cone `BL,BL,4×BD`、Wedge `BL,BL,3×BD`。四条 `delta=0` |
| B-10 | MENTALRAYRENDERSETTINGS | AC1027 1 | `OdDbMentalRayRenderSettings::dwgInFields` `SCENEOE.tx:0x81d46` + 基类 `OdDbRenderSettings::dwgInFields` `0x7f45a` | ✅ **完成** `dwg/mental_ray.go`：基类块 + 45 字段序列；`delta=0` |

> 说明：B-7..B-10 的 `dwgInFields` 未出现在 `TD_DbEntities.tx` / `libTD_DbCore.so`；
> 下一步在全部 `.tx` 里 `nm | grep` 定位模块（可能是 `ModelerGeometry.tx` /
> `AcModelDocObj.tx` 等），或这些类以 `ImplementObject`/内联方式注册。

## 3. 非语料缺口（无对拍样本）

- **其余实体**：✅ 已移植 TEXT/INSERT/LWPOLYLINE/SPLINE/DIMENSION 七子类/
  RAY/XLINE/SOLID/TRACE（`dwg/simple_entities.go`、`lwpolyline.go`、
  `spline.go`、`dimension.go`），以合成位流单测验证（镜像 Rust 测试向量）。
  MTEXT 早已在语料上收口。VIEW/UCS 表项亦已移植（`dwg/table_view_ucs.go`，
  合成位流单测）。
  **本轮续**：剩余 `entities/` 全量移植完成——VIEWPORT/3DFACE/LEADER/
  TOLERANCE/POLYLINE_2D/POLYLINE_3D/VERTEX 四变体/PFACE_FACE/POLYFACE_MESH/
  POLYGON_MESH/OLE2FRAME/ATTRIB/ATTDEF（共享 TEXT 字段 + R2010+ 属性体，
  R2018 多行变体含内嵌 MTEXT）/HELIX/IMAGE/WIPEOUT/IMAGEDEF/UNDERLAY/
  CAMERA/GEODATA/MESH/EXTRUDED·REVOLVED·SWEPT·LOFTED SURFACE/MLINE/
  MLEADER（嵌套 AnnotContext 全量：leader root/line/断点对/text·block
  content）。全部带合成位流单测（`dwg/entities_extra_test.go` 等）；
  MLEADER 本语料无实体记录，解码器已接入待真实文件验证。
  Rust `entities/` 至此全部覆盖。
- **R2007 端到端**：缺真实 AC1021 样例。

## 3.5 vtable 偏移映射校正（重要）

原始笔记的「Filer vtable map」来自 `_ZTV13OdR12DwgFiler`（TD_DbIO.tx）。实测该
vtable 与 SCENEOE 中读 SUN 时所用的**真实读 filer** 并不一致（R12 类插入/覆盖了
不同槽位）。以 SUN 的已知字段序 `BL,B,CmColor,BD,B,Date,BL,BS,RC` 反标定得到
真实读 filer 的偏移：

| off | 方法 | off | 方法 |
|---|---|---|---|
| 0xa8 | rdBool | 0xd0 | rdInt16 |
| 0xc8 | rdUInt8 | 0xd8 | rdInt32 |
| 0xf0 | rdDouble | 0x100 | rdDbHandle |

**纪律**：每个新类先用「一个已知正确记录的字段序」反标定偏移，再据以复原 vtable
调用序；不要直接套用 R12 vtable 的绝对偏移。

**跨模块一致性**：`TD_DbEntities.tx` 中 `OdDbMTextImpl::dwgInFields` 用 `0x130`
(rdPoint3d)、`0x140` (rdVector3d)、`0xf0` (rdDouble)，与本表一致 → 各模块复用同一
`OdDbDwgFiler` 偏移表。故 B-4 DETAILVIEWSTYLE 的「字段过宽」矛盾**不是** vtable
映射问题，确为函数内条件/宽度分支，需反编译器。

## 4. 执行纪律

- 每个 B 项：先 `objdump`/vtable 定字段序 → Go 实现 → 边界 `delta=0` 自检 →
  加 `isRouteBExtra`（Rust 不解码者）→ 全绿后合入。
- 无法收口（字段不确定）者**回退**，保持 Go 与 Rust 同为 Unhandled/ERROR。
