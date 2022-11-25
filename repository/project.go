package repository

import (
	_ "errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/araddon/dateparse"
	"log"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"time"
)

type Project interface {
	CreateOne(project *models.ProjectModel) (int64, *utils.GenericError)
	GetOneByName(project *models.ProjectModel) *utils.GenericError
	GetOneByID(project *models.ProjectModel) *utils.GenericError
	List(offset int64, limit int64) ([]models.ProjectModel, *utils.GenericError)
	Count() (int64, *utils.GenericError)
	UpdateOneByID(project models.ProjectModel) (int64, *utils.GenericError)
	DeleteOneByID(project models.ProjectModel) (int64, *utils.GenericError)
	GetBatchProjectsByIDs(projectIds []int64) ([]models.ProjectModel, *utils.GenericError)
}

type projectRepo struct {
	store   *fsm.Store
	jobRepo Job
	logger  *log.Logger
}

const (
	ProjectsTableName         = "projects"
	ProjectsIdColumn          = "id"
	NameColumn                = "name"
	DescriptionColumn         = "description"
	ProjectsDateCreatedColumn = "date_created"
)

func NewProjectRepo(logger *log.Logger, store *fsm.Store, jobRepo Job) Project {
	return &projectRepo{
		store:   store,
		jobRepo: jobRepo,
		logger:  logger,
	}
}

// CreateOne creates a single project
func (projectRepo *projectRepo) CreateOne(project *models.ProjectModel) (int64, *utils.GenericError) {
	if len(project.Name) < 1 {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, "name field is required")
	}

	if len(project.Description) < 1 {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, "description field is required")
	}

	projectWithName := models.ProjectModel{
		ID:   -1,
		Name: project.Name,
	}

	_ = projectRepo.GetOneByName(project)
	if projectWithName.ID > 0 {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("another project exist with the same name, project with id %v has the same name", projectWithName.ID))
	}

	query, params, err := sq.Insert(ProjectsTableName).
		Columns(
			NameColumn,
			DescriptionColumn,
			ProjectsDateCreatedColumn,
		).
		Values(
			project.Name,
			project.Description,
			time.Now().UTC(),
		).ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.logger, projectRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
	if applyErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	insertedId := res.Data[0].(int64)
	project.ID = insertedId

	getErr := projectRepo.GetOneByID(project)
	if getErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, getErr.Error())
	}

	return insertedId, nil

}

// GetOneByName returns a project with a matching name
func (projectRepo *projectRepo) GetOneByName(project *models.ProjectModel) *utils.GenericError {
	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		fmt.Sprintf("cast(\"%s\" as text)", ProjectsDateCreatedColumn),
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", NameColumn), project.Name).
		RunWith(projectRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var dateString string
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&dateString,
		)
		t, errParse := dateparse.ParseLocal(dateString)
		if errParse != nil {
			return utils.HTTPGenericError(500, fmt.Sprintf("%s dataString: %s", errParse.Error(), dateString))
		}
		project.DateCreated = t
		if err != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	if count == 0 {
		return utils.HTTPGenericError(http.StatusNotFound, "project with name : "+project.Name+" does not exist")
	}
	return nil
}

// GetOneByID returns a project that matches the uuid
func (projectRepo *projectRepo) GetOneByID(project *models.ProjectModel) *utils.GenericError {
	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		fmt.Sprintf("cast(\"%s\" as text)", ProjectsDateCreatedColumn),
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID).
		RunWith(projectRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var dateString string
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&dateString,
		)
		t, errParse := dateparse.ParseLocal(dateString)
		if errParse != nil {
			return utils.HTTPGenericError(500, fmt.Sprintf("%s dataString: %s", errParse.Error(), dateString))
		}
		project.DateCreated = t
		if err != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	if count == 0 {
		return utils.HTTPGenericError(http.StatusNotFound, "project does not exist")
	}
	return nil
}

func (projectRepo *projectRepo) GetBatchProjectsByIDs(projectIds []int64) ([]models.ProjectModel, *utils.GenericError) {
	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		fmt.Sprintf("cast(\"%s\" as text)", ProjectsDateCreatedColumn),
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s in (?)", ProjectsIdColumn), projectIds).
		RunWith(projectRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	projects := []models.ProjectModel{}
	for rows.Next() {
		project := models.ProjectModel{}
		var dateString string
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&dateString,
		)
		t, errParse := dateparse.ParseLocal(dateString)
		if errParse != nil {
			return nil, utils.HTTPGenericError(500, fmt.Sprintf("%s dataString: %s", errParse.Error(), dateString))
		}
		project.DateCreated = t
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		projects = append(projects, project)
		count += 1
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	if count == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "project does not exist")
	}

	return projects, nil
}

// List returns a paginated set of results
func (projectRepo *projectRepo) List(offset int64, limit int64) ([]models.ProjectModel, *utils.GenericError) {
	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		fmt.Sprintf("cast(\"%s\" as text)", ProjectsDateCreatedColumn),
	).
		From(ProjectsTableName).
		Offset(uint64(offset)).
		Limit(uint64(limit)).
		RunWith(projectRepo.store.SQLDbConnection)

	projects := []models.ProjectModel{}
	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		project := models.ProjectModel{}
		var dateString string
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&dateString,
		)
		t, errParse := dateparse.ParseLocal(dateString)
		if errParse != nil {
			return nil, utils.HTTPGenericError(500, fmt.Sprintf("%s dataString: %s", errParse.Error(), dateString))
		}
		project.DateCreated = t
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		projects = append(projects, project)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return projects, nil
}

// Count return the number of projects
func (projectRepo *projectRepo) Count() (int64, *utils.GenericError) {
	countQuery := sq.Select("count(*)").From(ProjectsTableName).RunWith(projectRepo.store.SQLDbConnection)
	rows, err := countQuery.Query()
	if err != nil {
		return 0, utils.HTTPGenericError(500, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&count,
		)
		if err != nil {
			return 0, utils.HTTPGenericError(500, err.Error())
		}
	}
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return int64(count), nil
}

// UpdateOneByID updates a single project
func (projectRepo *projectRepo) UpdateOneByID(project models.ProjectModel) (int64, *utils.GenericError) {
	updateQuery := sq.Update(ProjectsTableName).
		Set(DescriptionColumn, project.Description).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.logger, projectRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}
	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return count, nil
}

// DeleteOneByID deletes a single project
func (projectRepo *projectRepo) DeleteOneByID(project models.ProjectModel) (int64, *utils.GenericError) {
	projectJobs, getAllErr := projectRepo.jobRepo.GetAllByProjectID(project.ID, 0, 1, "id")
	if getAllErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, getAllErr.Error())
	}

	if len(projectJobs) > 0 {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, "cannot delete project with jobs")
	}

	deleteQuery := sq.
		Delete(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID)

	query, params, deleteErr := deleteQuery.ToSql()
	if deleteErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, deleteErr.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.logger, projectRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
	if applyErr != nil {
		return -1, applyErr
	}
	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return count, nil
}
