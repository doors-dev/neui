#!/bin/sh
tailwindcss -i input.css -o output.css 
sed -i '' 's/--tw-/--nechat-tw-/g' output.css
