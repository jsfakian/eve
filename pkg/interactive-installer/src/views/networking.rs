use std::collections::HashMap;

use crate::{
    data::{DNS, GATEWAY, NETWORKING, SUBNET, DHCP, STATIC, STATIC_NET_CONFIG},
    herr,
    utils::{save_config_value},
    actions::execute,
    state::{Move},
};

use cursive::{
    Cursive,
    traits::Nameable,
    view::Resizable,
    views::{EditView, ListView, NamedView, ResizedView, Dialog, LinearLayout, SelectView},
};

fn get_net_index(value: &str) -> usize {
    match value {
        "DHCP" => 0,
        "Static" => 1,
        &_ => 0,
    }
}

type NetworkingView = ResizedView<NamedView<Dialog>>;

pub fn get_networking(map: HashMap<String, String>) -> NetworkingView {
    let title = "Networking configuration";
    // We need to pre-create the groups for our RadioButtons.
    //l.with_name(NETWORKING).full_height();

    let l = LinearLayout::vertical()
            .child(
                SelectView::new()
                    .item(DHCP, DHCP)
                    .item(STATIC, STATIC)
                    .selected(get_net_index(&map.get(NETWORKING).unwrap().clone()))
                    .on_submit(move |s, v| herr!(s, network_config, v, map.clone()))
                    //.on_submit(move |s, v| herr!(s, save_config_value, NETWORKING, v, true))
                    .fixed_width(10),
            );
    let d = Dialog::new().title(title).content(l);
    d.with_name(NETWORKING).full_height()
}

fn network_config(c: &mut Cursive, v: &str, map: HashMap<String, String>) -> crate::error::Result<()> {
    let l = ListView::new()
        .child(
            SUBNET,
            EditView::new()
                .content(map.get(SUBNET).unwrap().clone())
                .on_edit(move |s, v, _| {
                    herr!(s, save_config_value, SUBNET, v.to_string().as_str(), false)
                }),
        )
        .child(
            GATEWAY,
            EditView::new()
                .content(map.get(GATEWAY).unwrap().clone())
                .on_edit(move |s, v, _| {
                    herr!(s, save_config_value, GATEWAY, v.to_string().as_str(), false)
                }),
        )
        .child(
            DNS,
            EditView::new()
                .content(map.get(DNS).unwrap().clone())
                .on_edit(move |s, v, _| {
                    herr!(s, save_config_value, DNS, v.to_string().as_str(), false)
                }),
        );
    if v == DHCP {
        save_config_value(c, NETWORKING, v, true)
    } else {
        let d = Dialog::new().title(STATIC_NET_CONFIG)
            .content(l)
            .button("Ok", move |s| {
                s.pop_layer();
                let _ = execute(s, Move::Next);
            })
            .with_name(STATIC_NET_CONFIG);
        c.add_layer(d);
        save_config_value(c, NETWORKING, v, false)
    }
}
