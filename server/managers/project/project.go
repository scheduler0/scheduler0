package project

import (
	"errors"
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/models"
	"scheduler0/utils"
)

type ProjectManager models.ProjectModel

// CreateOne creates a single project
func (projectManager *ProjectManager) CreateOne(dbConnection *pg.DB) (string, *utils.GenericError) {
	if len(projectManager.Name) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "name field is required")
	}

	if len(projectManager.Description) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "description field is required")
	}

	projectWithName := ProjectManager{
		Name: projectManager.Name,
	}

	_ = projectWithName.GetOneByName(dbConnection)
	if len(projectWithName.UUID) > 5 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "Another project exist with the same name")
	}

	_, err := dbConnection.Model(projectManager).Insert()
	if err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return projectManager.UUID, nil
}

// GetOneByName returns a project with a matching name
func (projectManager *ProjectManager) GetOneByName(dbConnection *pg.DB) *utils.GenericError {
	projectManagers := []ProjectManager{}

	err := dbConnection.
		Model(&projectManagers).
		Where("name = ?", projectManager.Name).
		Select()

	if len(projectManagers) < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, "project with name : "+projectManager.Name+" does not exist")
	}

	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	projectManager.Name = projectManagers[0].Name
	projectManager.UUID = projectManagers[0].UUID
	projectManager.Description = projectManagers[0].Description
	projectManager.DateCreated = projectManagers[0].DateCreated
	projectManager.ID = projectManagers[0].ID

	return nil
}

// GetOneByUUID returns a project that matches the uuid
func (projectManager *ProjectManager) GetOneByUUID(dbConnection *pg.DB) *utils.GenericError {
	err := dbConnection.
		Model(projectManager).
		Where("uuid = ?", projectManager.UUID).
		Select()

	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

// GetAll returns a paginated set of results
func (projectManager *ProjectManager) GetAll(dbConnection *pg.DB, offset int, limit int) ([]ProjectManager, *utils.GenericError) {
	projects := make([]ProjectManager, 0, limit)

	err := dbConnection.Model(&projects).
		Order("date_created").
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return projects, nil
}

// Count return the number of projects
func (projectManager *ProjectManager) Count(dbConnection *pg.DB) (int, *utils.GenericError) {
	count, err := dbConnection.Model(projectManager).
		Order("date_created").
		Count()

	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// UpdateOne updates a single project
func (projectManager *ProjectManager) UpdateOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	savedProject := ProjectManager{UUID: projectManager.UUID}

	if e := savedProject.GetOneByUUID(dbConnection); e != nil {
		return 0, e
	}

	if len(savedProject.UUID) < 1 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "project does not exist")
	}

	if savedProject.Name != projectManager.Name {
		projectWithSimilarName := ProjectManager{
			Name: projectManager.Name,
		}

		e := projectWithSimilarName.GetOneByName(dbConnection)
		if e != nil && e.Type != http.StatusNotFound {
			return 0, e
		}
	}

	res, err := dbConnection.Model(projectManager).Where("UUID = ?", projectManager.UUID).Update(projectManager)
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a single project
func (projectManager *ProjectManager) DeleteOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	jobs := []models.JobModel{}

	err := dbConnection.Model(&jobs).Where("project_uuid = ?", projectManager.UUID).Select()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	if len(jobs) > 0 {
		err = errors.New("cannot delete projects with jobs")
		return -1, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	r, err := dbConnection.Model(projectManager).Where("uuid = ?", projectManager.UUID).Delete()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return r.RowsAffected(), nil
}
