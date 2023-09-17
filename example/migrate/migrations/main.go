package migrations

import "github.com/uptrace/bun/migrate"

var Migrations = migrate.NewMigrations()

// このディレクトリの中でマイグレーションファイルがある時は
// その情報をMigrationsにセットする
func init() {
	if err := Migrations.DiscoverCaller(); err != nil {
		panic(err)
	}
}
