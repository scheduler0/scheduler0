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
	basicProjectController.CreateOne(w, r, cc.Pool)
}

func (cc *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetOne(w, r, cc.Pool)
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetOne(w, r, cc.Pool)
}

func (cc *CredentialController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetAll(w, r, cc.Pool)
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.DeleteOne(w, r, cc.Pool)
}
