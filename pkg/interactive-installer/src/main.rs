use serde_json::{json, Value};
use std::env;

mod actions;
mod data;
mod error;
mod installer;
mod state;
mod utils;
mod views;

fn help() {
    println!(
        "Usage: tui-cursive <installer.json>
    input file <installer.json> is optional."
    );
}

fn main() {
    let mut installer_json = json!(null);
    let args: Vec<String> = env::args().collect();

    match args.len() {
        // no arguments passed
        1 => {
            println!("Interactive installer mode!");
        } 
        2 => {
            installer_json = {
                let text = std::fs::read_to_string(&args[1]).unwrap();
                // Parse the string into a dynamically-typed JSON structure.
                serde_json::from_str::<Value>(&text).unwrap()
            }
        }
        // all the other cases
        _ => {
            // show a help message
            help();
        }
    }

    println!("Initializing EVE config!");
    installer::config(installer_json);
    installer::run();
}
