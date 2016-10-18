package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	//"time"
	"github.com/asiainfoLDP/datafoundry_coupon/log"
)

var logger = log.GetLogger()

/*
The fact of arbitrary chars in tag may be a problem for stat key.
This may make some tag stat keys are duplicated with some user stat key.
So it is best to avoid $#& in tag name.
*/

// todo: move following GetXxxKey functions into individual projects

func GetVersionKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "version")
}

func GetPhaseKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "phase")
}

func GetGeneralStatKey(words ...string) string {
	return strings.Join(words, "/")
}

func GetSubscriptionsStatKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "subs")
}

func GetSubscriptionPlanSigningTimesStatKey(words ...string) string { // params should be (repoName, itemName, planId string)
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "sgns")
}

func GetTransactionsStatKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "txns")
}

func GetStarsStatKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "strs")
}

func GetCommentsStatKey(words ...string) string {
	return fmt.Sprintf("%s%s%s", GetGeneralStatKey(words...), "#", "cmts")
}

// item doesn't mean data item. It means any objects.

func GetUserItemStatKey(username string, itemStatKey string) string {
	return fmt.Sprintf("%s$%s", username, itemStatKey)
}

func GetUserSubscriptionPlanSigningTimesStatKey(userName, repoName, itemName, planId string) string {
	return GetUserItemStatKey(userName, GetSubscriptionPlanSigningTimesStatKey(repoName, itemName, planId))
}

// user stats
func GetUserSubscriptionsStatKey(username string) string {
	return fmt.Sprintf("%s$#%s", username, "subs")
}

func GetUserTransactionsStatKey(username string) string {
	return fmt.Sprintf("%s$#%s", username, "txns")
}

func GetUserStarsStatKey(username string) string {
	return fmt.Sprintf("%s$#%s", username, "strs")
}

func GetUserCommentsStatKey(username string) string {
	return fmt.Sprintf("%s$#%s", username, "cmts")
}

// the following 2 will be removed
// it should be >$#
//func GetDateStatsStatKey(date time.Time) string {
//	return fmt.Sprintf("%s>%s", date.Format("2006-01-02"), "subs")
//}
//func GetDateTransactionsStatKey(date time.Time) string {
//	return fmt.Sprintf("%s>%s", date.Format("2006-01-02"), "txns")
//}

//==========================================================
//
//==========================================================

func ParseStatKey(statKey string) (date, user string, itemKeys []string, statName string) {
	index3 := strings.LastIndexByte(statKey, '#')
	if index3 < 0 {
		index3 = strings.LastIndexByte(statKey, '>')
		if index3 >= 0 {
			date = statKey[:index3]
			statName = statKey[index3+1:]
		}
	} else {
		statName = statKey[index3+1:]
		index1 := strings.IndexByte(statKey, '$')
		if index1 >= 0 && index1 < index3 {
			user = statKey[:index1]
			itemKeys = strings.Split(statKey[index1+1:index3], "/")
		} else {
			itemKeys = strings.Split(statKey[:index3], "/")
		}

		if len(itemKeys) == 1 && itemKeys[0] == "" {
			itemKeys = nil
		}
	}

	return
}

//==========================================================
//
//==========================================================

func UpdateStat(db *sql.DB, key string, delta int) (int, error) {
	return updateOrSetStat(db, key, delta, -1, true)
}

func SetStat(db *sql.DB, key string, newStat int) (int, error) {
	return updateOrSetStat(db, key, newStat, -1, false)
}

func SetStatIf(db *sql.DB, key string, newStat, ifOldStat int) (int, error) {
	return updateOrSetStat(db, key, newStat, ifOldStat, false)
}

var ErrOldStatNotMatch = errors.New("old stat not match")

// isUpdate == false means replace
// ifOldStat is only valid when it is >= 0s
// if old stat doesn't match ifOldStat, the old stat and error will be returned
func updateOrSetStat(db *sql.DB, key string, delta, ifOldStat int, isUpdate bool) (int, error) {
	sqlget := `select STAT_VALUE from DF_ITEM_STAT where STAT_KEY=?`

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	stat := 0
	err = tx.QueryRow(sqlget, key).Scan(&stat)
	if err != nil {
		if err != sql.ErrNoRows {
			tx.Rollback()
			return 0, err
		}

		// only for update if not exist
		if isUpdate {
			if ifOldStat >= 0 && ifOldStat != 0 {
				tx.Rollback()
				return stat, ErrOldStatNotMatch // fmt.Errorf("ifOldStat (%d) != 0", ifOldStat)
			}
		}

		stat = delta
		if stat < 0 {
			tx.Rollback()
			return 0, errors.New("stat delta can't be <= 0")
		}

		sqlinsert := `insert into DF_ITEM_STAT (STAT_KEY, STAT_VALUE) values (?, ?)`
		_, err := tx.Exec(sqlinsert, key, stat)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
	} else {
		if ifOldStat >= 0 && ifOldStat != stat {
			tx.Rollback()
			return stat, ErrOldStatNotMatch // fmt.Errorf("ifOldStat (%d) != stat (%d)", ifOldStat, stat)
		}

		if isUpdate {
			stat = stat + delta
		} else {
			stat = delta
		}

		// needed?
		//if stat < 0 {
		//	stat = 0
		//}

		sqlupdate := `update DF_ITEM_STAT set STAT_VALUE=? where STAT_KEY=?`
		_, err := tx.Exec(sqlupdate, stat, key)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	tx.Commit()

	return stat, nil
}

func RetrieveStat(db *sql.DB, key string) (int, error) {
	stat := 0
	sqlstr := `select STAT_VALUE from DF_ITEM_STAT where STAT_KEY=?`
	err := db.QueryRow(sqlstr, key).Scan(&stat)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return stat, nil
	}
}

// todo: maybe it is better to do this in a txn
func RemoveStat(db *sql.DB, key string) (int, error) {
	num, err := RetrieveStat(db, key)
	if err != nil {
		return 0, err
	}
	if num == 0 {
		return 0, nil
	}

	sqlstr := `delete from DF_ITEM_STAT where STAT_KEY=?`
	_, err = db.Exec(sqlstr, key)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return num, nil
	}
}

//=======================================================================
// cursor for outer package using
//=======================================================================

type StatCursor struct {
	rows *sql.Rows
}

func GetStatCursor(db *sql.DB) (*StatCursor, error) {
	rows, err := db.Query(`select STAT_KEY, STAT_VALUE from DF_ITEM_STAT`)
	if err != nil {
		return nil, err
	}

	return &StatCursor{rows: rows}, nil
}

func (cursor *StatCursor) Close() {
	if cursor.rows != nil {
		cursor.rows.Close()
		cursor.rows = nil
	}
}

func (cursor *StatCursor) Next() (string, int, error) {
	if cursor.rows != nil {
		if cursor.rows.Next() {
			key := ""
			value := 0
			if err := cursor.rows.Scan(&key, &value); err != nil {
				return "", 0, err
			}
			return key, value, nil
		}

		if err := cursor.rows.Err(); err != nil {
			return "", 0, err
		}

		cursor.Close()
	}

	return "", 0, nil
}
