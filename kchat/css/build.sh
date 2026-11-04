#!/bin/sh
tailwindcss -i input.css -o output.css 
sed -i '' 's/--tw-/--kchat-tw-/g' output.css
