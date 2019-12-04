package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"fmt"
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

func CreateCredentialModel() *models.Credential {
	return &models.Credential{}
}

func CreateExecutionModel() *models.Execution {
	return &models.Execution{}
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

	if modelType == "Credential" {
		innerModel = CreateCredentialModel()
	}

	if modelType == "Execution" {
		innerModel = CreateExecutionModel()
	}

	return innerModel
}

func (controller *BasicController) CreateOne(w http.ResponseWriter, r *http.Request, pool repository.Pool) {
	var model = controller.GetModel()

	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)
	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	err = model.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	id, err := model.CreateOne(&pool, r.Context())
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	misc.SendJson(w, id, true, http.StatusCreated, nil)
}

func (controller *BasicController) GetOne(w http.ResponseWriter, r *http.Request, pool repository.Pool) {
	var model = controller.GetModel()
	id, err := misc.GetRequestParam(r, "id", 2)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	model.SetId(id)
	_, err = model.GetOne(&pool, r.Context(), "id = ?", id)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	}

	misc.SendJson(w, model, true, http.StatusOK, nil)
}

func (controller *BasicController) GetAll(w http.ResponseWriter, r *http.Request, pool repository.Pool) {
	var model = controller.GetModel()
	var queryParams = misc.GetRequestQueryString(r.URL.RawQuery)
	var query, values = model.SearchToQuery(queryParams)

	var offset = 0
	var limit = 10
	var orderBy = "date_created DESC"

	if len(query) < 1 {
		misc.SendJson(w, "no valid query params", false, http.StatusBadRequest, nil)
		return
	}

	fmt.Println(query, values)

	count, data, err := model.GetAll(&pool, r.Context(), query, offset, limit, orderBy, values...)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	var Response = struct {
		data []interface{}
		count int
	}{ data: data, count:count }

	misc.SendJson(w, Response, true, http.StatusOK, nil)
}

func (controller *BasicController) UpdateOne(w http.ResponseWriter, r *http.Request, pool repository.Pool) {
	var model = controller.GetModel()
	id, err := misc.GetRequestParam(r, "id", 2)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)
	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	err = model.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	model.SetId(id)
	_, err = model.UpdateOne(&pool, r.Context())

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	misc.SendJson(w, model, true, http.StatusOK, nil)
}

func (controller *BasicController) DeleteOne(w http.ResponseWriter, r *http.Request, pool repository.Pool) {
	var model = controller.GetModel()
	id, err := misc.GetRequestParam(r, "id", 2)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	model.SetId(id)
	if _, err := model.DeleteOne(&pool, r.Context()); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	misc.SendJson(w, id, true, http.StatusOK, nil)
}
