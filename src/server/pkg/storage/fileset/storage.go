package fileset

import (
	"context"
	"io"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/pachyderm/pachyderm/src/server/pkg/obj"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/chunk"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/fileset/index"
)

const (
	headerTag = ""
	prefix    = "pfs"
	// DefaultMemoryThreshold is the default for the memory threshold that must
	// be met before a file set part is serialized (excluding close).
	DefaultMemoryThreshold = 1024 * chunk.MB
	// DefaultShardThreshold is the default for the size threshold that must
	// be met before a shard is created by the shard function.
	DefaultShardThreshold = 1024 * chunk.MB
	// DefaultLevelZeroSize is the default size for level zero in the compacted
	// representation of a file set.
	DefaultLevelZeroSize = 1 * chunk.MB
	// DefaultLevelSizeBase is the default base for the exponential growth function
	// for level sizes in the compacted representation of a file set.
	DefaultLevelSizeBase = 10
	// Diff is the suffix of a path that points to the diff of the prefix.
	Diff = "diff"
	// Compacted is the suffix of a path that points to the compaction of the prefix.
	Compacted = "compacted"
)

// ShardFunc is a callback that returns a PathRange for each shard.
type ShardFunc func(*index.PathRange) error

// Storage is the abstraction that manages fileset storage.
type Storage struct {
	objC                         obj.Client
	chunks                       *chunk.Storage
	memThreshold, shardThreshold int64
	levelZeroSize                int64
	levelSizeBase                int
}

// NewStorage creates a new Storage.
func NewStorage(objC obj.Client, chunks *chunk.Storage, opts ...StorageOption) *Storage {
	s := &Storage{
		objC:           objC,
		chunks:         chunks,
		memThreshold:   DefaultMemoryThreshold,
		shardThreshold: DefaultShardThreshold,
		levelZeroSize:  DefaultLevelZeroSize,
		levelSizeBase:  DefaultLevelSizeBase,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// New creates a new in-memory fileset.
func (s *Storage) New(ctx context.Context, fileSet string, opts ...Option) *FileSet {
	fileSet = applyPrefix(fileSet)
	return newFileSet(ctx, s, fileSet, s.memThreshold, opts...)
}

// NewWriter creates a new Writer.
func (s *Storage) NewWriter(ctx context.Context, fileSet string) *Writer {
	fileSet = applyPrefix(fileSet)
	return s.newWriter(ctx, fileSet)
}

func (s *Storage) newWriter(ctx context.Context, fileSet string) *Writer {
	return newWriter(ctx, s.objC, s.chunks, fileSet)
}

// NewReader creates a new Reader for a file set.
// (bryce) expose some notion of read ahead (read a certain number of chunks in parallel).
// this will be necessary to speed up reading large files.
func (s *Storage) NewReader(ctx context.Context, fileSet string, opts ...index.Option) *Reader {
	fileSet = applyPrefix(fileSet)
	return s.newReader(ctx, fileSet, opts...)
}

func (s *Storage) newReader(ctx context.Context, fileSet string, opts ...index.Option) *Reader {
	return newReader(ctx, s.objC, s.chunks, fileSet, opts...)
}

func (s *Storage) NewMergeReader(ctx context.Context, fileSet string, opts ...index.Option) *MergeReader {
	fileSet = applyPrefix(fileSet)
	var rs []*Reader
	for _, fileSet := range fileSets {
		if err := s.objC.Walk(ctx, path.Join(fileSet, Compacted), func(name string) error {
			rs = append(rs, s.newReader(ctx, name, opts...))
			return nil
		}); err != nil {
			return err
		}
	}
	return s.newMergeReader(ctx, rs)
}

// Shard shards the merge of the file sets with the passed in prefix into file ranges.
// (bryce) this should be extended to be more configurable (different criteria
// for creating shards).
func (s *Storage) Shard(ctx context.Context, fileSets []string, shardFunc ShardFunc) error {
	fileSets = applyPrefixes(fileSets)
	return s.merge(ctx, fileSets, shardMergeFunc(s.shardThreshold, shardFunc))
}

func (s *Storage) Compact(ctx context.Context, outputFileSet string, inputFileSets []string, opts ...index.Option) error {
	outputFileSet = applyPrefix(outputFileSet)
	inputFileSets = applyPrefixes(inputFileSets)
	w := s.newWriter(ctx, outputFileSet)
	if err := s.merge(ctx, inputFileSets, contentMergeFunc(w), opts...); err != nil {
		return err
	}
	return w.Close()
}

type CompactSpec struct {
	Output string
	Input  []string
}

func (s *Storage) CompactSpec(ctx context.Context, fileSet, compactedFileSet string) (*CompactSpec, error) {
	fileSet = applyPrefix(fileSet)
	compactedFileSet = applyPrefix(compactedFileSet)
	hdr, err := index.GetTopLevelIndex(ctx, s.objC, path.Join(fileSet, Diff))
	if err != nil {
		return nil, err
	}
	var level int
	size := hdr.Idx.SizeBytes
	spec := &CompactSpec{
		Input: []string{path.Join(fileSet, Diff)},
	}
	if err := s.objC.Walk(ctx, path.Join(compactedFileSet, Compacted), func(name string) error {
		nextLevel, err := strconv.Atoi(path.Base(name))
		if err != nil {
			return err
		}
		// Handle levels that are non-empty.
		if nextLevel == level {
			hdr, err := index.GetTopLevelIndex(ctx, s.objC, name)
			if err != nil {
				return err
			}
			size += hdr.Idx.SizeBytes
			// If the output level has not been determined yet, then the current level will be an input
			// to the compaction.
			// If the output level has been determined, then the current level will be copied.
			// The copied levels are above the output level.
			if spec.Output == "" {
				spec.Input = append(spec.Input, name)
			} else {
				w, err := s.objC.Writer(ctx, path.Join(fileSet, Compacted, strconv.Itoa(level)))
				if err != nil {
					return err
				}
				r, err := s.objC.Reader(ctx, name, 0, 0)
				if err != nil {
					return err
				}
				if _, err := io.Copy(w, r); err != nil {
					return err
				}
			}
		}
		// If the output level has not been determined yet and the compaction size is less than the threshold for
		// the current level, then the current level becomes the output level.
		if spec.Output == "" && size < s.levelZeroSize*int64(math.Pow(float64(s.levelSizeBase), float64(level))) {
			spec.Output = path.Join(fileSet, Compacted, strconv.Itoa(level))
		}
		level++
		return nil
	}); err != nil {
		return nil, err
	}
	return spec, nil
}

func (s *Storage) merge(ctx context.Context, fileSets []string, f mergeFunc, opts ...index.Option) error {
	var rs []*Reader
	for _, fileSet := range fileSets {
		if err := s.objC.Walk(ctx, fileSet, func(name string) error {
			rs = append(rs, s.newReader(ctx, name, opts...))
			return nil
		}); err != nil {
			return err
		}
	}
	var fileStreams []stream
	for _, r := range rs {
		fileStreams = append(fileStreams, &fileStream{r: r})
	}
	return merge(fileStreams, f)
}

func applyPrefix(fileSet string) string {
	if strings.HasPrefix(fileSet, prefix) {
		return fileSet
	}
	return path.Join(prefix, fileSet)
}

func applyPrefixes(fileSets []string) []string {
	var prefixedFileSets []string
	for _, fileSet := range fileSets {
		prefixedFileSets = append(prefixedFileSets, applyPrefix(fileSet))
	}
	return prefixedFileSets
}
