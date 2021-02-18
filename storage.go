package dropbox

import (
	"context"
	"io"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"

	"github.com/aos-dev/go-storage/v3/pkg/iowrap"
	. "github.com/aos-dev/go-storage/v3/types"
)

func (s *Storage) delete(ctx context.Context, path string, opt pairStorageDelete) (err error) {
	rp := s.getAbsPath(path)

	input := &files.DeleteArg{
		Path: rp,
	}

	_, err = s.client.DeleteV2(input)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) list(ctx context.Context, path string, opt pairStorageList) (oi *ObjectIterator, err error) {
	input := &objectPageStatus{
		limit: 200,
		path:  s.getAbsPath(path),
	}

	if opt.ListMode.IsPrefix() {
		input.recursive = true
	}

	return NewObjectIterator(ctx, s.nextObjectPage, input), nil
}

func (s *Storage) metadata(ctx context.Context, opt pairStorageMetadata) (meta *StorageMeta, err error) {
	meta = NewStorageMeta()
	meta.WorkDir = s.workDir
	meta.Name = ""

	return
}

func (s *Storage) nextObjectPage(ctx context.Context, page *ObjectPage) error {
	input := page.Status.(*objectPageStatus)

	var err error
	var output *files.ListFolderResult

	if input.cursor == "" {
		output, err = s.client.ListFolder(&files.ListFolderArg{
			Path: input.path,
		})
	} else {
		output, err = s.client.ListFolderContinue(&files.ListFolderContinueArg{
			Cursor: input.cursor,
		})
	}
	if err != nil {
		return err
	}

	for _, v := range output.Entries {
		var o *Object
		switch meta := v.(type) {
		case *files.FolderMetadata:
			o = s.formatFolderObject(meta)
		case *files.FileMetadata:
			o = s.formatFileObject(meta)
		}

		page.Data = append(page.Data, o)
	}

	if !output.HasMore {
		return IterateDone
	}

	input.cursor = output.Cursor
	return nil
}

func (s *Storage) read(ctx context.Context, path string, w io.Writer, opt pairStorageRead) (n int64, err error) {
	rp := s.getAbsPath(path)

	input := &files.DownloadArg{
		Path: rp,
	}

	_, rc, err := s.client.Download(input)
	if err != nil {
		return 0, err
	}

	if opt.HasSize {
		rc = iowrap.LimitReadCloser(rc, opt.Size)
	}

	if opt.HasIoCallback {
		rc = iowrap.CallbackReadCloser(rc, opt.IoCallback)
	}

	return io.Copy(w, rc)
}

func (s *Storage) stat(ctx context.Context, path string, opt pairStorageStat) (o *Object, err error) {
	rp := s.getAbsPath(path)

	input := &files.GetMetadataArg{
		Path: rp,
	}

	output, err := s.client.GetMetadata(input)
	if err != nil {
		return nil, err
	}

	switch meta := output.(type) {
	case *files.FolderMetadata:
		o = s.formatFolderObject(meta)
	case *files.FileMetadata:
		o = s.formatFileObject(meta)
	}

	return o, nil
}

func (s *Storage) write(ctx context.Context, path string, r io.Reader, size int64, opt pairStorageWrite) (n int64, err error) {
	rp := s.getAbsPath(path)

	r = io.LimitReader(r, size)

	if opt.HasIoCallback {
		r = iowrap.CallbackReader(r, opt.IoCallback)
	}

	input := &files.CommitInfo{
		Path: rp,
		Mode: &files.WriteMode{
			Tagged: dropbox.Tagged{
				Tag: files.WriteModeAdd,
			},
		},
	}

	_, err = s.client.Upload(input, r)
	if err != nil {
		return 0, err
	}

	return size, nil
}
