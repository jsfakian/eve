#!/bin/bash
# shellcheck shell=dash
if grep -q interactive /proc/cmdline; then
    echo "stop rungetty.sh to not re-run login"
    killall -STOP rungetty.sh
    echo "killing login"
    killall login
    echo "Running RUST installer"
    RUST_BACKTRACE=full /sbin/installer
    echo "resume rungetty.sh"
    killall -CONT rungetty.sh

    # Parse the installer JSON using jq and create an associative array
    declare -A assoc_array
    while IFS="=" read -r key value; do
        if [ "$value" == "" ];
        then
            continue
        fi
        key=$(echo "$key" | tr '[:upper:]' '[:lower:]')
        assoc_array["$key"]="$value"
    done < <(jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" "installer.json")

    # Create overide.json for networking config if networking is static
    if [ "${assoc_array['networking']}" == "Static" ]; then
        echo "Static Networking"
        cp override.json.tmpl override.json
        overide="override.json"
        # Update the JSON file with new values
        jq --arg subnet "${assoc_array['subnet']}" --arg gateway "${assoc_array['gateway']}"  --arg dns "${assoc_array['dns']}"\
            '.Ports[0].AddrSubnet = $subnet | .Ports[0].Gateway = $gateway | .Ports[0].DnsServers[0] = $dns' \
            "$overide" > tmp.$$.json && mv tmp.$$.json "$overide"
        mkdir -p /run/global/DevicePortConfig/
        mv "$overide" /run/global/DevicePortConfig/
    fi
fi