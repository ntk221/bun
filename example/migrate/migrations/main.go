package migrations

import "github.com/uptrace/bun/migrate"

var Migrations = migrate.NewMigrations()

// このディレクトリの中でマイグレーションファイルがある時は
// その情報をMigrationsにセットする
//
// init関数は特別な関数で、このpackageを使用する時に実行される
// https://go.dev/doc/effective_go#init
func init() {
	if err := Migrations.DiscoverCaller(); err != nil {
		panic(err)
	}
}
