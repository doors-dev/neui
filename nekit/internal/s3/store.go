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

package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/minio/minio-go/v7"
	"neui.dev/api"
)

const keyLimit = 1024

var partSize = 16 << 20

// Store keeps uploaded files in an S3-compatible bucket. Plug it in with
// neui.WithUploadStorage or neui.OptionUploadStorage. The service must support
// conditional writes (If-None-Match), as AWS S3 and MinIO do.
type Store struct {
	client *minio.Client
	core   minio.Core
	bucket string
	prefix string
}

func New(client *minio.Client, bucket, prefix string) *Store {
	prefix = strings.TrimRight(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &Store{client: client, core: minio.Core{Client: client}, bucket: bucket, prefix: prefix}
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && utf8.ValidString(name) && !strings.ContainsAny(name, "/\x00")
}

func (s *Store) checkFile(threadID api.ThreadID, origin api.Origin, name string) error {
	if !validName(string(threadID)) {
		return fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID)
	}
	if !validName(name) {
		return fmt.Errorf("%w %q", api.ErrInvalidName, name)
	}
	if !origin.Valid() {
		return fmt.Errorf("unknown origin %q", origin)
	}
	if len(s.prefix)+len(threadID)+len("/.committed/")+len(origin)+len("/")+len(name) > keyLimit {
		return fmt.Errorf("%w %q: key longer than %d bytes", api.ErrInvalidName, name, keyLimit)
	}
	return nil
}

func s3Error(err error, op string, key string) error {
	resp := minio.ToErrorResponse(err)
	switch {
	case resp.Code == minio.NoSuchKey || resp.Code == "NotFound":
		return fmt.Errorf("s3 %s %s: %w", op, key, api.ErrNotFound)
	case resp.Code == minio.PreconditionFailed || resp.Code == "ConditionalRequestConflict" || resp.StatusCode == http.StatusPreconditionFailed:
		return fmt.Errorf("s3 %s %s: %w", op, key, api.ErrExists)
	case resp.Code == minio.InvalidRange || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		return fmt.Errorf("s3 %s %s: %w", op, key, api.ErrOutOfRange)
	}
	return fmt.Errorf("s3 %s %s: %w", op, key, err)
}

func (s *Store) FilePut(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, mime string, r io.Reader, draft bool) (api.File, error) {
	if err := ctx.Err(); err != nil {
		return api.File{}, err
	}
	if err := s.checkFile(threadID, origin, name); err != nil {
		return api.File{}, err
	}
	key := s.prefix + string(threadID) + "/" + string(origin) + "/" + name
	marker := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/" + name
	if _, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err == nil {
		return api.File{}, fmt.Errorf("s3 put %s: %w", key, api.ErrExists)
	} else if err := s3Error(err, "stat", key); !errors.Is(err, api.ErrNotFound) {
		return api.File{}, err
	}
	created := false
	if draft {
		if err := s.client.RemoveObject(ctx, s.bucket, marker, minio.RemoveObjectOptions{}); err != nil {
			return api.File{}, s3Error(err, "delete", marker)
		}
	} else {
		mopts := minio.PutObjectOptions{}
		mopts.SetMatchETagExcept("*")
		if _, err := s.core.PutObject(ctx, s.bucket, marker, bytes.NewReader(nil), 0, "", "", mopts); err == nil {
			created = true
		} else if err := s3Error(err, "put", marker); !errors.Is(err, api.ErrExists) {
			return api.File{}, err
		}
	}
	opts := minio.PutObjectOptions{ContentType: mime}
	if mime == "" {
		opts.UserMetadata = map[string]string{"No-Mime": "1"}
	}
	opts.SetMatchETagExcept("*")
	buf := make([]byte, partSize)
	var size int64
	var uploadID string
	var parts []minio.CompletePart
	var err error
	for {
		k := 0
		eof := false
		for k < len(buf) && !eof && err == nil {
			var m int
			m, err = r.Read(buf[k:])
			k += m
			if err == io.EOF {
				eof = true
				err = nil
			}
		}
		if err != nil {
			break
		}
		size += int64(k)
		if uploadID == "" && eof {
			if _, err = s.core.PutObject(ctx, s.bucket, key, bytes.NewReader(buf[:k]), int64(k), "", "", opts); err != nil {
				err = s3Error(err, "put", key)
			}
			break
		}
		if uploadID == "" {
			if uploadID, err = s.core.NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{ContentType: mime, UserMetadata: opts.UserMetadata}); err != nil {
				err = s3Error(err, "start upload", key)
				break
			}
		}
		if k > 0 {
			var part minio.ObjectPart
			if part, err = s.core.PutObjectPart(ctx, s.bucket, key, uploadID, len(parts)+1, bytes.NewReader(buf[:k]), int64(k), minio.PutObjectPartOptions{}); err != nil {
				err = s3Error(err, "upload part", key)
				break
			}
			parts = append(parts, minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag})
		}
		if eof {
			complete := minio.PutObjectOptions{}
			complete.SetMatchETagExcept("*")
			if _, err = s.core.CompleteMultipartUpload(ctx, s.bucket, key, uploadID, parts, complete); err != nil {
				err = s3Error(err, "complete upload", key)
			}
			break
		}
	}
	if err != nil && uploadID != "" {
		s.core.AbortMultipartUpload(context.WithoutCancel(ctx), s.bucket, key, uploadID)
	}
	ctx = context.WithoutCancel(ctx)
	if err != nil {
		if created {
			s.client.RemoveObject(ctx, s.bucket, marker, minio.RemoveObjectOptions{})
		}
		return api.File{}, err
	}
	f := api.File{
		ThreadID:  threadID,
		Name:      name,
		MIME:      mime,
		Origin:    origin,
		Size:      size,
		CreatedAt: time.Now(),
		Draft:     draft,
	}
	if info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err == nil {
		f.CreatedAt = info.LastModified
	}
	return f, nil
}

func (s *Store) FileCommit(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkFile(threadID, origin, name); err != nil {
		return err
	}
	key := s.prefix + string(threadID) + "/" + string(origin) + "/" + name
	marker := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/" + name
	if _, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err != nil {
		return s3Error(err, "stat", key)
	}
	if _, err := s.core.PutObject(ctx, s.bucket, marker, bytes.NewReader(nil), 0, "", "", minio.PutObjectOptions{}); err != nil {
		return s3Error(err, "put", marker)
	}
	return nil
}

func (s *Store) FileDiscardDrafts(ctx context.Context, threadID api.ThreadID, origin api.Origin, keep []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validName(string(threadID)) {
		return fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID)
	}
	if !origin.Valid() {
		return fmt.Errorf("unknown origin %q", origin)
	}
	dir := s.prefix + string(threadID) + "/" + string(origin) + "/"
	markers := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/"
	var names []string
	for obj := range s.client.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: dir}) {
		if obj.Err != nil {
			return s3Error(obj.Err, "list", dir)
		}
		if name := strings.TrimPrefix(obj.Key, dir); validName(name) && !slices.Contains(keep, name) {
			names = append(names, name)
		}
	}
	committed := map[string]bool{}
	for obj := range s.client.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: markers}) {
		if obj.Err != nil {
			return s3Error(obj.Err, "list", markers)
		}
		committed[strings.TrimPrefix(obj.Key, markers)] = true
	}
	for _, name := range names {
		if committed[name] {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.bucket, dir+name, minio.RemoveObjectOptions{}); err != nil {
			return s3Error(err, "delete", dir+name)
		}
	}
	return nil
}

func (s *Store) FileStat(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) (api.File, error) {
	if err := ctx.Err(); err != nil {
		return api.File{}, err
	}
	if err := s.checkFile(threadID, origin, name); err != nil {
		return api.File{}, err
	}
	key := s.prefix + string(threadID) + "/" + string(origin) + "/" + name
	marker := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/" + name
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return api.File{}, s3Error(err, "stat", key)
	}
	f := api.File{
		ThreadID:  threadID,
		Name:      name,
		MIME:      info.ContentType,
		Origin:    origin,
		Size:      info.Size,
		CreatedAt: info.LastModified,
	}
	if info.UserMetadata["No-Mime"] != "" {
		f.MIME = ""
	}
	if _, err := s.client.StatObject(ctx, s.bucket, marker, minio.StatObjectOptions{}); err != nil {
		if err := s3Error(err, "stat", marker); !errors.Is(err, api.ErrNotFound) {
			return api.File{}, err
		}
		f.Draft = true
	}
	return f, nil
}

func (s *Store) FileRead(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, off int64, n int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.checkFile(threadID, origin, name); err != nil {
		return nil, err
	}
	if off < 0 {
		return nil, fmt.Errorf("negative offset %d", off)
	}
	key := s.prefix + string(threadID) + "/" + string(origin) + "/" + name
	opts := minio.GetObjectOptions{}
	switch {
	case n == 0:
		opts.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off))
	case n > 0 && n <= math.MaxInt64-off:
		opts.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	case n > 0 || off > 0:
		opts.Set("Range", fmt.Sprintf("bytes=%d-", off))
	}
	body, _, _, err := s.core.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, s3Error(err, "get", key)
	}
	if n == 0 {
		body.Close()
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return body, nil
}

func (s *Store) FileList(ctx context.Context, threadID api.ThreadID, origin api.Origin) iter.Seq2[api.File, error] {
	return func(yield func(api.File, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(api.File{}, err)
			return
		}
		if !validName(string(threadID)) {
			yield(api.File{}, fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID))
			return
		}
		if !origin.Valid() {
			yield(api.File{}, fmt.Errorf("unknown origin %q", origin))
			return
		}
		dir := s.prefix + string(threadID) + "/" + string(origin) + "/"
		markers := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/"
		var names []string
		for obj := range s.client.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: dir}) {
			if obj.Err != nil {
				yield(api.File{}, s3Error(obj.Err, "list", dir))
				return
			}
			if name := strings.TrimPrefix(obj.Key, dir); validName(name) {
				names = append(names, name)
			}
		}
		committed := map[string]bool{}
		for obj := range s.client.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: markers}) {
			if obj.Err != nil {
				yield(api.File{}, s3Error(obj.Err, "list", markers))
				return
			}
			committed[strings.TrimPrefix(obj.Key, markers)] = true
		}
		sort.Strings(names)
		for _, name := range names {
			info, err := s.client.StatObject(ctx, s.bucket, dir+name, minio.StatObjectOptions{})
			if err != nil && ctx.Err() != nil {
				yield(api.File{}, ctx.Err())
				return
			}
			if err != nil {
				continue
			}
			f := api.File{
				ThreadID:  threadID,
				Name:      name,
				MIME:      info.ContentType,
				Origin:    origin,
				Size:      info.Size,
				CreatedAt: info.LastModified,
				Draft:     !committed[name],
			}
			if info.UserMetadata["No-Mime"] != "" {
				f.MIME = ""
			}
			if !yield(f, nil) {
				return
			}
		}
	}
}

func (s *Store) FileDelete(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkFile(threadID, origin, name); err != nil {
		return err
	}
	key := s.prefix + string(threadID) + "/" + string(origin) + "/" + name
	marker := s.prefix + string(threadID) + "/.committed/" + string(origin) + "/" + name
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return s3Error(err, "delete", key)
	}
	if err := s.client.RemoveObject(ctx, s.bucket, marker, minio.RemoveObjectOptions{}); err != nil {
		return s3Error(err, "delete", marker)
	}
	return nil
}

func (s *Store) FileDeleteAll(ctx context.Context, threadID api.ThreadID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validName(string(threadID)) {
		return fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID)
	}
	dir := s.prefix + string(threadID) + "/"
	var listErr error
	objects := func(yield func(minio.ObjectInfo) bool) {
		for obj := range s.client.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: dir, Recursive: true}) {
			if obj.Err != nil {
				listErr = obj.Err
				return
			}
			if !yield(obj) {
				return
			}
		}
	}
	results, err := s.client.RemoveObjectsWithIter(ctx, s.bucket, objects, minio.RemoveObjectsOptions{})
	if err != nil {
		return s3Error(err, "delete", dir)
	}
	for res := range results {
		if res.Err != nil {
			return s3Error(res.Err, "delete", res.ObjectName)
		}
	}
	if listErr != nil {
		return s3Error(listErr, "list", dir)
	}
	return nil
}
