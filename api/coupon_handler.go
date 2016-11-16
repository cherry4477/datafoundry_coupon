package api

import (
	"github.com/asiainfoLDP/datafoundry_coupon/common"
	"github.com/asiainfoLDP/datafoundry_coupon/log"
	"github.com/asiainfoLDP/datafoundry_coupon/models"
	"github.com/asiainfoLDP/datafoundry_coupon/openshift"
	"github.com/julienschmidt/httprouter"
	"math/rand"
	"net/http"
	"time"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
	randNumber  = "0123456789"
)

var logger = log.GetLogger()

func CreateCoupon(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: POST %v.", r.URL)
	logger.Info("Begin create coupon handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	username, e := validateAuth(r.Header.Get("Authorization"))
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !canEditSaasApps(username) {
		JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
		return
	}

	coupon := &models.Coupon{}
	err := common.ParseRequestJsonInto(r, coupon)
	if err != nil {
		logger.Error("Parse body err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeParseJsonFailed, err.Error()), nil)
		return
	}

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

	username, e := validateAuth(r.Header.Get("Authorization"))
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !canEditSaasApps(username) {
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

	username, e := validateAuth(r.Header.Get("Authorization"))
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

	username, e := validateAuth(r.Header.Get("Authorization"))
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !canEditSaasApps(username) {
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

	username, e := validateAuth(r.Header.Get("Authorization"))
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

	err = couponRecharge(openshift.AdminToken(), serial, username, useInfo.Namespace, getResult.Amount)
	if err != nil {
		logger.Error("call recharge api err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCallRecharge, err.Error()), nil)
		return
	}

	result, err := models.UseCoupon(db, useInfo)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeUseCoupon, err.Error()), nil)
		return
	}

	logger.Info("End use a coupon handler.")
	JsonResult(w, http.StatusOK, nil, result)
}

func ProvideCoupons(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: PUT %v.", r.URL)
	logger.Info("Begin provide coupons handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	username, e := validateAuth(r.Header.Get("Authorization"))
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	if !canEditSaasApps(username) {
		JsonResult(w, http.StatusUnauthorized, GetError(ErrorCodePermissionDenied), nil)
		return
	}

	r.ParseForm()

	number := r.Form.Get("number")
	amount := r.Form.Get("amount")
	count, coupons, err := models.ProvideCoupon(db, number, amount)
	if err != nil {
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeProvideCoupons, err.Error()), nil)
		return
	}

	logger.Info("End provide coupons handler.")
	JsonResult(w, http.StatusOK, nil, NewQueryListResult(count, coupons))
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

func validateAuth(token string) (string, *Error) {
	if token == "" {
		return "", GetError(ErrorCodeAuthFailed)
	}

	username, err := getDFUserame(token)
	if err != nil {
		return "", GetError2(ErrorCodeAuthFailed, err.Error())
	}

	return username, nil
}

func canEditSaasApps(username string) bool {
	return username == "wangmeng5"
}
