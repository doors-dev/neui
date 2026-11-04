#!/bin/sh
tailwindcss -i input.css -o output.css
sed -i '' 's/--tw-/--kisland-tw-/g' output.css
