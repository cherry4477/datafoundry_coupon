package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Coupon struct {
	Id       int
	Serial   string    `json:"serial"`
	Code     string    `json:"code"`
	Kind     string    `json:"kind,omitempty"`
	ExpireOn time.Time `json:"expire_on,omitempty"`
	Amount   float32   `json:"amount,omitempty"`
}

type createResult struct {
	Serial   string  `json:"serial"`
	Code     string  `json:"code"`
	ExpireOn string  `json:"expire_on"`
	Amount   float32 `json:"amount"`
}

func CreateCoupon(db *sql.DB, couponInfo *Coupon) (*createResult, error) {
	logger.Info("Begin create a Coupon model.")

	sqlstr := fmt.Sprintf(`insert into DF_COUPON (
				SERIAL, CODE, KIND, EXPIRE_ON, AMOUNT, STATUS
				) values (?, ?, ?, ?, ?, ?)`,
	)

	couponInfo.Serial = strings.ToLower(couponInfo.Serial)
	couponInfo.Code = strings.ToLower(couponInfo.Code)
	_, err := db.Exec(sqlstr,
		couponInfo.Serial, couponInfo.Code, couponInfo.Kind, couponInfo.ExpireOn.Format("2006-01-02"),
		couponInfo.Amount, "available",
	)
	if err != nil {
		logger.Error("Exec err : %v", err)
		return nil, err
	}

	result := &createResult{Serial: strings.ToUpper(couponInfo.Serial),
		Code:     strings.ToUpper(couponInfo.Code),
		ExpireOn: couponInfo.ExpireOn.Format("2006-01-02"),
		Amount:   couponInfo.Amount,
	}

	logger.Info("End create a plan model.")
	return result, err
}

func DeleteCoupon(db *sql.DB, couponId string) error {
	logger.Info("Begin delete a plan model.")

	err := modifyCouponStatusToN(db, couponId)
	if err != nil {
		return err
	}

	logger.Info("End delete a plan model.")
	return err
}

type retrieveResult struct {
	Serial   string    `json:"serial"`
	ExpireOn time.Time `json:"expire_on"`
	Amount   float32   `json:"amount"`
	Status   string    `json:"status"`
}

func RetrieveCouponByID(db *sql.DB, couponId string) (*retrieveResult, error) {
	logger.Info("Begin get a coupon by id model.")

	couponId = strings.ToLower(couponId)

	logger.Info("End get a coupon by id model.")
	return getSingleCoupon(db, fmt.Sprintf("CODE = '%s'", couponId))
}

func getSingleCoupon(db *sql.DB, sqlWhere string) (*retrieveResult, error) {
	coupons, err := queryCoupons(db, sqlWhere, "", 1, 0)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	}

	if len(coupons) == 0 {
		logger.Error("No this coupon.")
		return nil, nil
	}

	err = updateCouponStatusToQ(db, coupons[0])
	if err != nil {
		return nil, err
	}

	return coupons[0], nil
}

func updateCouponStatusToQ(db *sql.DB, result *retrieveResult) error {
	sqlstr := fmt.Sprintf(`update DF_COUPON set status = "queried" where SERIAL = '%s' and STATUS = 'available'`, result.Serial)

	_, err := db.Exec(sqlstr)
	if err != nil {
		logger.Error("Exec err : %v", err)
		return err
	}

	return err
}

func ProvideCoupon(db *sql.DB, numberStr, amountStr string) (int64, []string, error) {
	number, err := ValidateNumber(numberStr, 1)
	if err != nil {
		logger.Error("Catch err: %v.", err)
		return 0, nil, err
	}

	var sqlWhere string

	if amountStr == "" {
		sqlWhere = "STATUS='available'"
	} else {
		amount, err := ValidateAmount(amountStr)
		if err != nil {
			logger.Error("Catch err: %v.", err)
			return 0, nil, err
		}
		sqlWhere = fmt.Sprintf("STATUS='available' AND AMOUNT=%d", amount)
	}

	codes, err := provideCodes(db, sqlWhere, number)
	//coupons, err := queryCoupons(db, sqlWhere, "", number, 0)
	if err != nil {
		logger.Error("Catch err: %v.", err)
		return 0, nil, err
	}

	err = updateCouponsStatusToP(db, codes)
	if err != nil {
		return 0, nil, err
	}

	return int64(len(codes)), codes, nil
}

func provideCodes(db *sql.DB, sqlWhere string, number int) ([]string, error) {
	sql := fmt.Sprintf("select CODE from DF_COUPON "+
		"WHERE %s limit %d offset %d",
		sqlWhere, number, 0)
	rows, err := db.Query(sql)
	if err != nil {
		logger.Error("Query err:", err)
		return nil, err
	}
	defer rows.Close()

	logger.Info(">>>>>>>%s", sql)

	var codes []string
	for rows.Next() {
		var code string
		rows.Scan(&code)
		codes = append(codes, code)
	}

	return codes, nil
}

func ValidateNumber(numberStr string, defaultNumber int) (int, error) {
	switch numberStr {
	case "":
		return defaultNumber, nil
	}

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		logger.Error("strconv.Atoi err: %v.", err)
		return defaultNumber, err
	}

	return number, nil
}

func ValidateAmount(amountStr string) (int, error) {
	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		logger.Error("strconv.Atoi err: %v.", err)
		return 0, err
	}

	return amount, err
}

func updateCouponsStatusToP(db *sql.DB, codes []string) error {
	var sql string
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, code := range codes {
		sql = fmt.Sprintf(`update DF_COUPON set status = "provided" where CODE = '%s' and STATUS = 'available'`, code)

		_, err = tx.Exec(sql)
		if err != nil {
			logger.Error("Exec err: %v", err)
			err = tx.Rollback()
			if err != nil {
				logger.Error("db rollback err: %v", err)
				return err
			}
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		logger.Error("db commit err: %v", err)
		return err
	}

	return nil
}

func queryCoupons(db *sql.DB, sqlWhere, orderBy string, limit int, offset int64, sqlParams ...interface{}) ([]*retrieveResult, error) {
	offset_str := ""
	if offset > 0 {
		offset_str = fmt.Sprintf("offset %d", offset)
	}

	logger.Debug("sqlWhere=%v", sqlWhere)
	sqlWhereAll := ""
	if sqlWhere != "" {
		sqlWhereAll = fmt.Sprintf("WHERE %s", sqlWhere)
	} else {
		sqlWhereAll = fmt.Sprintf(" %s", sqlWhere)
	}

	sql_str := fmt.Sprintf(`select
					SERIAL, EXPIRE_ON, AMOUNT, STATUS
					from DF_COUPON
					%s %s
					limit %d
					%s
					`,
		sqlWhereAll,
		orderBy,
		limit,
		offset_str)
	rows, err := db.Query(sql_str, sqlParams...)

	logger.Info(">>> %v", sql_str)

	if err != nil {
		logger.Error("Query err : %v", err)
		return nil, err
	}
	defer rows.Close()

	coupons := make([]*retrieveResult, 0, 100)
	for rows.Next() {
		coupon := &retrieveResult{}
		err := rows.Scan(
			&coupon.Serial, &coupon.ExpireOn, &coupon.Amount, &coupon.Status,
		)
		if err != nil {
			logger.Error("Scan err : %v", err)
			return nil, err
		}
		//validateApp(s) // already done in scanAppWithRows
		coupons = append(coupons, coupon)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Err : ", err)
		return nil, err
	}

	logger.Info("End get coupon list model.")
	return coupons, nil
}

func modifyCouponStatusToN(db *sql.DB, couponId string) error {
	sqlstr := fmt.Sprintf(`update DF_COUPON set status = "unavailable" where SERIAL = '%s' and STATUS = 'available'`, couponId)

	_, err := db.Exec(sqlstr)
	if err != nil {
		logger.Error("Exec err : %v", err)
		return err
	}

	return err
}

func QueryCoupons(db *sql.DB, kind, orderBy string, sortOrder bool, offset int64, limit int) (int64, []*retrieveResult, error) {
	logger.Info("Begin get coupon list model.")

	sqlParams := make([]interface{}, 0, 4)

	// ...

	sqlWhere := ""
	kind = strings.ToLower(kind)
	if kind != "" {
		if sqlWhere == "" {
			sqlWhere = "KIND = ?"
		} else {
			sqlWhere = sqlWhere + " and KIND = ?"
		}
		sqlParams = append(sqlParams, kind)
	}

	// ...

	switch strings.ToLower(orderBy) {
	default:
		orderBy = "EXPIRE_ON"
		sortOrder = false
	case "createtime":
		orderBy = "CREATE_TIME"
	}

	sqlSort := fmt.Sprintf("%s %s", orderBy, sortOrderText[sortOrder])

	// ...

	logger.Debug("sqlWhere=%v", sqlWhere)
	return getCouponList(db, offset, limit, sqlWhere, sqlSort, sqlParams...)
}

const (
	SortOrder_Asc  = "asc"
	SortOrder_Desc = "desc"
)

// true: asc
// false: desc
var sortOrderText = map[bool]string{true: "asc", false: "desc"}

func ValidateSortOrder(sortOrder string, defaultOrder bool) bool {
	switch strings.ToLower(sortOrder) {
	case SortOrder_Asc:
		return true
	case SortOrder_Desc:
		return false
	}

	return defaultOrder
}

func ValidateOrderBy(orderBy string) string {
	switch orderBy {
	case "createtime":
		return "CREATE_TIME"
	}

	return ""
}

func getCouponList(db *sql.DB, offset int64, limit int, sqlWhere string, sqlSort string, sqlParams ...interface{}) (int64, []*retrieveResult, error) {
	//if strings.TrimSpace(sqlWhere) == "" {
	//	return 0, nil, errors.New("sqlWhere can't be blank")
	//}

	count, err := queryCouponsCount(db, sqlWhere, sqlParams...)
	logger.Debug("count: %v", count)
	if err != nil {
		return 0, nil, err
	}
	if count == 0 {
		return 0, []*retrieveResult{}, nil
	}
	validateOffsetAndLimit(count, &offset, &limit)

	logger.Debug("sqlWhere=%v", sqlWhere)
	subs, err := queryCoupons(db, sqlWhere,
		fmt.Sprintf(`order by %s`, sqlSort),
		limit, offset, sqlParams...)

	return count, subs, err
}

func queryCouponsCount(db *sql.DB, sqlWhere string, sqlParams ...interface{}) (int64, error) {
	sqlWhere = strings.TrimSpace(sqlWhere)
	sql_where_all := ""
	if sqlWhere != "" {
		sql_where_all = fmt.Sprintf("where %s", sqlWhere)
	}

	count := int64(0)
	sql_str := fmt.Sprintf(`select COUNT(*) from DF_COUPON %s`, sql_where_all)
	logger.Debug(">>>\n"+
		"	%s", sql_str)
	logger.Debug("sqlParams: %v", sqlParams)
	err := db.QueryRow(sql_str, sqlParams...).Scan(&count)
	if err != nil {
		logger.Error("Scan err : %v", err)
		return 0, err
	}

	return count, err
}

func validateOffsetAndLimit(count int64, offset *int64, limit *int) {
	if *limit < 1 {
		*limit = 1
	}
	if *offset >= count {
		*offset = count - int64(*limit)
	}
	if *offset < 0 {
		*offset = 0
	}
	if *offset+int64(*limit) > count {
		*limit = int(count - *offset)
	}
}

type UseInfo struct {
	Serial    string    `json:"serial"`
	Code      string    `json:"code"`
	Username  string    `json:"username"`
	Namespace string    `json:"namespace"`
	Use_time  time.Time `json:"recharge_time"`
}

type useResult struct {
	Amount    float32 `json:"amount"`
	Namespace string  `json:"namespace"`
}

func UseCoupon(db *sql.DB, useInfo *UseInfo, callback func() error) (*useResult, error) {
	logger.Info("Begin use a coupon model.")

	useInfo.Serial = strings.ToLower(useInfo.Serial)
	useInfo.Code = strings.ToLower(useInfo.Code)

	tx, err := db.Begin()
	if err != nil {
		logger.Error("Begin a trasaction err: %v", err)
		return nil, err
	}
	return func() (*useResult, error) {
		type db struct{}
		sql := "SELECT AMOUNT, EXPIRE_ON, STATUS FROM DF_COUPON WHERE SERIAL=? AND CODE=?"
		row := tx.QueryRow(sql, useInfo.Serial, useInfo.Code)
		logger.Info(">>>\n%v\n%v, %v", sql, useInfo.Serial, useInfo.Code)

		var amount float32
		var expireOn time.Time
		var status string
		err = row.Scan(&amount, &expireOn, &status)
		if err != nil {
			tx.Rollback()
			logger.Error("Scan err : %v", err)
			return nil, err
		}
		logger.Info("expireOn=%v, amount=%v, status=%v", expireOn, amount, status)

		if status == "expired" {
			return nil, errors.New("The coupon has expired.")
		} else if status == "used" {
			return nil, errors.New("The coupon has used.")
		} else if status == "unavailable" {
			return nil, errors.New("The coupon unavailable.")
		}

		useInfo.Use_time = useInfo.Use_time.UTC().Add(time.Hour * 8)
		logger.Info("use time: %v", useInfo.Use_time)

		duration := expireOn.Sub(useInfo.Use_time)
		logger.Info("duration: %v", duration)

		if duration < 0 {
			sql = "UPDATE DF_COUPON SET STATUS='expired' WHERE SERIAL=? AND CODE=?"
			_, err := tx.Exec(sql, useInfo.Serial, useInfo.Code)
			if err != nil {
				tx.Rollback()
				logger.Error("Exec err : %v", err)
				return nil, err
			}
			logger.Info(">>>\n%v\n%v, %v", sql, useInfo.Serial, useInfo.Code)
			return nil, errors.New("The coupon has expired.")
		}

		sql = "UPDATE DF_COUPON SET USE_TIME=?, USERNAME=?, NAMESPACE=?, STATUS=? WHERE SERIAL=? AND CODE=?"
		_, err = tx.Exec(sql, useInfo.Use_time, useInfo.Username, useInfo.Namespace, "used", useInfo.Serial, useInfo.Code)
		if err != nil {
			tx.Rollback()
			logger.Error("Exec err : %v", err)
			return nil, err
		}
		logger.Info(">>>\n%v\n%v, %v, %v, %v, %v", sql,
			useInfo.Use_time, useInfo.Username, useInfo.Namespace, useInfo.Serial, useInfo.Code)

		err = callback()
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		tx.Commit()
		useResult := &useResult{Amount: amount, Namespace: useInfo.Namespace}

		logger.Info("End use a coupon model.")

		return useResult, nil
	}()
}

type FromUser struct {
	OpenId       string `json:"openId"`
	Provide_time int64  `json:"provideTime"`
}

func JudgeIsProvide(db *sql.DB, info *FromUser, timeStr string) (error, bool) {
	sql := "select count(*) from DF_COUPON_PROVIDE where TO_USER = ?"

	var count int
	err := db.QueryRow(sql, info.OpenId).Scan(&count)
	if err != nil {
		logger.Error("QueryRow err: %v.", err)
		return err, false
	}
	logger.Debug("count: %v.", count)

	if count == 0 {
		sql := "insert into DF_COUPON_PROVIDE (TO_USER, PROVIDE_TIME) values (?, ?)"
		_, err := db.Exec(sql, info.OpenId, timeStr)
		if err != nil {
			logger.Error("Exec err: %v.", err)
			return err, false
		}
		return nil, true
	} else {
		return nil, false
	}
}
