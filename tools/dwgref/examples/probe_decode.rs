use std::env;
use std::fs;
fn main() {
    for path in env::args().skip(1) {
        let raw = fs::read(&path).unwrap();
        let file = dwg::DwgFile::from_bytes(raw).unwrap();
        let class_map = file.class_map().and_then(|r| r.ok());
        let objs = match file.all_objects() { Some(Ok(o)) => o, _ => continue };
        for o in &objs {
            let e = match (&class_map, &o.kind) {
                (Some(cm), dwg::ObjectType::Custom(code)) => dwg::entities::decode_from_raw_with_class_map(o, file.version(), cm, *code),
                _ => dwg::entities::decode_from_raw(o, file.version()),
            };
            if o.type_code == 527 || o.type_code == 526 {
                println!("handle={:x} type={} -> {:?}", o.handle.value, o.type_code, e);
            }
        }
    }
}
