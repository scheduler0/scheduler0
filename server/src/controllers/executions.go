package controllers

import (
	"net/http"
)

type ExecutionController Controller

func (cc *ExecutionController) GetOne(w http.ResponseWriter, r *http.Request) {}


func (cc *ExecutionController) List(w http.ResponseWriter, r *http.Request) {}
