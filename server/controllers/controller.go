package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"io/ioutil"
	"net/http"
	"reflect"
)

//  Basic controller can be used to perform all REST operations for an endpoint
type BasicController struct {
	model interface{}
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

	id, err := model.CreateOne()
	misc.CheckErr(err)
	misc.SendJson(w, id, http.StatusCreated, nil)
}

func (controller *BasicController) GetOne(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		model.SetId(id)
		err := model.GetOne("id = ?", id)
		misc.CheckErr(err)
		misc.SendJson(w, model, http.StatusOK, nil)
	}
}

func (controller *BasicController) GetAll(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	var queryParams = misc.GetRequestQueryString(r.URL.RawQuery)
	var query = ""
	params := make([]string, len(queryParams))

	for i := 0; i < len(queryParams); i++ {
		query += queryParams[i][0] + " = ?"
		params[i] = queryParams[i][1]
	}

	data, err := model.GetAll(query, params...)
	misc.CheckErr(err)
	misc.SendJson(w, data, http.StatusOK, nil)
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
		err = model.UpdateOne()
		misc.CheckErr(err)
		misc.SendJson(w, model, http.StatusOK, nil)
	}
}

func (controller *BasicController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	var model = controller.GetModel()
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		model.SetId(id)
		_, err := model.DeleteOne()
		misc.CheckErr(err)
		misc.SendJson(w, id, http.StatusOK, nil)
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
