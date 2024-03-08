/*
 * installer-tui
 * Copyright (C) 2022 Ioannis Sfakianakis
 */

use crate::{data::RAID, herr, utils::save_config_value};

use cursive::{
    traits::Nameable,
    view::Resizable,
    views::{Dialog, NamedView, ResizedView, SelectView},
};

type RaidView = ResizedView<NamedView<Dialog>>;

fn get_raid_index(value: &str) -> usize {
    match value {
        "No raid" => usize::MAX,
        "0" => 0,
        "1" => 1,
        "5" => 2,
        "10" => 3,
        &_ => 0,
    }
}

pub fn get_raid(raid: String, fs: String) -> RaidView {
    let key = "Choose RAID";
    let mut sel_view = SelectView::new()
        .item("No raid", "")
        .selected(get_raid_index(&raid))
        .on_submit(move |s, v| herr!(s, save_config_value, RAID, v, true));
    if fs == "ZFS" {
        sel_view.add_item("0", "0");
        sel_view.add_item("1", "1");
        sel_view.add_item("5", "5");
        sel_view.add_item("10", "10");
    }
    let d = Dialog::new().title(key).content(
        sel_view
    );
    d.with_name(RAID).full_height()
}
