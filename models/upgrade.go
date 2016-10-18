package models

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	stat "github.com/asiainfoLDP/datafoundry_coupon/statistics"
	"io/ioutil"
	"path/filepath"
	"time"
)

//=============================================================
//
//=============================================================

var dbUpgraders = []DatabaseUpgrader{
	newDatabaseUpgrader_0(),
	//newDatabaseUpgrader_1(),
	//newDatabaseUpgrader_2(),
}

const (
	DbPhase_Unkown    = -1
	DbPhase_Serving   = 0 // must be 0
	DbPhase_Upgrading = 1
)

var dbPhase = DbPhase_Unkown

func IsServing() bool {
	return dbPhase == DbPhase_Serving
}

// for ut, reallyNeedUpgrade is false
func TryToUpgradeDatabase(db *sql.DB, dbName string, reallyNeedUpgrade bool) error {

	if reallyNeedUpgrade {

		if len(dbUpgraders) == 0 {
			return errors.New("at least one db upgrader needed")
		}
		lastDbUpgrader := dbUpgraders[len(dbUpgraders)-1]

		// create tables. (no table created in following _upgradeDatabase callings)

		err := lastDbUpgrader.TryToCreateTables(db)
		if err != nil {
			return err
		}

		// init version value stat as LatestDataVersion if it doesn't exist,
		// which means tables are just created. In this case, no upgrations are needed.

		// INSERT INTO DH_ITEM_STAT (STAT_KEY, STAT_VALUE)
		// VALUES(dbName#version, LatestDataVersion)
		// ON DUPLICATE KEY UPDATE STAT_VALUE=LatestDataVersion;
		dbVersionKey := stat.GetVersionKey(dbName)
		_, err = stat.SetStatIf(db, dbVersionKey, lastDbUpgrader.NewVersion(), 0)
		if err != nil && err != stat.ErrOldStatNotMatch {
			return err
		}

		current_version, err := stat.RetrieveStat(db, dbVersionKey)
		if err != nil {
			return err
		}

		// upgrade

		if current_version != lastDbUpgrader.NewVersion() {

			logger.Info("mysql start upgrading ...")

			dbPhase = DbPhase_Unkown

			for _, dbupgrader := range dbUpgraders {
				if err = _upgradeDatabase(db, dbName, dbupgrader); err != nil {
					return err
				}
			}
		}
	}

	dbPhase = DbPhase_Serving

	logger.Info("mysql start serving ...")

	return nil
}

func _upgradeDatabase(db *sql.DB, dbName string, upgrader DatabaseUpgrader) error {
	dbVersionKey := stat.GetVersionKey(dbName)
	current_version, err := stat.RetrieveStat(db, dbVersionKey)
	if err != nil {
		return err
	}
	if current_version == 0 {
		current_version = 1
	}

	logger.Info("TryToUpgradeDatabase current version: %d", current_version)

	if upgrader.NewVersion() <= current_version {
		return nil
	}
	if upgrader.OldVersion() != current_version {
		return fmt.Errorf("old version (%d) <= current version (%d)", upgrader.OldVersion(), current_version)
	}

	dbPhaseKey := stat.GetPhaseKey(dbName)
	phase, err := stat.SetStatIf(db, dbPhaseKey, DbPhase_Upgrading, DbPhase_Serving)

	logger.Info("TryToUpgradeDatabase current phase: %d", phase)

	if err != nil {
		return err
	}

	// ...

	dbPhase = DbPhase_Upgrading

	err = upgrader.Upgrade(db)
	if err != nil {
		return err
	}

	// ...

	_, err = stat.SetStat(db, dbVersionKey, upgrader.NewVersion())
	if err != nil {
		return err
	}

	logger.Info("TryToUpgradeDatabase new version: %d", upgrader.NewVersion())

	time.Sleep(30 * time.Millisecond)

	_, err = stat.SetStatIf(db, dbPhaseKey, DbPhase_Serving, DbPhase_Upgrading)
	if err != nil {
		return err
	}

	return nil
}

type DatabaseUpgrader interface {
	OldVersion() int
	NewVersion() int
	Upgrade(db *sql.DB) error
	TryToCreateTables(db *sql.DB) error
}

type DatabaseUpgrader_Base struct {
	oldVersion int
	newVersion int

	currentTableCreationSqlFile string
}

func (upgrader DatabaseUpgrader_Base) OldVersion() int {
	return upgrader.oldVersion
}

func (upgrader DatabaseUpgrader_Base) NewVersion() int {
	return upgrader.newVersion
}

func (upgrader DatabaseUpgrader_Base) TryToCreateTables(db *sql.DB) error {

	if upgrader.currentTableCreationSqlFile == "" {
		return nil
	}

	data, err := ioutil.ReadFile(filepath.Join("_db", upgrader.currentTableCreationSqlFile))
	if err != nil {
		return err
	}

	sqls := bytes.SplitAfter(data, []byte("DEFAULT CHARSET=UTF8;"))
	sqls = sqls[:len(sqls)-1]
	for _, sql := range sqls {
		_, err = db.Exec(string(sql))
		if err != nil {
			return err
		}
	}

	return nil
}
