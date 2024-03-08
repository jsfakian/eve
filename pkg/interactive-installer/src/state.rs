use std::fmt;

use crate::data::Data;

#[derive(Debug, Copy, Clone)]
pub enum CurrentState {
    INIT,
    FS,
    Raid,
    NIC,
    Networking,
    IDEV,
    PDEV,
    Config,
    Overview,
}

impl fmt::Display for CurrentState {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

impl CurrentState {
    pub fn next(&self) -> Self {
        use CurrentState::*;
        match *self {
            INIT => FS,
            FS => Raid,
            Raid => NIC,
            NIC => Networking,
            Networking => IDEV,
            IDEV => PDEV,
            PDEV => Config,
            Config => Overview,
            Overview => Overview,
        }
    }

    pub fn prev(&self) -> Self {
        use CurrentState::*;
        match *self {
            INIT => INIT,
            FS => FS,
            Raid => FS,
            NIC => Raid,
            Networking => NIC,
            IDEV => Networking,
            PDEV => IDEV,
            Config => PDEV,
            Overview => Config,
        }
    }
}

pub enum Move {
    Previous,
    Next,
}

#[derive(Clone)]
pub struct GlobalState {
    pub data: Data,
    pub current_state: CurrentState,
}
