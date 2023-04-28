package repository

import (
	_ "errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"time"
)

type Project interface {
	CreateOne(project *models.ProjectModel) (uint64, *utils.GenericError)
	GetOneByName(project *models.ProjectModel) *utils.GenericError
	GetOneByID(project *models.ProjectModel) *utils.GenericError
	List(offset uint64, limit uint64) ([]models.ProjectModel, *utils.GenericError)
	Count() (uint64, *utils.GenericError)
	UpdateOneByID(project models.ProjectModel) (uint64, *utils.GenericError)
	DeleteOneByID(project models.ProjectModel) (uint64, *utils.GenericError)
	GetBatchProjectsByIDs(projectIds []uint64) ([]models.ProjectModel, *utils.GenericError)
}

type projectRepo struct {
	fsmStore *fsm.Store
	jobRepo  Job
	logger   hclog.Logger
}

const (
	ProjectsTableName         = "projects"
	ProjectsIdColumn          = "id"
	NameColumn                = "name"
	DescriptionColumn         = "description"
	ProjectsDateCreatedColumn = "date_created"
)

func NewProjectRepo(logger hclog.Logger, store *fsm.Store, jobRepo Job) Project {
	return &projectRepo{
		fsmStore: store,
		jobRepo:  jobRepo,
		logger:   logger.Named("project-repo"),
	}
}

// CreateOne creates a single project
func (projectRepo *projectRepo) CreateOne(project *models.ProjectModel) (uint64, *utils.GenericError) {
	if len(project.Name) < 1 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "name field is required")
	}

	if len(project.Description) < 1 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "description field is required")
	}

	projectWithName := models.ProjectModel{
		ID:   0,
		Name: project.Name,
	}

	_ = projectRepo.GetOneByName(project)
	if projectWithName.ID > 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("another project exist with the same name, project with id %v has the same name", projectWithName.ID))
	}
	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	query, params, err := sq.Insert(ProjectsTableName).
		Columns(
			NameColumn,
			DescriptionColumn,
			ProjectsDateCreatedColumn,
		).
		Values(
			project.Name,
			project.Description,
			now,
		).ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if applyErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	insertedId := res.Data[0].(int64)
	project.ID = uint64(insertedId)

	getErr := projectRepo.GetOneByID(project)
	if getErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, getErr.Error())
	}

	return uint64(insertedId), nil
}

// GetOneByName returns a project with a matching name
func (projectRepo *projectRepo) GetOneByName(project *models.ProjectModel) *utils.GenericError {
	projectRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer projectRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		ProjectsDateCreatedColumn,
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", NameColumn), project.Name).
		RunWith(projectRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
		)
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
	projectRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer projectRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		ProjectsDateCreatedColumn,
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID).
		RunWith(projectRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
		)
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

func (projectRepo *projectRepo) GetBatchProjectsByIDs(projectIds []uint64) ([]models.ProjectModel, *utils.GenericError) {
	projectRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer projectRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	if len(projectIds) < 1 {
		return []models.ProjectModel{}, nil
	}

	cachedProjectIds := map[uint64]uint64{}

	for _, projectId := range projectIds {
		if _, ok := cachedProjectIds[projectId]; !ok {
			cachedProjectIds[projectId] = projectId
		}
	}

	ids := []uint64{}
	for _, projectId := range cachedProjectIds {
		ids = append(ids, projectId)
	}

	projectIdsArgs := []interface{}{ids[0]}
	idParams := "?"

	i := 0
	for i < len(ids)-1 {
		idParams += ",?"
		i += 1
		projectIdsArgs = append(projectIdsArgs, ids[i])
	}

	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		ProjectsDateCreatedColumn,
	).
		From(ProjectsTableName).
		Where(fmt.Sprintf("%s in (%s)", ProjectsIdColumn, idParams), projectIdsArgs...).
		RunWith(projectRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	count := 0
	projects := []models.ProjectModel{}
	for rows.Next() {
		project := models.ProjectModel{}
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		projects = append(projects, project)
		count += 1
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return projects, nil
}

// List returns a paginated set of results
func (projectRepo *projectRepo) List(offset uint64, limit uint64) ([]models.ProjectModel, *utils.GenericError) {
	projectRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer projectRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		ProjectsIdColumn,
		NameColumn,
		DescriptionColumn,
		ProjectsDateCreatedColumn,
	).
		From(ProjectsTableName).
		Offset(offset).
		Limit(limit).
		RunWith(projectRepo.fsmStore.DataStore.Connection)

	projects := []models.ProjectModel{}
	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	for rows.Next() {
		project := models.ProjectModel{}
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
		)
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
func (projectRepo *projectRepo) Count() (uint64, *utils.GenericError) {
	projectRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer projectRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	countQuery := sq.Select("count(*)").From(ProjectsTableName).RunWith(projectRepo.fsmStore.DataStore.Connection)
	rows, err := countQuery.Query()
	defer rows.Close()
	if err != nil {
		return 0, utils.HTTPGenericError(500, err.Error())
	}
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
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return uint64(count), nil
}

// UpdateOneByID updates a single project
func (projectRepo *projectRepo) UpdateOneByID(project models.ProjectModel) (uint64, *utils.GenericError) {
	updateQuery := sq.Update(ProjectsTableName).
		Set(DescriptionColumn, project.Description).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}
	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return uint64(count), nil
}

// DeleteOneByID deletes a single project
func (projectRepo *projectRepo) DeleteOneByID(project models.ProjectModel) (uint64, *utils.GenericError) {
	projectJobs, getAllErr := projectRepo.jobRepo.GetAllByProjectID(project.ID, 0, 1, "id")
	if getAllErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, getAllErr.Error())
	}

	if len(projectJobs) > 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot delete project with jobs")
	}

	deleteQuery := sq.
		Delete(ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectsIdColumn), project.ID)

	query, params, deleteErr := deleteQuery.ToSql()
	if deleteErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, deleteErr.Error())
	}

	res, applyErr := fsm.AppApply(projectRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if applyErr != nil {
		return 0, applyErr
	}
	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return uint64(count), nil
}
