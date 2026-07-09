package file

import (
	"context"
	"strings"

	"github.com/fastschema/fastschema/db"
	"github.com/fastschema/fastschema/entity"
	"github.com/fastschema/fastschema/fs"
)

// isAbsoluteURL reports whether path is a full http(s) URL. Such a path points
// to an externally hosted file: it must be served verbatim, never prefixed with
// a disk base URL, and has no storage object to delete.
func isAbsoluteURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

type AppLike interface {
	DB() db.Client
	Disk(names ...string) fs.Disk
}

type FileService struct {
	DB   func() db.Client
	Disk func(names ...string) fs.Disk
}

func New(app AppLike) *FileService {
	return &FileService{
		DB:   app.DB,
		Disk: app.Disk,
	}
}

func (m *FileService) CreateResource(api *fs.Resource) {
	api.Group("file").
		Add(fs.NewResource("upload", m.Upload, &fs.Meta{Post: "/upload"})).
		Add(fs.NewResource("delete", m.Delete, &fs.Meta{Delete: "/"}))
}

func (m *FileService) FileListHook(
	ctx context.Context,
	query *db.QueryOption,
	entities []*entity.Entity,
) ([]*entity.Entity, error) {
	if query.Schema == nil {
		return entities, nil
	}

	if query.Schema.Name != "file" {
		return entities, nil
	}

	for _, entity := range entities {
		path := entity.GetString("path")

		if isAbsoluteURL(path) {
			entity.Set("url", path)
			continue
		}

		disk := m.Disk(entity.GetString("disk"))
		if path != "" && disk != nil {
			entity.Set("url", disk.URL(path))
		}
	}

	return entities, nil
}
