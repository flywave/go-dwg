# go-dwg

纯 Go 的 Autodesk DWG 解析库——无 CGo、无外部依赖。从 DWG 容器（R13 → R2018 /
AC1032）中提取头部、命名 section、元数据、对象流与句柄映射。

本库以同目录参考实现 [`ref/dwg-rs-main`](ref/dwg-rs-main)（Apache-2.0 Rust
`dwg-rs`）为骨架对拍基准：**先参照 Rust 建立 Go 骨架并逐项对拍一致，再对
Rust 覆盖不足的部分用 RAE 结合 ODA SDK 补全**（见
[`analysis/`](analysis/README.md)）。

## 状态

| 层 | 状态 | 对拍 |
|---|---|---|
| 版本识别 AC1014 → AC1032 | ✅ | 与 Rust 一致 |
| R13–R15 平铺 locator 头 | ✅ | 与 Rust 一致 |
| R2004+ XOR 加密头 | ✅ | 与 Rust 一致（含 `AcFssFcAJMB` 校验） |
| Section Page Map + Section Info | ✅ | 与 Rust 一致 |
| Sec_Mask 页头解密 / 数据页 | ✅ | 与 Rust 一致 |
| LZ77 解压（§4.7，含 RLE 回绕） | ✅ | 与 Rust 一致 |
| CRC-8 / CRC-32 / 页校验和 | ✅ | 与 Rust 一致 |
| HandleMap / ClassMap | ✅ | 与 Rust 一致 |
| AcDb:Header 变量表 | ⚠ 原始位流 | 与 Rust 一致 |
| 元数据 SummaryInfo / AppInfo / Preview / FileDepList | ✅ | 与 Rust 一致 |
| 对象流 walker（顺序 + 句柄驱动） | ✅ | 与 Rust 一致 |
| R2007（AC1021）容器（RS + §5.10 LZ） | ✅ | 与 Rust 测试向量一致（无真实样例） |
| 逐实体解码 LINE/CIRCLE/ARC/POINT/BLOCK/ENDBLK/SEQEND | ✅ | 字段值 **bit 级一致**（含 R14/R2000/…/R2018） |
| 3DSOLID/BODY/REGION（§20.4.41 ACIS） | ✅ | **路线 B**：修复 R2013 data-store 修订字段宽度（8 vs 16 字节），Go 解码 Rust 拒绝的 4 条记录 |
| SOLID_BACKGROUND / SUN（自定义类） | ✅ | **路线 B**：SOLID_BACKGROUND 单 `rdInt32`；SUN 字段序由 `OdDbSun::dwgInFields` 重建（Go 解码 Rust 未处理的 SUN） |
| 非实体对象 DICTIONARY/CONTROL/PLACEHOLDER/XRECORD | ✅ | 与 Rust 一致（含 ODA R2004/R2013） |
| 符号表项 APPID/LAYER/LTYPE/STYLE/BLOCK_HEADER/DIMSTYLE | ✅ | 与 Rust 一致（含 ODA R2004/R2013） |
| 自定义类分发（class map）+ DICTIONARYVAR/SCALE/ACDBDICTIONARYWDFLT/VISUALSTYLE/MLEADERSTYLE | ✅ | 与 Rust 一致（含 ODA R2004/R2013） |
| ShSphere/Cylinder/Cone/Wedge（ACSH 图元） | ✅ | 路线 B：`OdDbEvalExpr` gc=-1 路径 + `_getType` 跳转表；四类 delta=0 |
| DETAILVIEWSTYLE | ✅ | 路线 B：直线主体 28 字段，delta=0 |
| TABLESTYLE | ✅ | 路线 B：双代分支；R2010+ 用 25.4 版完整 `OdCellStyle` 文法收口 |
| TEXT/INSERT/LWPOLYLINE/SPLINE/DIMENSION×7/RAY/XLINE/SOLID/TRACE | ✅ | 移植 + 合成位流单测（无语料记录） |
| VIEW/UCS 表项 | ✅ | 移植 + 合成位流单测（无语料记录） |
| VIEWPORT/3DFACE/LEADER/TOLERANCE/POLYLINE 族/OLE2FRAME/ATTRIB/ATTDEF | ✅ | 移植 + 合成位流单测 |
| HELIX/WIPEOUT/IMAGE/IMAGEDEF/UNDERLAY/CAMERA/GEODATA/MESH/四曲面/MLINE | ✅ | 移植 + 合成位流单测 |
| MLEADER | ✅ | 移植（嵌套 AnnotContext；本语料无实体记录，已接入待真实文件验证） |
| 其余实体/表项（MLEADER/MLINE/POLYLINE/LEADER/VIEW/UCS/…） | ⏳ 待补 | 由 RAE + ODA SDK 补全 |

> 对拍范围：容器层在 6 个合成样例 + 2 个 ODA 官方样例（R2004/R2013）上做到
> **section 反解字节逐字节一致**，以及 sections / handle map / class map /
> 对象列表 / 元数据全部一致。两份 ODA 语料上的**全部对象**均已解码：
> AC1018 48 条、AC1027 194 条，0 unhandled / 0 errored（`DWG_PROBE=1
> go test ./dwg/ -run TestDecodeProbeInventory` 可复核）。

## 安装

```
go get github.com/flywave/go-dwg
```

## 快速开始

### 命令行

```
go run ./cmd/dwginfo drawing.dwg            # JSON 摘要
go run ./cmd/dwginfo -objects drawing.dwg   # 附句柄驱动对象清单
```

### Go API

```go
f, err := dwg.OpenDwg("drawing.dwg")
fmt.Println(f.Version())                 // 2018
for _, s := range f.Sections() {
    fmt.Println(s.Name, s.Kind, s.Offset, s.Size)
}

// 任意命名 section 的解压字节
handles, ok, err := f.ReadSection("AcDb:Handles")

// 结构化解码
si, ok, err := f.SummaryInfo()
ai, ok, err := f.AppInfo()
hm, ok, err := f.HandleMap()
cm, ok, err := f.ClassMap()
hdr, ok, err := f.HeaderVars()

// 对象流
objs, ok, err := f.Objects()        // 顺序走
objs, ok, err := f.AllObjects()     // 句柄驱动（完整清单）
objs, summary, ok, err := f.AllObjectsLossy() // 含 skip/mismatch 汇总

// 逐实体解码（LINE/CIRCLE/ARC/POINT）
ents, summary, ok, err := f.DecodeAllEntities()
for _, e := range ents {
    if l, isLine := e.Value.(*dwg.Line); isLine {
        fmt.Println(l.Start, l.End)
    }
}
```

## 对拍（differential testing）

`tools/dwgref` 是一个 Rust 差分预言机：用参考实现 `dwg-rs` 解析同一文件并输出
规范化 JSON，可选地把每个 section 的解压字节落盘。Go 测试
`dwg/differential_test.go` 调用它并逐项比对。

```bash
# 构建预言机（需要 cargo）
cd tools/dwgref && cargo build

# 跑对拍（自动发现 tools/dwgref/target/debug/dwgref）
go test ./dwg/ -run TestDifferential -v

# 追加外部语料
DWG_CORPUS=/path/to/dwgs go test ./dwg/ -run TestDifferential -v
```

## 包结构

```
dwg/                 主包（对应 dwg-rs 的 src/）
  version.go         版本识别
  error.go           类型化错误
  cipher.go          R2004+ XOR / Sec_Mask
  crc.go             CRC-8 / CRC-32 / 页校验和
  bitcursor.go       §2 位流原语（BS/BL/BD/BLL/MC/MS/H/…）
  lz77.go            §4.7 LZ77 解压
  reed_solomon.go    Reed-Solomon(255,239) FEC（防御/修复路径）
  r21_lz.go          §5.10 R2007 LZ 变体（含字节置换表）
  header.go          R13-R15 / R2004+ 文件头
  section.go         命名 section
  section_map.go     §4.4-§4.6 页映射 / section info / 数据页
  handle_map.go      AcDb:Handles 句柄映射
  classes.go         AcDb:Classes 自定义类表
  metadata.go        SummaryInfo / AppInfo / Preview / FileDepList
  header_vars.go     AcDb:Header
  object_type.go     §20.3 对象类型
  object.go          AcDb:AcDbObjects walker
  string_stream.go   §19.1 R2007+ 字符串流定位
  common_entity.go   §19.4.1 通用实体前缀
  entity.go          LINE/CIRCLE/ARC/POINT/BLOCK/ENDBLK/SEQEND 解码 + 分发
  object_common.go   §19.4.2/§20.1 通用对象数据 + 对象流
  dictionary.go      DICTIONARY（§19.5.19）
  control.go         *_CONTROL 表拥有者（§19.5.1-10）
  placeholder.go     ACDB_PLACEHOLDER
  xrecord.go         XRECORD（§19.6.5）
  tables.go          §19.5.x 表项头 + CMC
  split_stream.go    R2007+ 表项字符串流
  table_appid.go     APPID（§19.5.50）
  table_layer.go     LAYER（§20.4.54）
  table_ltype.go     LTYPE（§20.4.58）
  table_style.go     STYLE（§20.4.56）
  block_record.go    BLOCK_HEADER（§20.4.52）+ 哨兵句柄解析
  table_dimstyle.go  DIMSTYLE（§20.4.68，15 变量 + 全体消费边界自检）
  classdispatch.go   自定义类分发（class map）+ DICTIONARYVAR/SCALE/WDFLT
  visual_style.go    ACAD_VISUALSTYLE（measured；flag-less + R2010+ 配对）
  mleader_style.go   ACAD_MLEADERSTYLE（measured）
  modeler.go         §20.4.41 3DSOLID/BODY/REGION（ACIS envelope + tail）
  solid_background.go ACAD 纯色背景对象（路线 B）
  sun.go             ACAD_SUN（路线 B，复用 §2.11 CMC）
  sh_primitive.go    ACSH 形状历史图元（路线 B，EvalExpr 链）
  detail_view_style.go ACDBDETAILVIEWSTYLE（路线 B）
  tablestyle.go      TABLESTYLE 自定义类（路线 B，双代分支）
  simple_entities.go RAY/XLINE/SOLID/TRACE/TEXT/INSERT
  lwpolyline.go      LWPOLYLINE
  spline.go          SPLINE
  dimension.go       DIMENSION 七子类
  entities_more.go   VIEWPORT/3DFACE/LEADER/TOLERANCE/ATTRIB/ATTDEF
  entities_legacy.go POLYLINE 族/PFACE/POLYMESH/OLE2FRAME
  entities_raster.go HELIX/IMAGE/WIPEOUT/IMAGEDEF/UNDERLAY
  entities_surface.go 四类 ACIS 曲面/MESH/CAMERA/GEODATA
  mleader.go         MULTILEADER
  object_color.go    §2.11 CMC 对象颜色
  block.go           BLOCK/ENDBLK/SEQEND 实体
  r2007.go           §5.1-§5.4 R2007 容器
  reader.go          DwgFile 顶层 API
cmd/dwginfo/         CLI
tools/dwgref/        Rust 差分预言机
ref/dwg-rs-main/     参考实现（Apache-2.0）
analysis/            端口状态 + RAE 补全计划
```

## 测试

```bash
go test ./...        # 单元测试 + 对拍（有预言机时）
go vet ./...
```

## 许可

MIT（Go 代码）。参考实现 `ref/dwg-rs-main` 为 Apache-2.0，仅作对拍基准，
其源码结构/命名不被复刻。
