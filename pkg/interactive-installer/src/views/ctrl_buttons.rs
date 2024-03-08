/*
 * rbmenu-tui - RBMenu TUI
 * Copyright (C) 2022 DevHyperCoder
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

use cursive::{
    traits::Nameable,
    view::Resizable,
    views::{Button, ListView, NamedView, OnEventView, ResizedView},
    Cursive,
};

use crate::state::Move;
use crate::{
    actions::execute,
    data::{BUTTONS, INSTALLER_CFG_OUT},
    herr,
    state::GlobalState,
};

//type CmdLine = OnEventView<ResizedView<NamedView<ListView>>>;
type CmdLine = OnEventView<ResizedView<NamedView<ListView>>>;

pub fn buttons(final_state: bool) -> CmdLine {
    let mut l = ListView::new().child(
        "Navigation",
        Button::new_raw("previous", |c| herr!(c, execute, Move::Previous)),
    );
    if final_state {
        l.add_child(
            "",
            Button::new_raw("finish", |c| herr!(c, write_config_and_quit)),
        );
    } else {
        l.add_child(
            "",
            Button::new_raw("next", |c| herr!(c, execute, Move::Next)),
        );
    }
    OnEventView::new(l.with_name(BUTTONS).full_width())
}

fn write_config_and_quit(c: &mut Cursive) -> crate::error::Result<()> {
    let mut d = c.take_user_data::<GlobalState>().unwrap().data;
    d.write(INSTALLER_CFG_OUT)?;
    c.quit();
    Ok(())
}
