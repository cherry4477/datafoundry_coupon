package api

import (
	"encoding/json"
	"github.com/asiainfoLDP/datafoundry_coupon/common"
	"github.com/asiainfoLDP/datafoundry_coupon/log"
	"github.com/asiainfoLDP/datafoundry_coupon/models"
	"github.com/julienschmidt/httprouter"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	letterBytes = "abcdefghjklmnpqrstuvwxyz0123456789"
	randNumber  = "0123456789"
)

var logger = log.GetLogger()

var AdminUsers = make([]string, 0)

func init() {
	initAdminUser()
}

type createInfo struct {
	Kind     string  `json:"kind,omitempty"`
	ExpireOn int     `json:"expire_on,omitempty"`
	Amount   float32 `json:"amount,omitempty"`
}

func CreateCoupon(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: POST %v.", r.URL)
	logger.Info("Begin create coupon handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()
	region := r.Form.Get("region")
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !checkAdminUsers(username) {
		JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
		return
	}

	createInfo := &createInfo{}
	err := common.ParseRequestJsonInto(r, createInfo)
	if err != nil {
		logger.Error("Parse body err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeParseJsonFailed, err.Error()), nil)
		return
	}

	expireDate := time.Now().Add(time.Hour * 24 * time.Duration(createInfo.ExpireOn)).UTC()

	coupon := &models.Coupon{}
	coupon.ExpireOn = expireDate
	coupon.Kind = createInfo.Kind
	coupon.Amount = createInfo.Amount
	coupon.Serial = "df" + genSerial() + "r"
	coupon.Code = genCode()

	logger.Debug("coupon: %v", coupon)

	//create coupon in database
	result, err := models.CreateCoupon(db, coupon)
	if err != nil {
		logger.Error("Create plan err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCreateCoupon, err.Error()), nil)
		return
	}

	logger.Info("End create coupon handler.")
	JsonResult(w, http.StatusOK, nil, result)
}

func DeleteCoupon(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: DELETE %v.", r.URL)
	logger.Info("Begin delete coupon handler.")

	r.ParseForm()
	region := r.Form.Get("region")
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !checkAdminUsers(username) {
		JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
		return
	}

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	couponId := params.ByName("id")
	logger.Debug("Coupon id: %s.", couponId)

	// /delete in database
	err := models.DeleteCoupon(db, couponId)
	if err != nil {
		logger.Error("Delete coupon err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeDeleteCoupon, err.Error()), nil)
		return
	}

	logger.Info("End delete coupon handler.")
	JsonResult(w, http.StatusOK, nil, nil)
}

func RetrieveCoupon(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: GET %v.", r.URL)
	logger.Info("Begin retrieve coupon handler.")

	r.ParseForm()
	region := r.Form.Get("region")
	logger.Info("region: %s", region)
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	couponId := params.ByName("code")
	coupon, err := models.RetrieveCouponByID(db, couponId)
	if err != nil {
		logger.Error("Get coupon err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeGetCouponById, err.Error()), nil)
		return
	} else if coupon == nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeGetCouponNotExsit, "This coupon does not exist."), nil)
		return
	}

	if coupon.Status == "used" {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCouponHasUsed, "This coupon does has used."), nil)
		return
	} else if coupon.Status == "expired" {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCouponHasExpired, "This coupon does has expired."), nil)
		return
	} else if coupon.Status == "unavailable" {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCouponUnavailable, "This coupon is not available."), nil)
		return
	}

	logger.Info("End retrieve coupon handler.")
	JsonResult(w, http.StatusOK, nil, coupon)
}

func QueryCouponList(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: GET %v.", r.URL)
	logger.Info("Begin retrieve coupon list handler.")

	r.ParseForm()
	region := r.Form.Get("region")
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !checkAdminUsers(username) {
		JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
		return
	}

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()

	kind := r.Form.Get("kind")

	offset, size := OptionalOffsetAndSize(r, 30, 1, 100)
	orderBy := models.ValidateOrderBy(r.Form.Get("orderby"))
	sortOrder := models.ValidateSortOrder(r.Form.Get("sortorder"), false)

	count, coupons, err := models.QueryCoupons(db, kind, orderBy, sortOrder, offset, size)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeQueryCoupons, err.Error()), nil)
		return
	}

	logger.Info("End retrieve coupon list handler.")
	JsonResult(w, http.StatusOK, nil, NewQueryListResult(count, coupons))
}

func UseCoupon(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: PUT %v.", r.URL)
	logger.Info("Begin use a coupon handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()
	region := r.Form.Get("region")
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	serial := params.ByName("serial")

	useInfo := &models.UseInfo{}
	err := common.ParseRequestJsonInto(r, useInfo)
	if err != nil {
		logger.Error("Parse body err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeParseJsonFailed, err.Error()), nil)
		return
	}
	useInfo.Serial = serial
	useInfo.Username = username
	useInfo.Use_time = time.Now()

	getResult, err := models.RetrieveCouponByID(db, useInfo.Code)
	if err != nil {
		logger.Error("db get coupon err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeGetCoupon, err.Error()), nil)
		return
	}

	callback := func() error {
		return couponRecharge(region, serial, username, useInfo.Namespace, getResult.Amount)

	}

	result, err := models.UseCoupon(db, useInfo, callback)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeUseCoupon, err.Error()), nil)
		return
	}

	//if err != nil {
	//	logger.Error("call recharge api err: %v", err)
	//	JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCallRecharge, err.Error()), nil)
	//	return
	//}

	logger.Info("End use a coupon handler.")
	JsonResult(w, http.StatusOK, nil, result)
}

func ProvideCoupons(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: POST %v.", r.URL)
	logger.Info("Begin provide coupons handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	//username, e := validateAuth(r.Header.Get("Authorization"))
	//if e != nil {
	//	JsonResult(w, http.StatusUnauthorized, e, nil)
	//	return
	//}
	//logger.Debug("username:%v", username)
	//
	//if !canEditSaasApps(username) {
	//	JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
	//	return
	//}

	fromUserInfo := &models.FromUser{}
	err := common.ParseRequestJsonInto(r, fromUserInfo)
	logger.Debug("fromUserInfo: %v.", fromUserInfo)

	if fromUserInfo.OpenId == "" {
		fromUserInfo.OpenId = genCode()
		fromUserInfo.Provide_time = time.Now().Unix()
	}

	tm := time.Unix(fromUserInfo.Provide_time, 0)
	timeStr := tm.Format("2006-01-02 15:04:05.999999")

	err, isProvide := models.JudgeIsProvide(db, fromUserInfo, timeStr)
	logger.Info("isProvide: %v.", isProvide)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeProvideCoupons, err.Error()), nil)
		return
	} else if err == nil && isProvide == false {
		var card = struct {
			IsProvide bool   `json:"isProvide"`
			Code      string `json:"code"`
		}{true, ""}
		JsonResult(w, http.StatusOK, nil, card)
		return
	}

	r.ParseForm()

	number := r.Form.Get("number")
	amount := r.Form.Get("amount")
	count, codes, err := models.ProvideCoupon(db, number, amount)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeProvideCoupons, err.Error()), nil)
		return
	}

	if count == 0 {
		JsonResult(w, http.StatusBadRequest, GetError(ErrorNoMoreCoupon), nil)
		return
	}

	resultCode := codes[0]
	resultCode = resultCode[0:4] + "-" + resultCode[4:8] + "-" + resultCode[8:12] + "-" + resultCode[12:16]

	var card = struct {
		IsProvide bool   `json:"isProvide"`
		Code      string `json:"code"`
	}{false, strings.ToUpper(resultCode)}

	logger.Info("End provide coupons handler.")
	JsonResult(w, http.StatusOK, nil, card)
}

func FetchCoupons(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: %s %v.", r.Method, r.URL)
	logger.Info("Begin fetch coupons handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()
	env := r.Form.Get("env")
	logger.Info("env=%s", env)
	switch env {
	case "dev":
		ProvideCoupons(w, r, params)
		break
	case "pro":
		data, err := fecthCouponOnPro()
		if err != nil {
			break
		}
		result := struct {
			Code int
			Msg  string
			Data interface{}
		}{}
		card := struct {
			IsProvide bool   `json:"isProvide"`
			Code      string `json:"code"`
		}{}
		result.Data = &card
		err = json.Unmarshal(data, &result)
		if err != nil {
			logger.Error("Unmarshal err: %v", err)
			JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCallRecharge, err.Error()), nil)
			return
		}
		JsonResult(w, http.StatusOK, nil, card)
	}
	logger.Info("End fetch coupons handler.")
}

func genSerial() string {
	b := make([]byte, 15)
	for i := range b {
		b[i] = randNumber[rand.Intn(len(randNumber))]
	}
	return string(b)
}

func genCode() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func validateAuth(token, region string) (string, *Error) {
	if token == "" {
		return "", GetError(ErrorCodeAuthFailed)
	}

	username, err := getDFUserame(token, region)
	if err != nil {
		return "", GetError2(ErrorCodeAuthFailed, err.Error())
	}

	return username, nil
}

func initAdminUser() {
	admins := os.Getenv("ADMINUSERS")
	if admins == "" {
		logger.Warn("Not set admin users.")
	}
	admins = strings.TrimSpace(admins)
	AdminUsers = strings.Split(admins, " ")
	logger.Info("Admin users: %v.", AdminUsers)
}

func checkAdminUsers(user string) bool {
	for _, adminUser := range AdminUsers {
		if adminUser == user {
			return true
		}
	}

	return false
}
