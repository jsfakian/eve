#!/bin/sh
# Copyright (c) 2023 Zededa, Inc.
# SPDX-License-Identifier: Apache-2.0

set -e

# Enable all available FCC-unlock scripts.
# For more info, see: https://modemmanager.org/docs/modemmanager/fcc-unlock/
enable_fcc_unlock() {
  SOURCE_DIR="/usr/share/ModemManager/fcc-unlock.available.d/"
  TARGET_DIR="/etc/ModemManager/fcc-unlock.d/"

  for SCRIPT in "${SOURCE_DIR}"*; do
    if [ -f "$SCRIPT" ]; then
      SCRIPT_NAME="$(basename "$SCRIPT")"
      case "$SCRIPT_NAME" in
        "1eac:1002" | "2c7c:030a" | "2c7c:0311")
          # For these modems we have our own custom scripts.
          continue
          ;;
      esac
      ln -sf "$SCRIPT" "${TARGET_DIR}${SCRIPT_NAME}"
    fi
  done
}

echo "Loading kernel modules used by ModemManager"
modprobe -a qcserial usb_wwan qmi_wwan cdc_wdm cdc_mbim cdc_acm \
            wwan mhi mhi_pci_generic mhi_wwan_ctrl mhi_wwan_mbim
echo "Kernel modules are loaded"

echo "Starting D-Bus daemon"
mkdir -p /var/run/dbus
dbus-daemon --system
echo "D-Bus daemon started"

echo "Starting Udev daemon"
udevd --debug --daemon 2>/dev/null
# Apply installed ModemManager udev rules.
udevadm control --reload
udevadm trigger
echo "Udev daemon started"

echo "Starting Modem Manager"
enable_fcc_unlock
ModemManager --debug &

echo "Starting Modem Manager Agent"
# Monitor liveness of the agent (and Modem Manager) with watchdog.
mkdir -p /run/watchdog/file
touch /run/watchdog/file/wwan.touch
mmagent
