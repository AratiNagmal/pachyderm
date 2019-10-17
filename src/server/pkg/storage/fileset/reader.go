package fileset

import (
	"context"
	"io"

	"github.com/pachyderm/pachyderm/src/server/pkg/obj"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/chunk"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/fileset/index"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/fileset/tar"
)

// Reader reads the serialized format of a fileset.
type Reader struct {
	ctx     context.Context
	chunks  *chunk.Storage
	ir      *index.Reader
	cr      *chunk.Reader
	tr      *tar.Reader
	hdr     *index.Header
	peekHdr *index.Header
	tags    []*index.Tag
}

func newReader(ctx context.Context, objC obj.Client, chunks *chunk.Storage, path string, opts ...index.Option) *Reader {
	cr := chunks.NewReader(ctx)
	return &Reader{
		ctx:    ctx,
		chunks: chunks,
		ir:     index.NewReader(ctx, objC, chunks, path, opts...),
		cr:     cr,
	}
}

// Next returns the next header, and prepares the file's content for reading.
func (r *Reader) Next() (*index.Header, error) {
	var hdr *index.Header
	if r.peekHdr != nil {
		hdr, r.peekHdr = r.peekHdr, nil
	} else {
		var err error
		hdr, err = r.next()
		if err != nil {
			return nil, err
		}
	}
	r.hdr = hdr
	r.cr.NextRange(r.hdr.Idx.DataOp.DataRefs)
	r.tr = nil
	r.tags = hdr.Idx.DataOp.Tags[1:]
	return hdr, nil
}

func (r *Reader) next() (*index.Header, error) {
	hdr, err := r.ir.Next()
	if err != nil {
		return nil, err
	}
	// Convert index tar header to corresponding content tar header.
	indexToContentHeader(hdr)
	return hdr, nil
}

// Peek returns the next header without progressing the reader.
func (r *Reader) Peek() (*index.Header, error) {
	if r.peekHdr != nil {
		return r.peekHdr, nil
	}
	hdr, err := r.next()
	if err != nil {
		return nil, err
	}
	r.peekHdr = hdr
	return hdr, err
}

// PeekTag returns the next tag without progressing the reader.
func (r *Reader) PeekTag() (*index.Tag, error) {
	if len(r.tags) == 0 {
		return nil, io.EOF
	}
	return r.tags[0], nil
}

func indexToContentHeader(idx *index.Header) {
	idx.Hdr = &tar.Header{
		Name: idx.Hdr.Name,
		Size: idx.Idx.SizeBytes,
	}
}

// Read reads from the current file in the tar stream.
func (r *Reader) Read(data []byte) (int, error) {
	// Lazily setup reader for underlying file.
	if err := r.setupReader(); err != nil {
		return 0, err
	}
	return r.tr.Read(data)
}

func (r *Reader) setupReader() error {
	if r.tr == nil {
		r.tr = tar.NewReader(r.cr)
		// Remove tar header from content stream.
		if _, err := r.tr.Next(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) newSplitLimitReader() *chunk.SplitLimitReader {
	return r.cr.NewSplitLimitReader()
}

type copyTags struct {
	hdr     *index.Header
	content *chunk.Copy
	tags    []*index.Tag
}

func (r *Reader) readCopyTags(tagBound ...string) (*copyTags, error) {
	// Lazily setup reader for underlying file.
	if err := r.setupReader(); err != nil {
		return nil, err
	}
	// Determine the tags and number of bytes to copy.
	var idx int
	var numBytes int64
	for i, tag := range r.tags {
		if !index.BeforeBound(tag.Id, tagBound...) {
			break
		}
		idx = i + 1
		numBytes += tag.SizeBytes

	}
	// Setup copy struct.
	c := &copyTags{hdr: r.hdr}
	var err error
	c.content, err = r.cr.ReadCopy(numBytes)
	if err != nil {
		return nil, err
	}
	c.tags = r.tags[:idx]
	// Update reader state.
	r.tags = r.tags[idx:]
	if err := r.tr.Skip(numBytes); err != nil {
		return nil, err
	}
	return c, nil
}

// Close closes the reader.
func (r *Reader) Close() error {
	if err := r.cr.Close(); err != nil {
		return err
	}
	return r.ir.Close()
}
