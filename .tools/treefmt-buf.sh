#!/usr/bin/env bash
# Wrapper script for buf format to work with treefmt
# treefmt passes individual files, but we just run buf format on the entire proto/ directory
set -euo pipefail

buf format -w .
