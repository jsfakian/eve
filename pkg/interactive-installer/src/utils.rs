use std::{fs, os::unix::prelude::FileTypeExt};
use cursive::Cursive;

use crate::{
    actions::execute,
    error::Error,
    state::{GlobalState, Move},
};

pub fn get_state_mut(c: &mut Cursive) -> Result<GlobalState, Error> {
    match c.take_user_data::<GlobalState>() {
        Some(data) => {
            c.set_user_data(data.clone());
            Ok(data)
        }
        None => Err(Error::NoState),
    }
}

pub fn get_block_devices() -> Option<Vec<String>> {
    let mut vec = Vec::new();
    let devpaths = fs::read_dir("/dev/").unwrap();
    for path in devpaths {
        let devname = path.unwrap().path();
        let meta = fs::metadata(devname.clone()).unwrap();
        let file_type = meta.file_type();
        let name = devname.into_os_string().into_string().unwrap();

        // We're interested in block devices, so skip non-block devices
        if !name.contains("sd") && !name.contains("hd") && !name.contains("nvme") {
            continue;
        }

        if file_type.is_block_device() {
            vec.push(name);
        }
    }
    Some(vec)
}

pub fn press_next(c: &mut Cursive) {
    let cb = c.cb_sink().clone();
    cb.send(Box::new(move |s: &mut cursive::Cursive| {
        let _ = execute(s, Move::Next);
    }))
    .unwrap();
}

pub fn save_config_value(c: &mut Cursive, k: &str, v: &str, m: bool) -> crate::error::Result<()> {
    let mut s: GlobalState = c.take_user_data().unwrap();
    s.data.map.insert(k.to_string(), v.to_string());
    c.set_user_data(s);

    if m {
        press_next(c);
    }

    Ok(())
}

#[macro_export]
macro_rules! herr {
    ($c:expr,$f:expr) => {{
        if let Err(e) = $f($c) {
            e.show_dialog($c);
            return
        }
    }};
    ($c:expr,$f:expr,$($args:expr),*) => {{
        if let Err(e) = $f($c,$($args),*) {
            e.show_dialog($c);
            return
        }
    }};
}

#[macro_export]
macro_rules! herrcl {
    ($f:expr) => {{
        |c| {
            use crate::herr;
            herr!(c,$f);
        }
    }};
    ($f:expr,$($args:expr),*) => {{
        move |c| {
            use crate::herr;
            herr!(c,$f,$($args),*);
        }
    }};
}
