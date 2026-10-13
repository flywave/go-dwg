# 端口进度与对拍结果

## 已完成：容器层（对拍逐字节一致）

| 模块 | Go 文件 | 对应 Rust | 对拍维度 |
|---|---|---|---|
| 版本识别 | `dwg/version.go` | `version.rs` | magic→版本、family 谓词 |
| 类型化错误 | `dwg/error.go` | `error.rs` | — |
| R2004+ XOR / Sec_Mask | `dwg/cipher.go` | `cipher.rs` | 108B magic、页掩码 |
| CRC-8 / CRC-32 / 页校验 | `dwg/crc.go` | `crc.rs` | 标准测试向量 |
| 位流原语 | `dwg/bitcursor.go` | `bitcursor.rs` | §2.2/§2.3/§2.5 规范样例 |
| LZ77 | `dwg/lz77.go` | `lz77.rs` | 字面量/回绕/0x40 族样例 |
| Reed-Solomon FEC | `dwg/reed_solomon.go` | `reed_solomon.rs` | GF(256) 结构 |
| R2007 LZ (§5.10) | `dwg/r21_lz.go` | `r21_lz.rs` | 置换表 + line_2007 头向量 |
| 文件头 | `dwg/header.go` | `header.rs` | 字段值 |
| 命名 section | `dwg/section.go` | `section.rs` | 名称/类型/偏移/大小 |
| 页映射 / section info | `dwg/section_map.go` | `section_map.rs` | 页项 + 描述 |
| 句柄映射 | `dwg/handle_map.go` | `handle_map.rs` | 条目数 + 前 N 条目 |
| 类表 | `dwg/classes.go` | `classes.rs` | 全字段 |
| 元数据 | `dwg/metadata.go` | `metadata.rs` | 全部字段 |
| Header 变量 | `dwg/header_vars.go` | `header_vars.rs` | sentinel/size/body |
| 对象类型 | `dwg/object_type.go` | `object_type.rs` | label/分类 |
| 对象 walker | `dwg/object.go` | `object.rs` | off/size/type/handle/raw |
| 字符串流定位 | `dwg/string_stream.go` | `string_stream.rs` | §19.1 trailer/边界 |
| 通用实体前缀 | `dwg/common_entity.go` | `common_entity.rs` | 字段游标 |
| 实体解码+分发 | `dwg/entity.go` | `entities/{line,circle,arc,point,block,endblk,seqend}.rs` | **字段 bit 级一致** |
| 通用对象数据 | `dwg/object_common.go` | `objects/modern.rs` | 边界自检 |
| DICTIONARY/控制/占位 | `dwg/{dictionary,control,placeholder}.go` | `objects/*.rs` | 键/计数一致 |
| XRECORD | `dwg/xrecord.go` | `objects/xrecord.rs` | 体字节/标志一致 |
| 表项 APPID/LAYER/LTYPE/STYLE | `dwg/{tables,split_stream,table_appid,table_layer,table_ltype,table_style}.go` | `tables/{mod,modern,appid,layer,ltype,style}.rs` | 名称/字段一致 |
| R2007 容器 | `dwg/r2007.go` | `r2007.rs` | RS 页/页映射/section map 向量 |
| 顶层 API | `dwg/reader.go` | `reader.rs` | 摘要 + 状态 |

## 对拍语料与结果

预言机：`tools/dwgref`（Rust，`--dump-sections` 落盘解压字节）。
Go 测试：`dwg/differential_test.go`。

| 语料 | 版本 | 结果 |
|---|---|---|
| `synthetic_2000.dwg` | AC1015 | ✅ 全项一致 |
| `synthetic_2004.dwg` | AC1018 | ✅ 全项一致 |
| `synthetic_2010.dwg` | AC1024 | ✅ 全项一致 |
| `synthetic_2013.dwg` | AC1027 | ✅ 全项一致 |
| `synthetic_2018.dwg` | AC1032 | ✅ 全项一致 |
| `synthetic_r14.dwg` | AC1014 | ✅ 全项一致 |
| `OdWriteEx XRef.dwg`（ODA 官方） | AC1018 | ✅ 含 **section 字节逐字节一致** |
| `OpenRL_PointLight.dwg`（ODA 官方） | AC1027 | ✅ 含 **section 字节逐字节一致** |

判定维度：magic/version/file_size/section_map_status、sections 列表、
**每个 section 的反解字节（bytes.Equal）**、handle map（条目数 + 前 64）、
class map（全字段）、header vars、顺序对象计数、句柄驱动对象清单
（off/size/type/label/handle/raw_len）、walk 汇总、summary/app info。

## R2007（AC1021）验证

无真实 AC1021 样例（Rust 语料亦无）。以 Rust 测试模块的**精确向量**在
`dwg/r2007_test.go` 复刻验证，全部通过：

- `system_page_size`（0/304→0x400，2132→0x1200）；
- 反交织 `deinterleave`；
- 页映射偏移累加 + 负 id（空闲页）按绝对 id 解析；
- section map 名长含终止符（`AcDb:Handles` → 页表起点正确）；
- §5.10 字节置换表 2/4/8/11/16/32 字节块；
- `line_2007.dwg` 文件头前缀 → `HeaderSize=0x70`、`FileSize=66304`；
- 截断/输出上限/空输入错误路径。

另 `dwg/reader_test.go` 验证：容器解析失败时 R2007 文件落到 `Deferred`
占位 section 列表且不 panic（与 Rust 回退行为一致）。

## 实体解码对拍（P1 首片）

已移植 LINE / CIRCLE / ARC / POINT 解码，并经 `common_entity`（§19.4.1）
通用前缀与 `string_stream`（§19.1）边界自检。预言机输出每条实体的**字段
IEEE-754 位型**（`entity_values`），Go 侧逐字段比对：

| 语料 | 版本 | 实体 | 结果 |
|---|---|---|---|
| synthetic_2000 | AC1015 | 2×LINE/CIRCLE/ARC/POINT | ✅ bit 级一致 |
| synthetic_2004/2010/2013/2018 | AC1018/24/27/32 | 同上 | ✅ |
| synthetic_r14 | AC1014 | 同上（3BD 布局） | ✅ |
| OpenRL_PointLight.dwg（ODA） | AC1027 | CIRCLE | ✅ |

判定：`start/end/center/radius/thickness/extrusion/start_angle/end_angle` 等
全部字段的 f64 位型逐一相等；边界自检（数据字段恰好落在字符串流起点或
`RL` 对象数据边界）不允许"看似合理"的错位结果。

## 非实体对象对拍（P1 第二批）

移植共享对象流（`objects/modern` 的 `open/finish/read_tv`）与 DICTIONARY /
`*_CONTROL` / ACDB_PLACEHOLDER。预言机经 **no-class-map 的
`decode_from_raw`** 投影这些对象（与 Go 语义一致；custom class 两侧都不解析），
逐字段比对：

| 语料 | 版本 | 新增解码对象 | 结果 |
|---|---|---|---|
| OdWriteEx XRef.dwg（ODA） | AC1018 | 9 DICTIONARY + 9 CONTROL + 1 PLACEHOLDER | ✅ 键/计数逐一一致 |
| OpenRL_PointLight.dwg（ODA） | AC1027 | 27 DICTIONARY + 9 CONTROL + 1 PLACEHOLDER | ✅ |

DICTIONARY 按 §19.5.19 读 `BL numitems` / `BS cloning` / `RC hardowner` /
`TV×N` 名称块（值句柄在对象句柄流，不在数据流）；R2007+ 名称取自 §19.1
字符串流。控制对象按 §19.5.1-10 读 `BL numentries`，DIMSTYLE_CONTROL 另读
一个 `RC`（R14 无）。所有路径以数据流边界自检收口。

## 表项与 XRECORD 对拍（P1 第三批）

| 类型 | Rust 源 | Go 源 | 对拍 |
|---|---|---|---|
| XRECORD | `objects/xrecord.rs` | `dwg/xrecord.go` | `data` 字节 + cloning 标志 |
| APPID | `tables/appid.rs` | `dwg/table_appid.go` | 名称（inline + R2007+ split-stream） |
| LAYER | `tables/layer.rs` | `dwg/table_layer.go` | 名称/flags/lineweight/color |
| LTYPE | `tables/ltype.rs` | `dwg/table_ltype.go` | 名称/描述/pattern/alignment/dash 数 |
| STYLE | `tables/style.rs` | `dwg/table_style.go` | 名称/flags/fixed_height/font/bigfont |
| BLOCK/ENDBLK/SEQEND | `entities/{block,endblk,seqend}.rs` | `dwg/block.go` | 名称/空体 + 边界 |
| BLOCK_HEADER | `tables/block_record.rs` | `dwg/block_record.go` | name/base/xref/desc + 哨兵句柄 |
| 3DSOLID/BODY/REGION | `entities/modeler.rs` | `dwg/modeler.go` | **路线 B 修复**（见下） |
| SUN | `objects/acad_sun.rs` | `dwg/sun.go` | **路线 B**：Go 解码 Rust 拒绝的 1 条 |
| LIGHT | `objects/acad_light.rs` | `dwg/light.go` | **路线 B**：实体基类（见下），Go 解码 1 条 |
| LAYOUT | `objects/acad_layout.rs` | `dwg/layout.go` | **路线 B**：移植 + 真机收口，Go 每语料解码 3 条 |
| PLOTSETTINGS | `objects/acad_plot_settings.rs` | `dwg/plot_settings.go` | **路线 B**：LAYOUT 内嵌块，同一 `readPlotSettingsFields` |
| SOLID_BACKGROUND | — | `dwg/solid_background.go` | 路线 B（本语料无对象） |
| VPORT | `tables/vport.rs` | `dwg/table_vport.go` | 路线 A：20 字段逐位一致（AC1018/AC1027 各 1 条 `*Active`） |
| HATCH | `entities/hatch.rs` | `dwg/hatch.go` | 路线 A：规范签名逐字段一致（AC1027 1 条，含梯度块 + 弧边界 + 种子点） |
| SECTIONVIEWSTYLE | `objects/acad_section_view_style.rs` | `dwg/section_view_style.go` | 路线 A：全字段一致（AC1027 1 条，34 字段） |

## 路线 B 首个成果（3DSOLID §20.4.41）

Rust 在 ODA R2013 语料对 4 条 3DSOLID 报 error（+64 bit）。用「vtable 重定位解析 →
filer 偏移映射方法名 + Go 逐字段 trace」定位：R2013 的 data-store 修订字段为
**8 字节**（R2018 为 16）。修复 `revisionBytes()` 后 4 条 `delta=0`：

- Go `Decoded` 157→161、`Errored` 4→0，**解码了 Rust 拒绝的记录**；
- 对拍测试改为「Rust 结果 ⊆ Go 结果，Go 多出项仅限 modeler 类型」。
- 方法可复用于 MAT/LAYOUT 等同类缺口（`analysis/gap-fill-plan.md` B1.1）。

## 路线 B 续：SUN + LIGHT（自定义类）

| 语料 | 版本 | Rust 投影 | Go 投影 | 差值（Route-B 额外项） |
|---|---|---|---|---|
| OdWriteEx XRef.dwg | AC1018 | 43 | 47 | LAYOUT×3 · MTEXT×1 |
| OpenRL_PointLight.dwg | AC1027 | 161 | 188 | 3DSOLID×4 · SUN×1 · LIGHT×1 · LAYOUT×3 · SORTENTSTABLE×1 · PLANESURFACE×1 · SH_HISTORY×4 · EVALUATION_GRAPH×4 · MENTALRAYRENDERSETTINGS×1 · MATERIAL×7 |

（投影总数含其后补齐的路线 A 类型 VPORT/HATCH/SECTIONVIEWSTYLE；Rust 对这些记录
本可解码但预言机原先未投影，见下。）

- **SUN**（`OdDbObject` 子类）：`BL, B, CmColor, BD, B, Date(BL,BL), B, BL, BS, RC`，
  复用 §2.11 `OdCmColor` 与 `OdDbDate::dwgIn`。
- **LIGHT**（`OdDbEntity` 子类）：关键在**基类**——LIGHT 走实体公共前导（§19.4.1），
  而非对象前缀；体 `BL,TV,BL,B,CmColor,B,BD,3d×2,BL,B,BD×4,B,BL,BS,RC`，末尾
  `[filerType()!=0]` 门控块在文件读路径（`kFileFiler=0`）不出现。`name="Pointlight1"`、
  `delta=0`。
- **LAYOUT / PLOTSETTINGS**：Rust 已有完整实现（`acad_layout.rs`/`acad_plot_settings.rs`）
  但在两份真实 ODA 语料上全部落入 Undecoded；Go `dwg/layout.go` + `dwg/plot_settings.go`
  忠实移植，**收口成功**（`finish` 边界校验通过，无 ERROR）。两条独立佐证：
  `paper_width×paper_height` 与字符串流里的 `paper_size` 完全吻合
  （`ISO_A4_(210.00_x_297.00_MM)`→210×297、`ANSI_A_(8.50_x_11.00_Inches)`→215.9×279.4）；
  Model 记录 `lim=(0,0)..(12,9)`、`ucsX=(1,0,0)`、`ucsY=(0,1,0)`、scale=0.386863
  （与 Rust 文档实测值一致）。
- **可复用判据**：`<Class>::dwgInFields` 的第一句 `call` 目标是 `OdDbEntity` →
  用实体前导，是 `OdDbObject` → 用对象前缀。
| DIMSTYLE | `tables/dimstyle.rs` | `dwg/table_dimstyle.go` | 15 变量 + 全体消费边界 |
| 自定义类分发 | `entities/dispatch.rs` | `dwg/classdispatch.go` | class map → DXF 名 |
| DICTIONARYVAR/SCALE | `objects/{dictionary_var,acad_scale}.rs` | `dwg/classdispatch.go` | 字段一致 |
| VISUALSTYLE | `objects/acad_visual_style.rs` | `dwg/visual_style.go` | flag-less + R2010+ 配对/tail 一致 |
| MLEADERSTYLE | `objects/acad_mleader_style.rs` | `dwg/mleader_style.go` | 全字段一致（含 §2.11 CMC） |

ODA 真实图纸上的**已对拍解码对象数**（预言机 `entity_values` 投影）：

| 语料 | 版本 | 投影总数 | 构成 |
|---|---|---|---|
| OdWriteEx XRef.dwg | AC1018 | 41 | 9 CONTROL · 10 DICTIONARY · 3 LTYPE · 3 BLOCK_HEADER · 3 BLOCK · 3 ENDBLK · 3 DICTIONARYVAR · 2 LAYER · 1 STYLE · 1 APPID · 1 DIMSTYLE · 1 PLACEHOLDER · 1 VPORT |
| OpenRL_PointLight.dwg | AC1027 | 160 | 34 XRECORD · 28 DICTIONARY · 26 VISUALSTYLE · 17 SCALE · 11 APPID · 9 CONTROL · 9 DICTIONARYVAR · 3 LTYPE · 3 BLOCK_HEADER · 3 BLOCK · 3 ENDBLK · 3 DIMSTYLE · 2 MLEADERSTYLE · 2 LAYER · 2 STYLE · 1 PLACEHOLDER · 1 CIRCLE · 1 VPORT · 1 HATCH · 1 SECTIONVIEWSTYLE |

表项头（name + xref bits）与 R2007+ split-stream（字符串流取 name、数据字段
恰好落在字符串流起点）两个布局均已移植并对拍。

## 路线 A 续：VPORT / HATCH / SECTIONVIEWSTYLE（Rust 已实现、Go 补齐）

用一次性 Rust 探针（`dwgref/examples/probe_decode.rs`，已删）逐对象打印参考派发
结果，确定本语料上 **Rust 能解码、Go 尚未移植**的三类：`VPORT`（AC1018 + AC1027
各 1）、`HATCH`（AC1027 1）、`SECTIONVIEWSTYLE`（AC1027 1）。三者随即增量移植，
并把预言机 `entity_value_json` 与 Go 侧投影同时补齐，纳入逐字段对拍：

- **VPORT**（`dwg/table_vport.go`）：`tables/vport.rs` 忠实移植，inline（pre-R2007）
  与 R2007+ split-stream 两条路径；20 个字段（view height/width、center、target、
  direction、twist、lens、clips、view/render mode、screen rect、UCS/snap/grid）
  以 IEEE-754 位型逐一比对。加 `Read4bits`（VIEWMODE 4BITS）。边界 `finish` 收口。
- **HATCH**（`dwg/hatch.go`）：`entities/hatch.rs` 全量移植（梯度块、边界路径树
  折线/边列表 1–4、样条、图案线表、种子点），含 7 个数组防御上限。R2007+ 走实体
  字符串流取 pattern/gradient 名，`checkedInline` 边界自检（delta 0）。对拍用两
  侧一致的**规范签名串**（`dwg/hatchsig.go` ⇄ `main.rs:hatch_sig`，浮点取位型）。
  真机 1 条为梯度填充（2 色）+ 单条弧边界 + 1 种子点，逐字段一致。
- **SECTIONVIEWSTYLE**（`dwg/section_view_style.go`）：`acad_section_view_style.rs`
  移植（34 字段，R2018 头为可选）。复用 `openObjectStream` + `readObjectColor`。
  对拍以有序数组投影（ints/flags/floats/colors/strings）比对。

结果：AC1018 投影 40→41、AC1027 投影 157→160；Go `Decoded` AC1027 166→169，
`Unhandled` 28→25、`Errored` 0。全部差分测试通过。

> 注：Rust 本可解码这三类，但预言机原先在 `entity_value_json` 的回退分支丢弃它们
> （`_ => return None`），故此前未进入对拍；现两侧同补。

## 语料收口（本轮）

两份 ODA 真实语料上的**全部对象**现已解码（`dwg/decode_probe_test.go`，
`DWG_PROBE=1`）：AC1018 48 decoded / 0 unhandled / 0 errored；AC1027
194 decoded / 0 unhandled / 0 errored。本轮收口：

- **ShSphere/Cylinder/Cone/Wedge**（`dwg/sh_primitive.go`）：解开
  `OdDbEvalExpr` 的 gc=-1 路径（结尾再读一个 BL）与 `OdDxfCode::_getType`
  跳转表映射，`ShHistoryNodeImpl` 实为 BL,BL,16×BD,CMC,BL；四类 delta=0。
- **DETAILVIEWSTYLE**（`dwg/detail_view_style.go`）：旧「字段宽度矛盾」
  系 bodyStart 误算；直线主体 28 字段一次收口，delta=0，标签模板字符串佐证。
- **TABLESTYLE**（`dwg/tablestyle.go`）：双代分支；R2010+ 用 25.4 PE DLL
  反汇编出的完整 `OdCellStyle` 文件路径文法（2021 版欠读新版记录），
  双语料 delta=0。
- **无语料实体**（合成位流单测）：TEXT/INSERT/LWPOLYLINE/SPLINE/
  DIMENSION 七子类/RAY/XLINE/SOLID/TRACE 移植自 Rust 参考并加实现注释。

## 未完成（待 RAE + ODA SDK 补全）

- **其余实体解码**：✅ Rust `entities/` 全量移植完成（本轮：VIEWPORT/
  3DFACE/LEADER/TOLERANCE/POLYLINE 族/ATTRIB·ATTDEF/HELIX/IMAGE 族/
  UNDERLAY/CAMERA/GEODATA/MESH/四曲面/MLINE/MLEADER），全部带合成位流
  单测；MLEADER 待真实文件验证（本语料无实体记录）。
- **符号表项**：✅ VIEW/UCS 已移植；SECTION_SHAPE 等自定义类仍待。
- **Rust 覆盖不足处**：MLEADER 引线细节、完整 DIMSTYLE、实体图遍历——
  RAE + ODA SDK 目标（见 `gap-fill-plan.md`）。
- **R2007 端到端**：需一份真实 AC1021 样例做 section 字节对拍。

## 未知

- R2010+ 对象记录 `MS` 不含自身 `MC` 宽度的规则已按 Rust 实现移植并通过对拍，
  但合成样例未覆盖多字节 `MC`；待更多真实文件验证。
- R13-R15 类表无 `max_class_number` 记录，按到达即停读取；已通过对拍样例。
