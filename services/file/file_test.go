package file_test

import (
	"context"
	"os"
	"testing"

	"github.com/fastschema/fastschema/db"
	"github.com/fastschema/fastschema/entity"
	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/logger"
	"github.com/fastschema/fastschema/pkg/entdbadapter"
	"github.com/fastschema/fastschema/pkg/rclonefs"
	rr "github.com/fastschema/fastschema/pkg/restfulresolver"
	"github.com/fastschema/fastschema/pkg/utils"
	"github.com/fastschema/fastschema/schema"
	ms "github.com/fastschema/fastschema/services/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type testApp struct {
	sb    *schema.Builder
	db    db.Client
	disks []fs.Disk
}

func (s testApp) DB() db.Client {
	return s.db
}

func (s testApp) Disk(names ...string) fs.Disk {
	return s.disks[0]
}

func createFileService(t *testing.T) (*ms.FileService, *rr.Server) {
	sb := utils.Must(schema.NewBuilderFromDir(t.TempDir(), fs.SystemSchemaTypes...))
	disks := utils.Must(rclonefs.NewFromConfig([]*fs.DiskConfig{{
		Name:    "local_test",
		Driver:  "local",
		Root:    t.TempDir(),
		BaseURL: "http://localhost:3000/files",
	}}, t.TempDir()))

	testApp := &testApp{sb: sb, disks: disks}
	fileService := ms.New(testApp)
	testApp.db = utils.Must(entdbadapter.NewTestClient(utils.Must(os.MkdirTemp("", "migrations")), sb, func() *db.Hooks {
		return &db.Hooks{
			PostDBQuery: []db.PostDBQuery{fileService.FileListHook},
		}
	}))
	resources := fs.NewResourcesManager()
	resources.Middlewares = append(resources.Middlewares, func(c fs.Context) error {
		c.Local("user", &fs.User{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")})
		return c.Next()
	})
	resources.Group("file").
		Add(fs.NewResource("upload", fileService.Upload, &fs.Meta{
			Post: "/upload",
		})).
		Add(fs.NewResource("delete", fileService.Delete, &fs.Meta{
			Delete: "/",
		}))
	assert.NoError(t, resources.Init())
	restResolver := rr.NewRestfulResolver(&rr.ResolverConfig{
		ResourceManager: resources,
		Logger:          logger.CreateMockLogger(true),
	})

	return fileService, restResolver.Server()
}

func TestNewFileService(t *testing.T) {
	service, server := createFileService(t)
	assert.NotNil(t, service)
	assert.NotNil(t, server)
}

func TestCreateResource(t *testing.T) {
	service, _ := createFileService(t)
	api := fs.NewResourcesManager().Group("api")
	service.CreateResource(api)
	assert.NotNil(t, api.Find("api.file.upload"))
	assert.NotNil(t, api.Find("api.file.delete"))
}

// A file record whose path is an absolute URL points to an externally hosted
// file: its url must be served verbatim, never prefixed with a disk base URL.
func TestFileListHookAbsoluteURL(t *testing.T) {
	service, _ := createFileService(t)
	query := &db.QueryOption{Schema: &schema.Schema{Name: "file"}}

	external := entity.New().Set("path", "https://cdn.example.com/img.png").Set("disk", "")
	relative := entity.New().Set("path", "some/path/test.txt").Set("disk", "local_test")

	out, err := service.FileListHook(context.Background(), query, []*entity.Entity{external, relative})
	assert.NoError(t, err)

	assert.Equal(t, "https://cdn.example.com/img.png", out[0].GetString("url"))
	assert.Contains(t, out[1].GetString("url"), "http://localhost:3000/files")
	assert.Contains(t, out[1].GetString("url"), "some/path/test.txt")
}
