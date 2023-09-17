package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type MigrationsOption func(m *Migrations)

func WithMigrationsDirectory(directory string) MigrationsOption {
	return func(m *Migrations) {
		m.explicitDirectory = directory
	}
}

type Migrations struct {
	ms MigrationSlice

	explicitDirectory string
	implicitDirectory string
}

func NewMigrations(opts ...MigrationsOption) *Migrations {
	m := new(Migrations)
	for _, opt := range opts {
		opt(m)
	}
	m.implicitDirectory = filepath.Dir(migrationFile())
	return m
}

func (m *Migrations) Sorted() MigrationSlice {
	migrations := make(MigrationSlice, len(m.ms))
	copy(migrations, m.ms)
	sortAsc(migrations)
	return migrations
}

func (m *Migrations) MustRegister(up, down MigrationFunc) {
	if err := m.Register(up, down); err != nil {
		panic(err)
	}
}

func (m *Migrations) Register(up, down MigrationFunc) error {
	fpath := migrationFile()
	name, comment, err := extractMigrationName(fpath)
	if err != nil {
		return err
	}

	m.Add(Migration{
		Name:    name,
		Comment: comment,
		Up:      up,
		Down:    down,
	})

	return nil
}

func (m *Migrations) Add(migration Migration) {
	if migration.Name == "" {
		panic("migration name is required")
	}
	m.ms = append(m.ms, migration)
}

// DiscoverCaller は現在の呼び出し元をトレースし、そのファイルが存在するディレクトリを特定します。
// その後、そのディレクトリをルートとしてマイグレーションファイルを探索します。
// マイグレーションの発見に成功した場合、マイグレーションリストに追加します。
func (m *Migrations) DiscoverCaller() error {
    // migrationFile 関数を使って現在の呼び出し元のファイルパスを取得します。
    dir := filepath.Dir(migrationFile())

    // 取得したディレクトリをルートとしてマイグレーションファイルを探索します。
    return m.Discover(os.DirFS(dir))
}

// Discover 関数は指定されたファイルシステム `fsys` 内のマイグレーションファイルを探索し、
// 見つかったマイグレーションに関する情報を更新します。
func (m *Migrations) Discover(fsys fs.FS) error {
    return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }

        if !strings.HasSuffix(path, ".up.sql") && !strings.HasSuffix(path, ".down.sql") {
            return nil
        }

        name, comment, err := extractMigrationName(path)
        if err != nil {
            return err
        }

        // マイグレーションがすでに存在する場合は取得し、存在しない場合は新しく作成します。
        migration := m.getOrCreateMigration(name)
        if err != nil {
            return err
        }

        migration.Comment = comment
        migrationFunc := NewSQLMigrationFunc(fsys, path)

        if strings.HasSuffix(path, ".up.sql") {
            migration.Up = migrationFunc
            return nil
        }
        if strings.HasSuffix(path, ".down.sql") {
            migration.Down = migrationFunc
            return nil
        }

        return errors.New("migrate: not reached")
    })
}

func (m *Migrations) getOrCreateMigration(name string) *Migration {
	for i := range m.ms {
		m := &m.ms[i]
		if m.Name == name {
			return m
		}
	}

	m.ms = append(m.ms, Migration{Name: name})
	return &m.ms[len(m.ms)-1]
}

func (m *Migrations) getDirectory() string {
	if m.explicitDirectory != "" {
		return m.explicitDirectory
	}
	if m.implicitDirectory != "" {
		return m.implicitDirectory
	}
	return filepath.Dir(migrationFile())
}

// migrationFile は現在の呼び出し元をトレースし、
// 最初に "/bun/migrate." を含まない関数のファイル名を返します。
// この関数は通常、マイグレーションファイルのパスを特定するために使用されます。
// もし呼び出し元が "/bun/migrate." を含む関数であれば、
// 呼び出し元のファイルパスを返します。
func migrationFile() string {
    const depth = 32
    var pcs [depth]uintptr
    n := runtime.Callers(1, pcs[:])
    frames := runtime.CallersFrames(pcs[:n])

    for {
        f, ok := frames.Next()
        if !ok {
            break
        }
        if !strings.Contains(f.Function, "/bun/migrate.") {
            return f.File
        }
    }

    return ""
}

func migrationFile() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	for {
		f, ok := frames.Next()
		if !ok {
			break
		}
		if !strings.Contains(f.Function, "/bun/migrate.") {
			return f.File
		}
	}

	return ""
}

var fnameRE = regexp.MustCompile(`^(\d{1,14})_([0-9a-z_\-]+)\.`)

func extractMigrationName(fpath string) (string, string, error) {
	fname := filepath.Base(fpath)

	matches := fnameRE.FindStringSubmatch(fname)
	if matches == nil {
		return "", "", fmt.Errorf("migrate: unsupported migration name format: %q", fname)
	}

	return matches[1], matches[2], nil
}
