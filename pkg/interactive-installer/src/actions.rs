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
    align::HAlign, views::{LinearLayout, TextView}, Cursive
};

use crate::{
    data::{INIT, FS, NIC, RAID},
    error::Result,
    state::{CurrentState, GlobalState},
    utils::get_state_mut,
    views::{
        config::get_config, ctrl_buttons::buttons, fs_select::get_fs, idev::get_idev,
        networking::get_networking, nic::get_nic, overview::get_overview, pdev::get_pdev,
        raid::get_raid,
    },
};

use crate::state::Move;

fn new_state(state: GlobalState) -> (Box<(dyn cursive::View + 'static)>, bool) {
    let map = state.data.map.clone();
    match state.current_state {
        CurrentState::INIT => {
            return (Box::new(get_fs(map.get(INIT).unwrap().clone())), false);
        }
        CurrentState::FS => {
            return (Box::new(get_fs(map.get(FS).unwrap().clone())), false);
        }
        CurrentState::Raid => {
            return (Box::new(get_raid(map.get(RAID).unwrap().clone(), map.get(FS).unwrap().clone())), false);
        }
        CurrentState::NIC => {
            return (Box::new(get_nic(map.get(NIC).unwrap().clone())), false);
        }
        CurrentState::Networking => {
            return (Box::new(get_networking(map)), false);
        }
        CurrentState::IDEV => {
            return (Box::new(get_idev(map)), false);
        }
        CurrentState::PDEV => {
            return (Box::new(get_pdev(map)), false);
        }
        CurrentState::Config => {
            return (Box::new(get_config(map)), false);
        }
        CurrentState::Overview => {
            return (Box::new(get_overview(map)), true);
        }
    };
}

fn navigate(c: &mut Cursive, state: GlobalState) {
    let (view, final_state) = new_state(state);
    c.pop_layer();
    c.add_fullscreen_layer(
        LinearLayout::vertical()
            .child(TextView::new("Installer").h_align(HAlign::Center))
            .child(view)
            .child(buttons(final_state)),
    );
}

fn state_move(state: &mut GlobalState, m: Move) {
    match m {
        Move::Previous => {
            state.current_state = state.current_state.prev();
        }
        Move::Next => {
            state.current_state = state.current_state.next();
        }
    }
}

pub fn execute(c: &mut Cursive, m: Move) -> Result<()> {
    let mut state = get_state_mut(c)?;
    state_move(&mut state, m);
    c.set_user_data(state.clone());
    navigate(c, state);

    Ok(())
}
