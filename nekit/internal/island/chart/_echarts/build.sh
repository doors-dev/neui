#!/bin/sh
# Copyright 2026 doors dev LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e
npm ci
npx esbuild entry.js --bundle --format=esm --minify --target=es2022 --legal-comments=none \
	--define:process.env.NODE_ENV='"production"' \
	--banner:js='/*! Apache ECharts 6.1.0 (Apache-2.0, see NOTICE and LICENSE) with ZRender 6.1.0 (BSD-3-Clause, see LICENSE-zrender) and code from d3 (BSD-3-Clause, see LICENSE-d3) */' \
	--outfile=echarts.js
cp node_modules/echarts/LICENSE LICENSE
cp node_modules/echarts/NOTICE NOTICE
cp node_modules/echarts/licenses/LICENSE-d3 LICENSE-d3
cp node_modules/zrender/LICENSE LICENSE-zrender
