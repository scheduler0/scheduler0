package controllers

import (
	"cron-server/server/src/misc"
	"cron-server/server/src/service"
	"cron-server/server/src/transformers"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
)

type CredentialController struct {
	Pool *misc.Pool
}

func (credentialController *CredentialController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
	}

	credentialBody := transformers.Credential{}

	err = credentialBody.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
	}

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}

	if newCredentialID, err := credentialService.CreateNewCredential(credentialBody.HTTPReferrerRestriction); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
	} else {
		if credential, err := credentialService.FindOneCredentialByID(newCredentialID); err != nil {
			fmt.Println(err,newCredentialID)
			misc.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
		} else {
			misc.SendJson(w, credential, true, http.StatusCreated, nil)
		}
	}
}

func (credentialController *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.FindOneCredentialByID(params["id"])

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (credentialController *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
	}

	credentialBody := transformers.Credential{}

	err = credentialBody.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
	}

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.UpdateOneCredential(params["id"], credentialBody.HTTPReferrerRestriction)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (credentialController *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	_, err := credentialService.DeleteOneCredential(params["id"])
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
	} else {
		misc.SendJson(w, nil, true, http.StatusNoContent, nil)
	}
}

func (credentialController *CredentialController) List(w http.ResponseWriter, r *http.Request) {
	queryParams := misc.GetRequestQueryString(r.URL.RawQuery)
	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}

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
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		misc.SendJson(w, credentials, true, http.StatusOK, nil)
	}
}
