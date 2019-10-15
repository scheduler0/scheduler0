package controllers

import (
	"cron-server/server/models"
	"cron-server/server/repository"
	"net/http"
)

var basicCredentialController = BasicController{model: models.Credential{}}

type CredentialController struct {
	Pool repository.Pool
}

func (cc *CredentialController) CreateOne(w http.ResponseWriter, r *http.Request) {
	basicCredentialController.CreateOne(w, r, cc.Pool)
}

func (cc *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicCredentialController.GetOne(w, r, cc.Pool)
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	basicCredentialController.UpdateOne(w, r, cc.Pool)
}

func (cc *CredentialController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicCredentialController.GetAll(w, r, cc.Pool)
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicCredentialController.DeleteOne(w, r, cc.Pool)
}

func (cc *CredentialController) GetAllOrCreateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cc.GetAll(w, r)
	}

	if r.Method == http.MethodPost {
		cc.CreateOne(w, r)
	}
}
