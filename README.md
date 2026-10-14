# go-dwg

纯 Go 的 CAD 图纸处理库：**DWG 解析 + DXF 读写 + DWG → DXF 转换**。

零 CGo、零第三方依赖，仅使用 Go 标准库，可静态编译为单一二进制，
适合嵌入服务端管线、桌面工具或 WASM 环境。

## 特性

- **DWG 解析**（[`dwg/`](dwg/)）：R14 → R2018（AC1014 → AC1032）全系列容器，
  包括 R2007 独立容器（Reed-Solomon + 专有 LZ 变体）
- **DXF 读写**（[`dxf/`](dxf/)）：R12 → R2018 双向读写，ASCII 与二进制双格式，
  未识别内容无损保留
- **DWG → DXF 转换**：一行调用把 DWG 实体、符号表、块定义与对象转换为
  DXF 文档
- **三个 CLI**：`dwginfo`（DWG JSON 摘要）、`dwg2dxf`（格式转换）、
  `dxfinfo`（DXF JSON 摘要）
- **输出经 ezdxf 1.4.4 互操作验证**：骨架文件 audit 0 错误 0 修复，
  转换文件 audit 0 错误

## 安装

```
go get github.com/flywave/go-dwg
```

## 命令行

```
go run ./cmd/dwginfo drawing.dwg            # DWG JSON 摘要
go run ./cmd/dwginfo -objects drawing.dwg   # 附句柄驱动对象清单
go run ./cmd/dwg2dxf -o out.dxf drawing.dwg # DWG → DXF 转换
go run ./cmd/dwg2dxf drawing.dwg > out.dxf  # 输出到 stdout
go run ./cmd/dxfinfo out.dxf                # DXF JSON 摘要
```

`dwg2dxf` 支持 `-version R12…R2018`（或 `AC1009…AC1032`）选择目标方言。

## Go API

### 读取 DWG

```go
f, err := dwg.OpenDwg("drawing.dwg")
fmt.Println(f.Version())                 // AC1032 (2018)
for _, s := range f.Sections() {
    fmt.Println(s.Name, s.Kind, s.Offset, s.Size)
}

// 任意命名 section 的解压字节
handles, ok, err := f.ReadSection("AcDb:Handles")

// 结构化解码
si, ok, err := f.SummaryInfo()   // Title / Author / Keywords / …
ai, ok, err := f.AppInfo()
hm, ok, err := f.HandleMap()
cm, ok, err := f.ClassMap()
hdr, ok, err := f.HeaderVars()

// 对象流
objs, ok, err := f.Objects()               // 顺序遍历
objs, ok, err := f.AllObjects()            // 句柄驱动（完整清单）
objs, summary, ok, err := f.AllObjectsLossy()

// 逐实体类型化解码
ents, summary, ok, err := f.DecodeAllEntities()
for _, e := range ents {
    if l, isLine := e.Value.(*dwg.Line); isLine {
        fmt.Println(l.Start, l.End)
    }
}
```

### 读写 DXF

```go
// 读：ASCII 与二进制自动识别
doc, err := dxf.Open("drawing.dxf")
fmt.Println(doc.Version)                 // R2018
for _, name := range doc.LayerNames() {
    fmt.Println(name)
}
for _, e := range doc.Entities {
    if c, ok := e.Data.(*dxf.Circle); ok {
        fmt.Println(c.Center, c.Radius)
    }
}

// 写：从合法最小骨架开始
doc := dxf.NewDocument(dxf.R2018)
line := &dxf.Line{Start: dxf.Pt2(0, 0), End: dxf.Pt2(1, 1)}
line.Handle = doc.AllocHandle()
doc.Entities = append(doc.Entities, &dxf.Entity{Type: "LINE", Data: line})
doc.Save("out.dxf")                      // 或 doc.SaveBinary("out.dxf")
```

### DWG → DXF 转换

```go
doc, err := dxf.ConvertDwgFile("drawing.dwg", dxf.R2018)
if err != nil {
    log.Fatal(err)
}
doc.Save("out.dxf")
```

## 支持范围

### DWG 容器层（`dwg` 包）

| 能力 | 说明 |
|---|---|
| 版本识别 | R14 / R2000 / R2004 / R2007 / R2010 / R2013 / R2018 |
| R13–R15 容器 | 平铺 section locator 头 |
| R2004+ 容器 | XOR 加密头（含 `AcFssFcAJMB` 校验）、Section Page Map、Section Info |
| R2007 容器 | Reed-Solomon FEC + 专有 LZ 解压变体 |
| 数据页 | Sec_Mask 页头解密、页校验和、CRC-8 / CRC-32 |
| 解压 | LZ77（含 RLE 回绕）、R2007 LZ 变体 |
| 索引 | HandleMap（句柄 → 偏移）、ClassMap（自定义类表） |
| 元数据 | SummaryInfo / AppInfo / Preview / FileDepList / AcDb:Header |
| 对象流 | 顺序与句柄驱动两种遍历，含容错（lossy）模式 |

### DWG 实体与对象（`dwg` 包）

| 类别 | 覆盖 |
|---|---|
| 几何实体 | LINE / POINT / CIRCLE / ARC / ELLIPSE / RAY / XLINE / SOLID / TRACE / 3DFACE |
| 曲线 | LWPOLYLINE / SPLINE / POLYLINE 族（2D、3D、多面网格、多边形网格及各类 VERTEX） |
| 注记 | TEXT / MTEXT / ATTRIB / ATTDEF / LEADER / MLEADER / TOLERANCE |
| 块与参照 | BLOCK / ENDBLK / SEQEND / INSERT |
| 填充与投影 | HATCH（含梯度填充与边界路径树）/ VIEWPORT / OLE2FRAME |
| 三维 | 3DSOLID / BODY / REGION（ACIS 记录）/ MESH / 四类 ACIS 曲面 / CAMERA |
| 栅格与地理 | IMAGE / IMAGEDEF / WIPEOUT / UNDERLAY / HELIX / GEODATA |
| 符号表项 | APPID / LAYER / LTYPE / STYLE / VPORT / VIEW / UCS / BLOCK_HEADER / DIMSTYLE |
| 非实体对象 | DICTIONARY / XRECORD / GROUP / ACDBPLACEHOLDER / LAYOUT / ACAD_MLINESTYLE |
| 自定义类 | class map 驱动分发：DICTIONARYVAR / SCALE / ACDBDICTIONARYWDFLT / VISUALSTYLE / MLEADERSTYLE / TABLESTYLE / DETAILVIEWSTYLE / ACSH 形状图元 / SUN / 纯色背景等 |

### DXF（`dxf` 包）

| 能力 | 说明 |
|---|---|
| 方言 | R12（AC1009）→ R2018（AC1032）；子类标记、句柄、OBJECTS section 按方言门控 |
| 格式 | ASCII 读写 + AutoCAD 二进制 DXF 读写（组码类编码、310–319 二进制块） |
| HEADER | 无损变量表（未知变量原样保留）+ 类型化访问器（$ACADVER / $INSUNITS / $EXTMIN …） |
| TABLES | 全表通用模型，条目逐字保留；LAYER / LTYPE 提供类型化访问器 |
| 实体 | LINE / POINT / CIRCLE / ARC / ELLIPSE / RAY / XLINE / SOLID / TRACE / 3DFACE / SHAPE / TEXT / MTEXT / ATTRIB / ATTDEF / INSERT（含 ATTRIB 挂接）/ POLYLINE（含 VERTEX / SEQEND）/ LWPOLYLINE / SPLINE / LEADER / HATCH（边界路径全语法）/ DIMENSION 七子类 / TOLERANCE / VIEWPORT |
| 实体公共属性 | 句柄 / 属主 / 图层 / ACI + 真彩色 / 线型 / 线宽 / 线型比例 / 可见性 / 厚度 / 拉伸向量 |
| 无损往返 | 未建模实体、未知表、未知 section 均逐字保留，读→写不丢数据 |
| 骨架生成 | `NewDocument` 产出合法最小文档（标准 HEADER 变量、符号表、块占位），可被主流阅读器直接打开 |

### DWG → DXF 转换

- 已解码实体逐一映射为对应 DXF 类型化实体
- 符号表进入 TABLES，BLOCK_HEADER 展开为 BLOCKS，非实体对象进入 OBJECTS
- 句柄空间去冲突：模板句柄序列自动让开源图句柄，$HANDSEED 随分配自动抬升
- 无覆盖记录以 `999 skipped: KIND xN` 注释显式列在输出中

## 包结构

```
dwg/                 DWG 解析包
  version.go         版本识别（AC1014 → AC1032）
  error.go           类型化错误
  cipher.go          R2004+ XOR / Sec_Mask
  crc.go             CRC-8 / CRC-32 / 页校验和
  bitcursor.go       位流原语（BS/BL/BD/BLL/MC/MS/H/…）
  lz77.go            LZ77 解压（含 RLE 回绕）
  reed_solomon.go    Reed-Solomon(255,239) FEC
  r21_lz.go          R2007 LZ 变体
  header.go          R13–R15 / R2004+ 文件头
  section.go         命名 section
  section_map.go     页映射 / section info / 数据页
  handle_map.go      AcDb:Handles 句柄映射
  classes.go         AcDb:Classes 自定义类表
  metadata.go        SummaryInfo / AppInfo / Preview / FileDepList
  header_vars.go     AcDb:Header
  object_type.go     对象类型分类
  object.go          AcDb:AcDbObjects walker
  string_stream.go   R2007+ 字符串流定位
  common_entity.go   通用实体前缀
  entity.go          实体分发 + LINE/CIRCLE/ARC/POINT
  object_common.go   通用对象数据 + 对象流
  dictionary.go      DICTIONARY
  group.go           GROUP
  control.go         *_CONTROL 表拥有者
  placeholder.go     ACDB_PLACEHOLDER
  xrecord.go         XRECORD
  tables.go          表项头 + CMC
  split_stream.go    R2007+ 表项字符串流
  table_*.go         APPID / LAYER / LTYPE / STYLE / VPORT / VIEW+UCS / DIMSTYLE / BLOCK_HEADER
  classdispatch.go   自定义类分发（class map）
  visual_style.go    ACAD_VISUALSTYLE
  mleader_style.go   ACAD_MLEADERSTYLE
  modeler.go         3DSOLID/BODY/REGION（ACIS）
  sun.go             ACAD_SUN
  solid_background.go 纯色背景对象
  sh_primitive.go    ACSH 形状历史图元
  detail_view_style.go ACDBDETAILVIEWSTYLE
  tablestyle.go      TABLESTYLE
  simple_entities.go RAY/XLINE/SOLID/TRACE/TEXT/INSERT
  lwpolyline.go      LWPOLYLINE
  spline.go          SPLINE
  dimension.go       DIMENSION 七子类
  entities_more.go   VIEWPORT/3DFACE/LEADER/TOLERANCE/ATTRIB/ATTDEF
  entities_legacy.go POLYLINE 族/PFACE/POLYMESH/OLE2FRAME
  entities_raster.go HELIX/IMAGE/WIPEOUT/IMAGEDEF/UNDERLAY
  entities_surface.go ACIS 曲面/MESH/CAMERA/GEODATA
  mleader.go         MULTILEADER
  mtext.go           MTEXT
  hatch.go           HATCH
  object_color.go    CMC 对象颜色
  block.go           BLOCK/ENDBLK/SEQEND
  r2007.go           R2007 容器
  reader.go          DwgFile 顶层 API

dxf/                 DXF 读写包
  version.go         方言 AC1009…AC1032 + 特性门控
  color.go           ACI 0-255 调色板 + 真彩色
  geometry.go        Point2 / Point3
  tag.go             Tag + ASCII 流解析
  tagwriter.go       规范组码发射器
  common.go          通用实体前缀
  entity.go          Entity / UnknownEntity + codec 注册表
  entities_basic.go  线/点/圆弧类几何实体
  entities_poly.go   LWPOLYLINE / POLYLINE / SPLINE / LEADER
  entities_text.go   TEXT / MTEXT / ATTRIB / ATTDEF / INSERT / SEQEND
  entities_hatch.go  HATCH
  entities_dim.go    DIMENSION
  entities_misc.go   TOLERANCE / VIEWPORT
  header.go          HEADER 变量表
  tables.go          TABLES
  classes.go         CLASSES
  blocks.go          BLOCK / ENDBLK
  objects.go         OBJECTS
  document.go        Document 顶层 API
  reader.go          section 解析 + 子实体挂接
  writer.go          文档发射 + NewDocument 骨架
  binary.go          二进制 DXF
  fromdwg.go         DWG → DXF 转换器

cmd/dwginfo/         DWG JSON 摘要
cmd/dwg2dxf/         DWG → DXF 转换
cmd/dxfinfo/         DXF JSON 摘要
```

## 测试

```
go test ./...        # 全部单元测试（含合成语料与 ODA 样例）
go vet ./...
```

`dwg/testdata/` 内置 6 个合成夹具（R14 → R2018）与 2 个 ODA 官方样例；
转换器套件对语料逐文件验证「转换 → 重新解析 → 实体数一致」，DXF 输出
另经 ezdxf 1.4.4 互操作验证（audit 0 错误）。

## 已知边界

- 实体级图层/颜色/线型在 DWG 侧是句柄引用，当前解码器未做跨句柄解析，
  转换出的实体携带默认值（layer "0"、ByLayer）；符号表本身完整输出。
- DICTIONARY 的 3/350 值句柄对、对象属主链依赖尚未开放的句柄流 → OBJECTS
  记录统一挂到命名对象根字典，严格阅读器（ezdxf）会把未被 ACAD_LAYOUT
  引用的 LAYOUT 视为孤儿清理（记录内容不丢失，仅父链不重建）。
- INSERT 的块名依赖 BLOCK_HEADER 解码结果，无法解析时该引用留空。
- DXF 侧未建模的复杂实体（IMAGE / MESH / MULTILEADER / 3DSOLID 族等）在
  读取时以 `UnknownEntity` 逐字保留，写出时不失真；转换器会跳过它们并
  在注释中列出清单。

## 许可

MIT
