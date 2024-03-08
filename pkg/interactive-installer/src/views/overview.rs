use std::collections::HashMap;

use crate::data::OVERVIEW;

use cursive::{
    traits::Nameable,
    view::Resizable,
    views::{ListView, NamedView, ResizedView, TextView},
};

type Overview = ResizedView<NamedView<ListView>>;

pub fn get_overview(map: HashMap<String, String>) -> Overview {
    let mut l = ListView::new();
    for (k, v) in map {
        l.add_child(&k, TextView::new(v));
    }
    l.with_name(OVERVIEW).full_height()
}
