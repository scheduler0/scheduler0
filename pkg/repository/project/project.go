package project

import (
	_ "errors"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	job_repo "scheduler0/pkg/repository/job"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

type ProjectRepo interface {
	CreateOne(project *models.Project) (uint64, *utils.GenericError)
	GetOneByName(project *models.Project) *utils.GenericError
	GetOneByID(project *models.Project) *utils.GenericError
	List(offset uint64, limit uint64, accountId uint64, orderByColumn string, orderByDirection string) ([]models.Project, *utils.GenericError)
	ListAll(offset uint64, limit uint64) ([]models.Project, *utils.GenericError)
	Count(accountId uint64) (uint64, *utils.GenericError)
	CountAll() (uint64, *utils.GenericError)
	UpdateOneByID(project models.Project) (uint64, *utils.GenericError)
	DeleteOneByID(project models.Project) (uint64, *utils.GenericError)
	GetBatchProjectsByIDs(projectIds []uint64) ([]models.Project, *utils.GenericError)
}

type projectRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	jobRepo               job_repo.JobRepo
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

func NewProjectRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, store fsm.Scheduler0RaftStore, jobRepo job_repo.JobRepo) ProjectRepo {
	return &projectRepo{
		fsmStore:              store,
		scheduler0RaftActions: scheduler0RaftActions,
		jobRepo:               jobRepo,
		logger:                logger.Named("project-repo"),
	}
}

// CreateOne creates a single project
func (projectRepo *projectRepo) CreateOne(project *models.Project) (uint64, *utils.GenericError) {
	projectName := strings.TrimSpace(project.Name)
	if projectName == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "name field is required")
	}

	projectDescription := strings.TrimSpace(project.Description)
	if projectDescription == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "description field is required")
	}

	if project.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	projectWithName := models.Project{
		ID:        0,
		Name:      projectName,
		AccountId: project.AccountId,
	}

	_ = projectRepo.GetOneByName(&projectWithName)
	if projectWithName.ID > 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("another project exist with the same name, project with id %v has the same name", projectWithName.ID))
	}
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	query, params, err := sq.Insert(constants.ProjectsTableName).
		Columns(
			constants.ProjectsNameColumn,
			constants.ProjectsDescriptionColumn,
			constants.ProjectsDateCreatedColumn,
			constants.ProjectsAccountIdColumn,
		).
		Values(
			projectName,
			projectDescription,
			now,
			project.AccountId,
		).ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := projectRepo.scheduler0RaftActions.WriteCommandToRaftLog(projectRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - create one raft log result is nil")
	}

	insertedId := res.Data.LastInsertedId
	project.ID = uint64(insertedId)

	getErr := projectRepo.GetOneByID(project)
	if getErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, getErr.Error())
	}

	return uint64(insertedId), nil
}

// GetOneByName returns a project with a matching name
func (projectRepo *projectRepo) GetOneByName(project *models.Project) *utils.GenericError {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if project.AccountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		constants.ProjectsIdColumn,
		constants.ProjectsNameColumn,
		constants.ProjectsDescriptionColumn,
		constants.ProjectsDateCreatedColumn,
		constants.ProjectsAccountIdColumn,
	).
		From(constants.ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsNameColumn), project.Name).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), project.AccountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())

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
			&project.AccountId,
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
func (projectRepo *projectRepo) GetOneByID(project *models.Project) *utils.GenericError {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if project.AccountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		constants.ProjectsIdColumn,
		constants.ProjectsNameColumn,
		constants.ProjectsDescriptionColumn,
		constants.ProjectsDateCreatedColumn,
		constants.ProjectsAccountIdColumn,
	).
		From(constants.ProjectsTableName).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsIdColumn), project.ID).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), project.AccountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())

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
			&project.AccountId,
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

func (projectRepo *projectRepo) GetBatchProjectsByIDs(projectIds []uint64) ([]models.Project, *utils.GenericError) {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if len(projectIds) < 1 {
		return []models.Project{}, nil
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
		constants.ProjectsIdColumn,
		constants.ProjectsNameColumn,
		constants.ProjectsDescriptionColumn,
		constants.ProjectsDateCreatedColumn,
		constants.ProjectsAccountIdColumn,
	).
		From(constants.ProjectsTableName).
		Where(fmt.Sprintf("%s in (%s)", constants.ProjectsIdColumn, idParams), projectIdsArgs...).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	projects := []models.Project{}
	for rows.Next() {
		project := models.Project{}
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
			&project.AccountId,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		projects = append(projects, project)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return projects, nil
}

// List returns a paginated set of results
func (projectRepo *projectRepo) List(offset uint64, limit uint64, accountId uint64, orderByColumn string, orderByDirection string) ([]models.Project, *utils.GenericError) {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	// Validate orderByColumn to prevent SQL injection
	validColumns := map[string]bool{
		"id":           true,
		"name":         true,
		"description":  true,
		"date_created": true,
		"account_id":   true,
	}

	if !validColumns[orderByColumn] {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by column")
	}

	// Validate orderByDirection
	if orderByDirection != "" {
		orderByDirection = strings.ToLower(orderByDirection)
		if orderByDirection != "asc" && orderByDirection != "desc" {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by direction. Must be ASC or DESC")
		}
		orderByDirection = strings.ToUpper(orderByDirection)
	}

	selectBuilder := sq.Select(
		constants.ProjectsIdColumn,
		constants.ProjectsNameColumn,
		constants.ProjectsDescriptionColumn,
		constants.ProjectsDateCreatedColumn,
		constants.ProjectsAccountIdColumn,
	).
		From(constants.ProjectsTableName).
		Offset(offset).
		Limit(limit).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), accountId).
		OrderBy(fmt.Sprintf("%s %s", orderByColumn, orderByDirection)).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())

	projects := []models.Project{}
	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		project := models.Project{}
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
			&project.AccountId,
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
func (projectRepo *projectRepo) Count(accountId uint64) (uint64, *utils.GenericError) {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").
		From(constants.ProjectsTableName).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), accountId).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, err := countQuery.Query()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return uint64(count), nil
}

// Count return the number of projects
func (projectRepo *projectRepo) CountAll() (uint64, *utils.GenericError) {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").
		From(constants.ProjectsTableName).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, err := countQuery.Query()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return uint64(count), nil
}

// UpdateOneByID updates a single project
func (projectRepo *projectRepo) UpdateOneByID(project models.Project) (uint64, *utils.GenericError) {
	if project.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	projectDescription := strings.TrimSpace(project.Description)
	if projectDescription == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "description field is required")
	}

	// Check if project exists
	existingProject := models.Project{
		ID:        project.ID,
		AccountId: project.AccountId,
	}
	getErr := projectRepo.GetOneByID(&existingProject)
	if getErr != nil {
		return 0, getErr
	}

	updateQuery := sq.Update(constants.ProjectsTableName).
		Set(constants.ProjectsDescriptionColumn, projectDescription).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsIdColumn), project.ID).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), project.AccountId)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := projectRepo.scheduler0RaftActions.WriteCommandToRaftLog(projectRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}
	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// DeleteOneByID marks a project as deleted by setting the DeletedBy field and returns number of affected row
func (projectRepo *projectRepo) DeleteOneByID(project models.Project) (uint64, *utils.GenericError) {
	projectJobs, getAllErr := projectRepo.jobRepo.GetAllByProjectID(project.ID, 0, 1, "id", "ASC")
	if getAllErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, getAllErr.Error())
	}

	if len(projectJobs) > 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot delete project with jobs")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if project.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	updateQuery := sq.Update(constants.ProjectsTableName).
		Set(constants.ProjectsDeletedByColumn, project.DeletedBy).
		Set(constants.ProjectsDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsIdColumn), project.ID).
		Where(fmt.Sprintf("%s = ?", constants.ProjectsAccountIdColumn), project.AccountId)

	query, params, deleteErr := updateQuery.ToSql()
	if deleteErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, deleteErr.Error())
	}

	res, applyErr := projectRepo.scheduler0RaftActions.WriteCommandToRaftLog(projectRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - delete one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

func (projectRepo *projectRepo) ListAll(offset uint64, limit uint64) ([]models.Project, *utils.GenericError) {
	projectRepo.fsmStore.GetDataStore().ConnectionLock()
	defer projectRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.ProjectsIdColumn,
		constants.ProjectsNameColumn,
		constants.ProjectsDescriptionColumn,
		constants.ProjectsDateCreatedColumn,
		constants.ProjectsAccountIdColumn,
	).
		From(constants.ProjectsTableName).
		Offset(offset).
		Limit(limit).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.ProjectsDeletedByColumn, constants.ProjectsDeletedByColumn)).
		RunWith(projectRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	projects := []models.Project{}
	for rows.Next() {
		project := models.Project{}
		err = rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.DateCreated,
			&project.AccountId,
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
