#!/bin/sh
set -e

postfix start

exec "$@"
