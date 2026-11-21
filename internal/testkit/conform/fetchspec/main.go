// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type pin struct {
	File       string   `json:"file"`
	Source     string   `json:"source"`
	Commit     string   `json:"commit"`
	Gzipped    bool     `json:"-"`
	APIVersion string   `json:"apiVersion"`
	Roots      []string `json:"roots"`
	Upstream   string   `json:"upstreamSha256"`
	Sha256     string   `json:"sha256"`
	Fetched    string   `json:"fetched"`
}

var pins = []pin{
	{
		File:       "anthropic.json.gz",
		Source:     "https://raw.githubusercontent.com/anthropics/anthropic-sdk-go/%s/scripts/mock-spec.json.gz",
		Commit:     "b98ff3342a95f1197d9b2ba2c1820311bec698d7",
		Gzipped:    true,
		APIVersion: "2026-10-21",
		Roots: []string{
			"/components/schemas/CreateMessageParams",
			"/components/schemas/CountMessageTokensParams",
			"/components/schemas/MessageStreamEvent",
			"/components/schemas/ErrorResponse",
		},
	},
	{
		File:       "openai.json.gz",
		Source:     "https://raw.githubusercontent.com/openai/openai-openapi/%s/openapi.json",
		Commit:     "20a459affe7aea4cc7333375802706e296a56011",
		APIVersion: "2.3.0",
		Roots: []string{
			"/components/schemas/CreateResponse",
			"/components/schemas/TokenCountsBody",
			"/components/schemas/ResponseStreamEvent",
			"/components/schemas/CreateChatCompletionRequest",
			"/components/schemas/CreateChatCompletionStreamResponse",
			"/components/schemas/ErrorResponse",
		},
	},
	{
		File:       "gemini.json.gz",
		Source:     "https://aistudio.google.com/api-reference/interactions-v1.openapi.json",
		Upstream:   "03fa4a5bee4fb8617b1f042e15335577f8a15111a97bf2ec524df36a61c30477",
		APIVersion: "2026-05-20",
		Roots: []string{
			"/components/schemas/CreateModelInteractionParams",
			"/components/schemas/InteractionSseEvent",
			"/paths/~1{api_version}~1interactions/post/responses/4XX/content/application~1json/schema",
		},
	},
}

const update = "Regenerate with: go run ./internal/testkit/conform/fetchspec. To move a pin, change its commit in fetchspec/main.go and commit the new specs and SPECS together. The Anthropic pin must describe the anthropic-version the claude adapter sends (its defaultVersion); anthropic-sdk-go b98ff33 is release 2.7.0, which sends 2026-10-21. Pass pin files as arguments to refresh only those; the others keep their committed spec and SPECS entry. The Gemini pin is a URL Google replaces in place, pinned by its upstream sha256, and must describe the Api-Revision the gemini adapter sends (its defaultRevision). A pin is never replaced silently: the suite checks sha256 of the decompressed spec against this file."

func main() {
	dir := flag.String("dir", "internal/testkit/conform/testdata/specs", "where the pinned specs and SPECS go")
	flag.Parse()
	named := map[string]bool{}
	for _, file := range flag.Args() {
		if !slices.ContainsFunc(pins, func(p pin) bool { return p.File == file }) {
			log.Fatalf("%s is not a pin in fetchspec/main.go", file)
		}
		named[file] = true
	}
	committed := map[string]pin{}
	if len(named) > 0 {
		data, err := os.ReadFile(filepath.Join(*dir, "SPECS"))
		if err != nil {
			log.Fatal(err)
		}
		var file struct {
			Specs []pin `json:"specs"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			log.Fatalf("SPECS: %v", err)
		}
		for _, p := range file.Specs {
			committed[p.File] = p
		}
	}
	gzipped := map[string][]byte{}
	var entries []pin
	for _, p := range pins {
		if len(named) > 0 && !named[p.File] {
			c, ok := committed[p.File]
			if !ok {
				log.Fatalf("%s is not in SPECS; name it to fetch it", p.File)
			}
			entries = append(entries, c)
			continue
		}
		source := p.Source
		if p.Commit != "" {
			source = fmt.Sprintf(p.Source, p.Commit)
		}
		raw, err := fetch(source, p.Gzipped)
		if err != nil {
			log.Fatal(err)
		}
		up := sha256.Sum256(raw)
		if p.Commit == "" && hex.EncodeToString(up[:]) != p.Upstream {
			log.Fatalf("%s: %s now serves sha256 %s, the pin is %s; the file is replaced in place: review the new spec, then set Upstream in fetchspec/main.go", p.File, source, hex.EncodeToString(up[:]), p.Upstream)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			log.Fatalf("%s: %v", p.File, err)
		}
		pruned, err := prune(doc, p.Roots)
		if err != nil {
			log.Fatalf("%s: %v", p.File, err)
		}
		body, err := json.Marshal(pruned)
		if err != nil {
			log.Fatal(err)
		}
		sum := sha256.Sum256(body)
		var gz bytes.Buffer
		w, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
		w.Write(body)
		w.Close()
		gzipped[p.File] = gz.Bytes()
		p.Source = source
		p.Upstream = hex.EncodeToString(up[:])
		p.Sha256 = hex.EncodeToString(sum[:])
		p.Fetched = time.Now().UTC().Format(time.DateOnly)
		entries = append(entries, p)
		log.Printf("%s: %d bytes upstream, %d pruned, %d gzipped", p.File, len(raw), len(body), gz.Len())
	}
	data, err := json.MarshalIndent(map[string]any{"update": update, "specs": entries}, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	for file, gz := range gzipped {
		if err := os.WriteFile(filepath.Join(*dir, file), gz, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(*dir, "SPECS"), append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

func fetch(url string, gzipped bool) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var r io.Reader = resp.Body
	if gzipped {
		if r, err = gzip.NewReader(resp.Body); err != nil {
			return nil, err
		}
	}
	return io.ReadAll(r)
}

func prune(doc map[string]any, roots []string) (map[string]any, error) {
	out := map[string]any{}
	seen := map[string]bool{}
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		ptr := queue[0]
		queue = queue[1:]
		if seen[ptr] {
			continue
		}
		seen[ptr] = true
		node, err := resolve(doc, ptr)
		if err != nil {
			return nil, err
		}
		place(out, ptr, node)
		walk(node, func(ref string) {
			if strings.HasPrefix(ref, "#/") {
				queue = append(queue, ref[1:])
			}
		})
	}
	return out, nil
}

func segments(ptr string) []string {
	parts := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts
}

func resolve(doc any, ptr string) (any, error) {
	node := doc
	for _, s := range segments(ptr) {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: %q is not inside an object", ptr, s)
		}
		if node, ok = m[s]; !ok {
			return nil, fmt.Errorf("%s: no %q", ptr, s)
		}
	}
	return node, nil
}

func place(out map[string]any, ptr string, node any) {
	parts := segments(ptr)
	m := out
	for _, s := range parts[:len(parts)-1] {
		next, ok := m[s].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[s] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = node
}

func walk(node any, ref func(string)) {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			if s, ok := v.(string); ok && k == "$ref" {
				ref(s)
				continue
			}
			walk(v, ref)
		}
	case []any:
		for _, v := range n {
			walk(v, ref)
		}
	}
}
