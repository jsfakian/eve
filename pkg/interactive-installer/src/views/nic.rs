/*
 * installer-tui
 * Copyright (C) 2022 Ioannis Sfakianakis
 */

use crate::{data::NIC, herr, utils::save_config_value};

use cursive::{
    align::HAlign,
    traits::Nameable,
    view::Resizable,
    views::{Dialog, NamedView, ResizedView, SelectView},
};

//use network_interface::{NetworkInterface, NetworkInterfaceConfig};
use nix;

type NICView = ResizedView<NamedView<Dialog>>;

pub fn get_nic(value: String) -> NICView {
    let title = "Choose NIC";
    let mut selected: usize = 0;

    //let network_interfaces = NetworkInterface::show().unwrap();
    let mut niv = SelectView::new().h_align(HAlign::Center).autojump();
    let mut i: usize = 0;
    /*for itf in network_interfaces.iter() {
        if value == itf.name {
            selected = i;
        }
        niv.add_item(itf.name.clone(), itf.name.clone());
        i += 1;
    }*/
    let interfaces = nix::ifaddrs::getifaddrs().unwrap();
    for interface in interfaces {
        if interface.interface_name == value {
            selected = i;
        }
        niv.add_item(interface.interface_name.clone(), interface.interface_name.clone());
        i += 1;
    }

    let d = Dialog::new().title(title).content(
        niv.selected(selected)
            .on_submit(move |s, v| herr!(s, save_config_value, NIC, v, true))
            .fixed_width(10),
    );
    d.with_name(NIC).full_height()
}
