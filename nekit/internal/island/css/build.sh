#!/bin/sh
tailwindcss -i input.css -o output.css
sed -i '' 's/--tw-/--neisland-tw-/g' output.css
