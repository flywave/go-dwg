//! `dwgref` — differential-testing oracle for the Go port.
//!
//! Reads a DWG file with the reference `dwg` crate and prints a canonical
//! JSON summary. Optionally dumps every decompressed section to a
//! directory so the Go reader can be compared byte-for-byte.
//!
//! Usage:
//!   dwgref <file.dwg> [--dump-sections DIR] [--max-objects N]

use std::env;
use std::fs;
use std::path::PathBuf;

use dwg::{DwgFile, ObjectType};

fn jstr(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

fn sanitize(name: &str) -> String {
    name.chars()
        .map(|c| if c.is_ascii_alphanumeric() || c == '_' || c == '-' || c == '.' { c } else { '_' })
        .collect()
}

fn status_variant(s: &dwg::SectionMapStatus) -> &'static str {
    match s {
        dwg::SectionMapStatus::Full => "Full",
        dwg::SectionMapStatus::Fallback { .. } => "Fallback",
        dwg::SectionMapStatus::Deferred { .. } => "Deferred",
    }
}

fn type_label(t: &ObjectType) -> String {
    t.to_string()
}

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!("usage: dwgref <file.dwg> [--dump-sections DIR] [--max-objects N]");
        std::process::exit(2);
    }
    let path = &args[1];
    let mut dump_dir: Option<PathBuf> = None;
    let mut max_objects: usize = 4096;
    let mut i = 2;
    while i < args.len() {
        match args[i].as_str() {
            "--dump-sections" => {
                if i + 1 < args.len() {
                    dump_dir = Some(PathBuf::from(&args[i + 1]));
                    i += 2;
                } else {
                    i += 1;
                }
            }
            "--max-objects" => {
                if i + 1 < args.len() {
                    max_objects = args[i + 1].parse().unwrap_or(max_objects);
                    i += 2;
                } else {
                    i += 1;
                }
            }
            _ => i += 1,
        }
    }

    let bytes = match fs::read(path) {
        Ok(b) => b,
        Err(e) => {
            println!("{{\"error\":{}}}", jstr(&format!("read failed: {e}")));
            std::process::exit(1);
        }
    };
    let file = match DwgFile::from_bytes(bytes) {
        Ok(f) => f,
        Err(e) => {
            println!("{{\"error\":{}}}", jstr(&format!("parse failed: {e}")));
            std::process::exit(1);
        }
    };

    if let Some(dir) = &dump_dir {
        let _ = fs::create_dir_all(dir);
    }

    let mut out = String::new();
    out.push('{');
    out.push_str(&format!("\"magic\":{},", jstr(&String::from_utf8_lossy(&file.version().magic()))));
    out.push_str(&format!("\"version\":{},", jstr(file.version().release())));
    out.push_str(&format!("\"file_size\":{},", file.file_size()));
    out.push_str(&format!(
        "\"section_map_status\":{},",
        jstr(status_variant(file.section_map_status()))
    ));

    // Sections.
    out.push_str("\"sections\":[");
    for (idx, s) in file.sections().iter().enumerate() {
        if idx > 0 {
            out.push(',');
        }
        out.push_str(&format!(
            "{{\"name\":{},\"kind\":{},\"offset\":{},\"size\":{},\"compressed\":{},\"encrypted\":{}}}",
            jstr(&s.name),
            jstr(s.kind.short_label()),
            s.offset,
            s.size,
            s.compressed,
            s.encrypted
        ));
    }
    out.push_str("],");

    // Sections manifest (for dump comparison) + optional byte dump.
    let mut manifest = String::from("[");
    let mut manifest_first = true;
    for s in file.sections() {
        let read = file.read_section(&s.name);
        match read {
            Some(Ok(data)) => {
                let fname = sanitize(&s.name);
                if let Some(dir) = &dump_dir {
                    let _ = fs::write(dir.join(&fname), &data);
                }
                if !manifest_first {
                    manifest.push(',');
                }
                manifest_first = false;
                manifest.push_str(&format!(
                    "{{\"name\":{},\"file\":{},\"len\":{}}}",
                    jstr(&s.name),
                    jstr(&fname),
                    data.len()
                ));
            }
            _ => {}
        }
    }
    manifest.push(']');
    out.push_str(&format!("\"section_manifest\":{},", manifest));

    // Handle map.
    match file.handle_map() {
        Some(Ok(m)) => {
            out.push_str(&format!("\"handle_map_entries\":{},", m.len()));
            out.push_str("\"handle_map_first\":[");
            for (idx, e) in m.iter().take(64).enumerate() {
                if idx > 0 {
                    out.push(',');
                }
                out.push_str(&format!("[{},{}]", e.handle, e.offset));
            }
            out.push_str("],");
        }
        Some(Err(e)) => {
            out.push_str(&format!("\"handle_map_error\":{},", jstr(&e.to_string())));
        }
        None => {
            out.push_str("\"handle_map_entries\":null,");
        }
    }

    // Class map.
    match file.class_map() {
        Some(Ok(cm)) => {
            out.push_str(&format!("\"class_max\":{},", cm.max_class_number));
            out.push_str("\"classes\":[");
            for (idx, c) in cm.classes.iter().enumerate() {
                if idx > 0 {
                    out.push(',');
                }
                out.push_str(&format!(
                    "{{\"n\":{},\"app\":{},\"cpp\":{},\"dxf\":{},\"num\":{},\"dwg\":{},\"maint\":{},\"proxy\":{},\"item\":{}}}",
                    c.class_number,
                    jstr(&c.app_name),
                    jstr(&c.cpp_class_name),
                    jstr(&c.dxf_class_name),
                    c.num_objects,
                    c.dwg_version,
                    c.maintenance_version,
                    c.was_a_proxy,
                    c.item_class_id
                ));
            }
            out.push_str("],");
        }
        _ => {
            out.push_str("\"class_max\":null,\"classes\":[],");
        }
    }

    // Header vars.
    match file.header_vars() {
        Some(Ok(hv)) => {
            out.push_str(&format!(
                "\"header_vars\":{{\"sentinel_ok\":{},\"size_in_bits\":{},\"body_len\":{}}},",
                hv.has_valid_sentinel(),
                hv.size_in_bits,
                hv.body_len()
            ));
        }
        _ => {
            out.push_str("\"header_vars\":null,");
        }
    }

    // Sequential objects walk.
    match file.objects() {
        Some(Ok(objs)) => {
            out.push_str(&format!("\"objects_seq_count\":{},", objs.len()));
        }
        _ => {
            out.push_str("\"objects_seq_count\":null,");
        }
    }

    // Handle-driven object walk.
    match file.all_objects_lossy() {
        Some(Ok((objs, summary))) => {
            out.push_str(&format!("\"all_objects_count\":{},", objs.len()));
            out.push_str(&format!(
                "\"walk\":{{\"decoded\":{},\"skipped\":{},\"unknown_types\":{},\"mismatches\":{}}},",
                summary.decoded_count,
                summary.skipped.len(),
                summary.unknown_types.len(),
                summary.handle_mismatches.len()
            ));
            out.push_str("\"all_objects\":[");
            for (idx, o) in objs.iter().take(max_objects).enumerate() {
                if idx > 0 {
                    out.push(',');
                }
                out.push_str(&format!(
                    "{{\"off\":{},\"size\":{},\"type\":{},\"label\":{},\"handle\":{},\"raw_len\":{}}}",
                    o.stream_offset,
                    o.size_bytes,
                    o.type_code,
                    jstr(&type_label(&o.kind)),
                    o.handle.value,
                    o.raw.len()
                ));
            }
            out.push_str("],");
        }
        Some(Err(e)) => {
            out.push_str(&format!("\"all_objects_error\":{},", jstr(&e.to_string())));
        }
        None => {
            out.push_str("\"all_objects_count\":null,");
        }
    }

    // Summary info.
    match file.summary_info() {
        Some(Ok(si)) => {
            out.push_str(&format!(
                "\"summary_info\":{{\"title\":{},\"author\":{},\"subject\":{},\"keywords\":{},\"last_saved_by\":{},\"props\":{}}},",
                jstr(&si.title),
                jstr(&si.author),
                jstr(&si.subject),
                jstr(&si.keywords),
                jstr(&si.last_saved_by),
                si.properties.len()
            ));
        }
        _ => {
            out.push_str("\"summary_info\":null,");
        }
    }

    // App info.
    match file.app_info() {
        Some(Ok(ai)) => {
            out.push_str(&format!(
                "\"app_info\":{{\"name\":{},\"version\":{},\"product\":{}}},",
                jstr(&ai.name),
                jstr(&ai.version),
                jstr(&ai.product)
            ));
        }
        _ => {
            out.push_str("\"app_info\":null,");
        }
    }

    // Entity dispatch coverage.
    let mut entity_values = String::from("[");
    let mut first_ev = true;
    let mut decoded_entity_json: Option<String> = None;
    // Project per-entity field values through the NO-class-map dispatch so
    // the comparison uses the same semantics as the Go side (which does not
    // resolve custom classes).
    if let Some(Ok(objs)) = file.all_objects() {
        let class_map = file.class_map().and_then(|r| r.ok());
        for o in &objs {
            let e = match (&class_map, &o.kind) {
                (Some(cm), dwg::ObjectType::Custom(code)) => {
                    dwg::entities::decode_from_raw_with_class_map(o, file.version(), cm, *code)
                }
                _ => dwg::entities::decode_from_raw(o, file.version()),
            };
            if let Some(obj) = entity_value_json(&e) {
                if !first_ev {
                    entity_values.push(',');
                }
                first_ev = false;
                entity_values.push_str(&obj);
            }
        }
    }
    match file.decoded_entities() {
        Some(Ok((_, summary))) => {
            decoded_entity_json = Some(format!(
                "{{\"decoded\":{},\"unhandled\":{},\"errored\":{}}}",
                summary.decoded, summary.unhandled, summary.errored
            ));
        }
        _ => {}
    }
    entity_values.push(']');
    out.push_str(&format!("\"entity_values\":{},", entity_values));
    match decoded_entity_json {
        Some(s) => out.push_str(&format!("\"decoded_entities\":{}", s)),
        None => out.push_str("\"decoded_entities\":null"),
    }

    out.push('}');
    println!("{out}");
}

/// f64 as its exact IEEE-754 bit pattern (compared bit-for-bit by the Go
/// side, avoiding float formatting differences).
fn bits(v: f64) -> u64 {
    v.to_bits()
}

fn hexbytes(b: &[u8]) -> String {
    let mut hex = String::with_capacity(b.len() * 2);
    for x in b {
        hex.push_str(&format!("{x:02x}"));
    }
    hex
}

/// Project a modeler (3DSOLID/BODY/REGION) record for comparison.
fn modeler_json(
    tag: &str,
    acis_empty: bool,
    in_data_store: bool,
    tail: &Option<dwg::entities::modeler::ModelerTail>,
) -> String {
    let (wf, pt, iso, isop, nw, ns, ae2, guid) = match tail {
        Some(t) => (
            t.wireframe_present,
            t.point
                .as_ref()
                .map(pt3)
                .unwrap_or_else(|| "null".to_string()),
            t.num_isolines,
            t.isolines_present,
            t.num_wires,
            t.num_silhouettes,
            t.acis_empty_2,
            t.revision_guid
                .as_ref()
                .map(|g| jstr(&hexbytes(g)))
                .unwrap_or_else(|| "null".to_string()),
        ),
        None => (false, "null".to_string(), 0, false, 0, 0, false, "null".to_string()),
    };
    format!(
        "{{\"t\":\"{tag}\",\"empty\":{acis_empty},\"instore\":{in_data_store},\"wf\":{wf},\"pt\":{pt},\"iso\":{iso},\"isop\":{isop},\"nw\":{nw},\"ns\":{ns},\"ae2\":{ae2},\"guid\":{guid}}}"
    )
}

fn pt3(p: &dwg::entities::Point3D) -> String {
    format!("[{},{},{}]", bits(p.x), bits(p.y), bits(p.z))
}

fn pt2(p: &dwg::entities::Point2D) -> String {
    format!("[{},{}]", bits(p.x), bits(p.y))
}

/// JSON for one decoded entity's fields, or `None` for types this oracle does
/// not project.
fn bb(v: bool) -> &'static str {
    if v {
        "1"
    } else {
        "0"
    }
}

/// Canonical HATCH signature; must match Go's `hatchSignature`.
fn hatch_sig(h: &dwg::entities::hatch::Hatch) -> String {
    let mut s = String::new();
    match &h.gradient {
        None => s.push_str("G0"),
        Some(g) => {
            s.push_str(&format!(
                "G1,{},{},{},{},{},{}",
                bits(g.angle),
                bits(g.shift),
                g.is_single_color,
                bits(g.tint),
                g.name,
                g.colors.len()
            ));
            for c in &g.colors {
                s.push_str(&format!(
                    ",({},{},{},{})",
                    bits(c.unknown_double),
                    c.unknown_short,
                    c.rgb,
                    c.ignored_color_byte
                ));
            }
        }
    }
    s.push_str(&format!(
        " H{},{},{},{},{},{},{}",
        bits(h.elevation),
        bits(h.extrusion.x),
        bits(h.extrusion.y),
        bits(h.extrusion.z),
        h.pattern_name,
        bb(h.solid_fill),
        bb(h.associative)
    ));
    s.push_str(&format!(" N{}", h.paths.len()));
    for p in &h.paths {
        s.push_str(&format!(
            " [{},{},{}]",
            p.flags,
            p.num_boundary_handles,
            seg_sig(&p.segments)
        ));
    }
    s.push_str(&format!(
        " T{},{},{},{},{},L{}",
        h.pattern_style,
        h.pattern_type,
        bits(h.pattern_angle),
        bits(h.pattern_scale),
        bb(h.pattern_double),
        h.pattern_lines.len()
    ));
    for pl in &h.pattern_lines {
        s.push_str(&format!(
            " {{{},{},{},{},{},{}",
            bits(pl.angle),
            bits(pl.origin.x),
            bits(pl.origin.y),
            bits(pl.offset.x),
            bits(pl.offset.y),
            pl.dashes.len()
        ));
        for d in &pl.dashes {
            s.push_str(&format!(",{}", bits(*d)));
        }
        s.push('}');
    }
    s.push_str(&format!(" P{},S{}", bits(h.pixel_size), h.seed_points.len()));
    for sp in &h.seed_points {
        s.push_str(&format!(" ({},{})", bits(sp.0), bits(sp.1)));
    }
    s
}

fn seg_sig(seg: &dwg::entities::hatch::HatchPathSegments) -> String {
    use dwg::entities::hatch::HatchPathSegments;
    match seg {
        HatchPathSegments::Polyline {
            has_bulge,
            is_closed,
            vertices,
        } => {
            let mut t = format!("P{},{},{}", bb(*has_bulge), bb(*is_closed), vertices.len());
            for (p, bulge) in vertices {
                match bulge {
                    Some(b) => t.push_str(&format!(",({},{},{})", bits(p.x), bits(p.y), bits(*b))),
                    None => t.push_str(&format!(",({},{})", bits(p.x), bits(p.y))),
                }
            }
            t
        }
        HatchPathSegments::Edges(edges) => {
            let mut t = format!("E{}", edges.len());
            for e in edges {
                t.push_str(&format!(",{}", edge_sig(e)));
            }
            t
        }
    }
}

fn edge_sig(e: &dwg::entities::hatch::HatchEdge) -> String {
    use dwg::entities::hatch::HatchEdge;
    match e {
        HatchEdge::Line { start, end } => format!(
            "L({},{},{},{})",
            bits(start.x),
            bits(start.y),
            bits(end.x),
            bits(end.y)
        ),
        HatchEdge::Arc {
            center,
            radius,
            start_angle,
            end_angle,
            counter_clockwise,
        } => format!(
            "A({},{},{},{},{},{})",
            bits(center.x),
            bits(center.y),
            bits(*radius),
            bits(*start_angle),
            bits(*end_angle),
            bb(*counter_clockwise)
        ),
        HatchEdge::Ellipse {
            center,
            endpoint,
            axis_ratio,
            start_angle,
            end_angle,
            counter_clockwise,
        } => format!(
            "X({},{},{},{},{},{},{},{})",
            bits(center.x),
            bits(center.y),
            bits(endpoint.x),
            bits(endpoint.y),
            bits(*axis_ratio),
            bits(*start_angle),
            bits(*end_angle),
            bb(*counter_clockwise)
        ),
        HatchEdge::Spline {
            degree,
            is_rational,
            is_periodic,
            knots,
            control_points,
            weights,
            fit_points,
            start_tangent,
            end_tangent,
        } => {
            let mut t = format!("S({},{},{},{}", degree, bb(*is_rational), bb(*is_periodic), knots.len());
            for k in knots {
                t.push_str(&format!(",{}", bits(*k)));
            }
            t.push_str(&format!(";{}", control_points.len()));
            for (i, cp) in control_points.iter().enumerate() {
                t.push_str(&format!(",({},{}", bits(cp.x), bits(cp.y)));
                if *is_rational {
                    t.push_str(&format!(",{}", bits(weights[i])));
                }
                t.push(')');
            }
            t.push_str(&format!(";{}", fit_points.len()));
            for fp in fit_points {
                t.push_str(&format!(",({},{})", bits(fp.x), bits(fp.y)));
            }
            t.push_str(&format!(
                ";{},{},{},{}",
                bits(start_tangent.x),
                bits(start_tangent.y),
                bits(end_tangent.x),
                bits(end_tangent.y)
            ));
            t.push(')');
            t
        }
    }
}

fn entity_value_json(e: &dwg::entities::DecodedEntity) -> Option<String> {
    use dwg::entities::DecodedEntity as D;
    Some(match e {
        D::Line(l) => format!(
            "{{\"t\":\"LINE\",\"is2d\":{},\"start\":{},\"end\":{},\"thickness\":{},\"extrusion\":{}}}",
            l.is_2d,
            pt3(&l.start),
            pt3(&l.end),
            bits(l.thickness),
            pt3(&l.extrusion)
        ),
        D::Circle(c) => format!(
            "{{\"t\":\"CIRCLE\",\"center\":{},\"radius\":{},\"thickness\":{},\"extrusion\":{}}}",
            pt3(&c.center),
            bits(c.radius),
            bits(c.thickness),
            pt3(&c.extrusion)
        ),
        D::Arc(a) => format!(
            "{{\"t\":\"ARC\",\"center\":{},\"radius\":{},\"thickness\":{},\"extrusion\":{},\"start_angle\":{},\"end_angle\":{}}}",
            pt3(&a.center),
            bits(a.radius),
            bits(a.thickness),
            pt3(&a.extrusion),
            bits(a.start_angle),
            bits(a.end_angle)
        ),
        D::Point(p) => format!(
            "{{\"t\":\"POINT\",\"position\":{},\"thickness\":{},\"extrusion\":{},\"angle\":{}}}",
            pt3(&p.position),
            bits(p.thickness),
            pt3(&p.extrusion),
            bits(p.x_axis_angle)
        ),
        D::Dictionary(d) => {
            let mut keys = String::new();
            for (i, k) in d.keys.iter().enumerate() {
                if i > 0 {
                    keys.push(',');
                }
                keys.push_str(&jstr(k));
            }
            format!(
                "{{\"t\":\"DICTIONARY\",\"cloning\":{},\"hard\":{},\"keys\":[{}]}}",
                d.cloning_flag, d.hard_owner, keys
            )
        }
        D::Placeholder(p) => {
            format!("{{\"t\":\"PLACEHOLDER\",\"reactors\":{}}}", p.num_reactors)
        }
        D::XRecord(x) => {
            let mut hex = String::with_capacity(x.data.len() * 2);
            for b in &x.data {
                hex.push_str(&format!("{b:02x}"));
            }
            format!(
                "{{\"t\":\"XRECORD\",\"cloning\":{},\"data\":{}}}",
                x.cloning_flags,
                jstr(&hex)
            )
        }
        D::AppId(a) => format!("{{\"t\":\"APPID\",\"name\":{}}}", jstr(&a.header.name)),
        D::Layer(l) => format!(
            "{{\"t\":\"LAYER\",\"name\":{},\"flags\":{},\"plot\":{},\"lw\":{},\"color\":{}}}",
            jstr(&l.header.name),
            l.flags,
            l.plot_flag,
            l.lineweight,
            l.color_index
        ),
        D::Ltype(l) => format!(
            "{{\"t\":\"LTYPE\",\"name\":{},\"desc\":{},\"pattern\":{},\"align\":{},\"nd\":{}}}",
            jstr(&l.header.name),
            jstr(&l.description),
            bits(l.pattern_length),
            l.alignment,
            l.dashes.len()
        ),
        D::Style(s) => format!(
            "{{\"t\":\"STYLE\",\"name\":{},\"flags\":{},\"fixed\":{},\"font\":{},\"bigfont\":{}}}",
            jstr(&s.header.name),
            s.flags,
            bits(s.fixed_height),
            jstr(&s.font_filename),
            jstr(&s.bigfont_filename)
        ),
        D::Block(b) => format!("{{\"t\":\"BLOCK\",\"name\":{}}}", jstr(&b.name)),
        D::EndBlk(_) => "{\"t\":\"ENDBLK\"}".to_string(),
        D::SeqEnd(_) => "{\"t\":\"SEQEND\"}".to_string(),
        D::BlockRecord(b) => format!(
            "{{\"t\":\"BLOCK_HEADER\",\"name\":{},\"anon\":{},\"isxref\":{},\"owned\":{},\"base\":{},\"xref\":{},\"desc\":{},\"sentinel\":{}}}",
            jstr(&b.header.name),
            b.is_anonymous,
            b.is_xref,
            match b.num_owned_objects { Some(n) => n.to_string(), None => "null".to_string() },
            pt3(&b.base_point),
            jstr(&b.xref_path),
            jstr(&b.description),
            match b.block_sentinel_handle { Some(h) => h.to_string(), None => "null".to_string() }
        ),
        D::DimStyle(d) => format!(
            "{{\"t\":\"DIMSTYLE\",\"name\":{},\"dimscale\":{},\"dimasz\":{},\"dimexo\":{},\"dimexe\":{},\"dimtxt\":{},\"dimcen\":{},\"dimtfac\":{},\"dimlfac\":{},\"dimaltf\":{},\"dimaltrnd\":{},\"dimtad\":{},\"dimtolj\":{},\"dimtih\":{},\"dimtoh\":{},\"dimupt\":{}}}",
            jstr(&d.header.name),
            bits(d.dimscale),
            bits(d.dimasz),
            bits(d.dimexo),
            bits(d.dimexe),
            bits(d.dimtxt),
            bits(d.dimcen),
            bits(d.dimtfac),
            bits(d.dimlfac),
            bits(d.dimaltf),
            bits(d.dimaltrnd),
            d.dimtad,
            d.dimtolj,
            d.dimtih,
            d.dimtoh,
            d.dimupt
        ),
        D::DictionaryVar(v) => format!(
            "{{\"t\":\"DICTIONARYVAR\",\"schema\":{},\"value\":{}}}",
            v.schema,
            jstr(&v.value)
        ),
        D::Scale(s) => format!(
            "{{\"t\":\"SCALE\",\"name\":{},\"version\":{},\"paper\":{},\"drawing\":{},\"unit\":{}}}",
            jstr(&s.scale_name),
            s.version,
            bits(s.paper_units),
            bits(s.drawing_units),
            s.is_unit_scale
        ),
        D::VisualStyle(v) => format!(
            "{{\"t\":\"VISUALSTYLE\",\"name\":{},\"ist\":{},\"fmt\":{},\"internal\":{},\"flm\":{},\"flq\":{},\"fcm\":{},\"fmod\":{},\"fop\":{},\"fsp\":{},\"emod\":{},\"est\":{},\"ecr\":{},\"dw\":{},\"ov\":{},\"jt\":{},\"sw\":{},\"hg\":{},\"ic\":{},\"hp\":{},\"db\":{},\"dst\":{},\"nflags\":{},\"next\":{},\"nstr\":{}}}",
            jstr(&v.description),
            v.internal_style_type,
            v.format_version,
            v.is_internal_use_only,
            v.face_lighting_model,
            v.face_lighting_quality,
            v.face_color_mode,
            v.face_modifier,
            bits(v.face_opacity),
            bits(v.face_specular),
            v.edge_model,
            v.edge_style,
            bits(v.edge_crease_angle),
            v.edge_width,
            v.edge_overhang,
            v.edge_jitter,
            v.edge_silhouette_width,
            v.edge_halo_gap,
            v.edge_isoline_count,
            v.edge_hide_precision,
            bits(v.display_brightness),
            v.display_shadow_type,
            v.property_flags.len(),
            v.extended.len(),
            v.trailing_strings.len()
        ),
        D::ThreeDSolid(s) => modeler_json("3DSOLID", s.acis_empty, s.in_data_store, &s.tail),
        D::Region(r) => modeler_json("REGION", r.acis_empty, r.in_data_store, &r.tail),
        D::Body(b) => modeler_json("BODY", b.acis_empty, b.in_data_store, &b.tail),
        D::MLeaderStyle(m) => format!(
            "{{\"t\":\"MLEADERSTYLE\",\"name\":{},\"version\":{},\"content\":{},\"dmo\":{},\"dlo\":{},\"maxseg\":{},\"fsa\":{},\"ssa\":{},\"llt\":{},\"llw\":{},\"landing\":{},\"lgb\":{},\"dogleg\":{},\"dl\":{},\"arrow\":{},\"tla\":{},\"tra\":{},\"tat\":{},\"talt\":{},\"th\":{},\"frame\":{},\"alwaysleft\":{},\"aspace\":{},\"scale\":{},\"overwrite\":{},\"annot\":{},\"bg\":{},\"tad\":{},\"btad\":{},\"ttad\":{},\"ext\":{}}}",
            jstr(&m.description),
            m.version,
            m.content_type,
            m.draw_mleader_order_type,
            m.draw_leader_order_type,
            m.max_leader_segment_points,
            bits(m.first_segment_angle),
            bits(m.second_segment_angle),
            m.leader_line_type,
            m.leader_lineweight,
            m.enable_landing,
            bits(m.landing_gap),
            m.enable_dogleg,
            bits(m.dogleg_length),
            bits(m.arrowhead_size),
            m.text_left_attachment,
            m.text_right_attachment,
            m.text_angle_type,
            m.text_alignment_type,
            bits(m.text_height),
            m.enable_frame_text,
            m.text_align_always_left,
            bits(m.align_space),
            bits(m.scale),
            m.overwrite_property_value,
            m.is_annotative,
            bits(m.break_gap_size),
            m.text_attachment_direction,
            m.bottom_text_attachment_direction,
            m.top_text_attachment_direction,
            m.extended_flag
        ),
        D::VPort(v) => format!(
            "{{\"t\":\"VPORT\",\"name\":{},\"vh\":{},\"vw\":{},\"vc\":{},\"vt\":{},\"vd\":{},\"twist\":{},\"lens\":{},\"fclip\":{},\"bclip\":{},\"vmode\":{},\"rmode\":{},\"ll\":{},\"ur\":{},\"ucsorg\":{},\"ucsper\":{},\"sbase\":{},\"ssp\":{},\"gsp\":{},\"srot\":{}}}",
            jstr(&v.header.name),
            bits(v.view_height),
            bits(v.aspect_ratio),
            pt2(&v.view_center),
            pt3(&v.view_target),
            pt3(&v.view_direction),
            bits(v.view_twist),
            bits(v.lens_length),
            bits(v.front_clip),
            bits(v.back_clip),
            v.view_mode,
            v.render_mode,
            pt2(&v.lower_left),
            pt2(&v.upper_right),
            v.ucs_at_origin,
            v.ucs_per_vport,
            pt2(&v.snap_base),
            pt2(&v.snap_spacing),
            pt2(&v.grid_spacing),
            bits(v.snap_rotation)
        ),
        D::Hatch(h) => format!("{{\"t\":\"HATCH\",\"hatch\":{}}}", jstr(&hatch_sig(h))),
        D::Ellipse(e) => format!(
            "{{\"t\":\"ELLIPSE\",\"center\":{},\"emajor\":{},\"extrusion\":{},\"eaxis\":{},\"estart\":{},\"eend\":{}}}",
            pt3(&e.center),
            pt3(&e.major_axis),
            pt3(&e.extrusion),
            bits(e.axis_ratio),
            bits(e.start_param),
            bits(e.end_param)
        ),
        D::MLineStyle(m) => {
            let mlines: Vec<String> = m
                .lines
                .iter()
                .map(|ln| format!("{},{},{}", bits(ln.offset), ln.color.rgb, ln.linetype_index as u16))
                .collect();
            format!(
                "{{\"t\":\"MLINESTYLE\",\"mname\":{},\"mdesc\":{},\"mflags\":{},\"mfill\":{},\"msa\":{},\"mea\":{},\"mlines\":[{}]}}",
                jstr(&m.name),
                jstr(&m.description),
                m.flags,
                m.fill_color.rgb,
                bits(m.start_angle),
                bits(m.end_angle),
                mlines.join(",")
            )
        }
        D::SectionViewStyle(s) => {
            let col = |c: &dwg::objects::color::ObjectColor| format!("{},{}", c.index, c.rgb);
            let svc = [
                col(&s.identifier_color),
                col(&s.arrow_symbol_color),
                col(&s.cutting_plane_line_color),
                col(&s.end_line_color),
                col(&s.view_label_color),
                col(&s.hatch_color),
                col(&s.hatch_background_color),
            ]
            .join(",");
            let mut svf: Vec<String> = [
                s.identifier_height,
                s.arrow_symbol_size,
                s.identifier_offset,
                s.end_line_length,
                s.end_line_overshoot,
                s.view_label_text_height,
                s.view_label_offset,
                s.hatch_scale,
                s.hatch_spacing,
            ]
            .iter()
            .map(|v| bits(*v).to_string())
            .collect();
            for a in s.hatch_angles {
                svf.push(bits(a).to_string());
            }
            format!(
                "{{\"t\":\"SECTIONVIEWSTYLE\",\"svname\":{},\"svalias\":{},\"svi\":[{},{},{},{},{},{},{},{},{}],\"svb\":[{},{},{}],\"svf\":[{}],\"svc\":[{}],\"svs\":[{},{},{}],\"svhf\":{}}}",
                jstr(&s.name),
                jstr(&s.name_alias),
                s.unknown_head,
                s.flags,
                s.cutting_plane_line_weight,
                s.end_line_weight,
                s.view_label_position,
                s.view_label_attachment,
                s.hatch_transparency,
                s.hatch_angle_count,
                s.unknown_tail,
                s.flag_c,
                s.flag_d,
                s.flag_e,
                svf.join(","),
                svc,
                jstr(&s.identifier_exclude_characters),
                jstr(&s.view_label_pattern),
                jstr(&s.hatch_pattern_name),
                s.hatch_flags
            )
        }
        D::Control { kind, control } => match control.dimstyle_trailing_rc {
            Some(rc) => format!(
                "{{\"t\":\"CONTROL\",\"kind\":{},\"n\":{},\"dimrc\":{}}}",
                jstr(kind.short_label()),
                control.num_entries,
                rc
            ),
            None => format!(
                "{{\"t\":\"CONTROL\",\"kind\":{},\"n\":{}}}",
                jstr(kind.short_label()),
                control.num_entries
            ),
        },
        _ => return None,
    })
}
