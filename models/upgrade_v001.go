package models

import (
	"database/sql"
	//"fmt"
	//"errors"
	//stat "github.com/asiainfoLDP/datafoundry_appmarket/statistics"
	//"github.com/asiainfoLDP/datahub_commons/log"
)

type DatabaseUpgrader_0 struct {
	DatabaseUpgrader_Base
}

func newDatabaseUpgrader_0() *DatabaseUpgrader_0 {
	updater := &DatabaseUpgrader_0{}

	updater.currentTableCreationSqlFile = "initdb_v001.sql"

	updater.oldVersion = 0
	updater.newVersion = 1

	return updater
}

func (upgrader DatabaseUpgrader_0) Upgrade(db *sql.DB) error {
	return nil
}
