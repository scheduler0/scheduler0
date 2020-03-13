package controllers

import (
	"cron-server/server/dtos"
	"cron-server/server/migrations"
	"cron-server/server/misc"
	"cron-server/server/service"
	"github.com/gorilla/mux"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
)

type CredentialController struct {
	Pool migrations.Pool
}

func (cc *CredentialController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
	}

	credentialBody := dtos.CredentialDto{}

	err = credentialBody.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
	}

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }

	if newCredentialID, err := credentialService.CreateNewCredential(credentialBody.HTTPReferrerRestriction); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusTeapot, nil)
	} else {
		misc.SendJson(w, newCredentialID, true, http.StatusCreated, nil)
	}
}

func (cc *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.FindOneCredentialByID(params["id"])

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.UpdateOneCredential(params["id"])

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.DeleteOneCredential(params["id"])
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) ListAll(w http.ResponseWriter, r *http.Request) {
	queryParams := misc.GetRequestQueryString(r.URL.RawQuery)
	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }

	offset := 0
	limit := 0

	orderBy := "date_created DESC" // TODO: Extract orderBy from query params

	for i := 0; i < len(queryParams); i++ {
		if offsetInQueryString, ok := queryParams["offset"]; ok {
			if offsetInt, err := strconv.Atoi(offsetInQueryString); err != nil {
				misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
			} else {
				offset = offsetInt
			}
		}

		if limitInQueryString, ok := queryParams["limit"]; ok {
			if limitInt, err := strconv.Atoi(limitInQueryString); err != nil {
				misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
			} else {
				limit = int(math.Min(float64(limit), float64(limitInt)))
			}
		}
	}

	credentials, err := credentialService.ListCredentials(offset, limit, orderBy)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credentials, true, http.StatusOK, nil)
	}
}