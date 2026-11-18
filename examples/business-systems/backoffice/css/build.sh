#!/bin/sh
cd "$(dirname "$0")" && tailwindcss -i input.css -o output.css
