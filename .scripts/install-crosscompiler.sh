#!/usr/bin/env bash

set -euo pipefail

sudo mkdir -p "${APT_CACHE}/archives/partial"

# Redirect apt cache to something we can put into an GH Actions cache
# Credits: https://unix.stackexchange.com/a/261169/17409
cat <<APTCONF | sudo tee --append /etc/apt/apt.conf
Dir{Cache ${APT_CACHE}}
Dir::Cache ${APT_CACHE};
APTCONF

sudo apt-get update \
&& sudo apt-get install -y \
    'gcc-14-aarch64-linux-gnu-base' \
    'g++-aarch64-linux-gnu'

# Change owner of cache files so pickup works later
sudo chown --recursive "$(id -u):$(id -g)" "${APT_CACHE}"
