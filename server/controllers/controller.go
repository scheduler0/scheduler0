package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
)

//  Basic controller can be used to perform all REST operations for an endpoint
type BasicController struct {
	model interface{}
	pool  repository.Pool
}

func CreateProjectModel() *models.Project {
	return &models.Project{}
}

func CreateJobModel() *models.Job {
	return &models.Job{}
}

func (controller *BasicController) CreateOne(w http.ResponseWriter, r *http.Request) {
	var model models.Model
	var modelType = reflect.TypeOf(controller.model).Name()

	if modelType == "Project" {
		model = CreateProjectModel()
	}

	if modelType == "Job" {
		model = CreateJobModel()
	}

	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)
	model.FromJson(body)

	if id, err := model.CreateOne(&controller.pool); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		misc.SendJson(w, id, http.StatusCreated, nil)
	}
}

func (controller *BasicController) GetOne(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		model.SetId(id)
		if err := model.GetOne(&controller.pool, "id = ?", id); err != nil {
			misc.SendJson(w, err, http.StatusOK, nil)
		} else {
			misc.SendJson(w, model, http.StatusOK, nil)
		}
	}
}

func (controller *BasicController) GetAll(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	var queryParams = misc.GetRequestQueryString(r.URL.RawQuery)
	var query, values = model.SearchToQuery(queryParams)

	if len(query) < 1 {
		misc.SendJson(w, errors.New("no valid query params"), http.StatusBadRequest, nil)
		return
	}

	if data, err := model.GetAll(&controller.pool, query, values...); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		misc.SendJson(w, data, http.StatusOK, nil)
	}
}

func (controller *BasicController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		body, err := ioutil.ReadAll(r.Body)
		misc.CheckErr(err)
		model.FromJson(body)
		model.SetId(id)
		if err = model.UpdateOne(&controller.pool); err != nil {
			misc.SendJson(w, err, http.StatusBadRequest, nil)
		} else {
			misc.SendJson(w, model, http.StatusOK, nil)
		}
	}
}

func (controller *BasicController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		model.SetId(id)
		if _, err := model.DeleteOne(&controller.pool); err != nil {
			misc.SendJson(w, err, http.StatusBadRequest, nil)
		} else {
			misc.SendJson(w, id, http.StatusOK, nil)
		}
	}
}

func (controller *BasicController) GetModel() models.Model {
	var innerModel models.Model
	var modelType = reflect.TypeOf(controller.model).Name()

	if modelType == "Project" {
		innerModel = CreateProjectModel()
	}

	if modelType == "Job" {
		innerModel = CreateJobModel()
	}

	return innerModel
}
