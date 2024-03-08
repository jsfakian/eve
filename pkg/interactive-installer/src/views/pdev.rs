/*
 * installer-tui
 * Copyright (C) 2022 Ioannis Sfakianakis
 */

use std::collections::HashMap;

use crate::{
    data::{PERSIST_DISK},
    herr,
    utils::{get_block_devices, save_config_value},
};

use cursive::{
    align::HAlign,
    traits::Nameable,
    view::Resizable,
    views::{Dialog, NamedView, ResizedView, SelectView},
};

type PDEVView = ResizedView<NamedView<Dialog>>;

pub fn get_pdev(map: HashMap<String, String>) -> PDEVView {
    let title = "Choose persist disk";
    let pdev = map.get(PERSIST_DISK).unwrap().clone();
    let mut value: usize = 0;

    let mut bv: SelectView<String> = SelectView::new().h_align(HAlign::Center).autojump();
    let mut i = 0;

    let devices = get_block_devices();
    for d in devices.unwrap() {
        if d == pdev {
            value = i;
        }
        //if !d.contains(&idev) {
            bv.add_item(d.clone(), d.clone());
            i += 1;
        //}
    }

    let d = Dialog::new().title(title).content(
        bv.selected(value)
            .on_submit(move |s, v| herr!(s, save_config_value, PERSIST_DISK, v, true))
            .fixed_width(10),
    );
    d.with_name(PERSIST_DISK).full_height()
}
