package managers

import (
	"errors"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"github.com/victorlenerd/scheduler0/server/src/models"
	"github.com/victorlenerd/scheduler0/server/src/utils"
)

type ProjectManager models.ProjectModel

func (p *ProjectManager) CreateOne(pool *utils.Pool) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(p.Name) < 1 {
		err := errors.New("name field is required")
		return "", err
	}

	if len(p.Description) < 1 {
		err := errors.New("description field is required")
		return "", err
	}

	var projectWithName = ProjectManager{
		Name: p.Name,
	}

	c, e := projectWithName.GetOne(pool)
	if c > 0 && e == nil {
		err := errors.New("projects exits with the same name " + p.Name)
		return "", err
	}

	p.ID = ksuid.New().String()

	_, err = db.Model(p).Insert()
	if err != nil {
		return "", err
	}

	return p.ID, nil
}

func (p *ProjectManager) GetOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	baseQuery := db.
		Model(p).
		WhereOr("id = ?", p.ID).
		WhereOr("name = ?", p.Name)

	count, err := baseQuery.Count()
	if count < 1 {
		return 0, nil
	}

	err = baseQuery.Select()
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (p *ProjectManager) GetAll(pool *utils.Pool, offset int, limit int) ([]ProjectManager, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	projects := make([]ProjectManager, 0, limit)

	err = db.Model(&projects).
		Order("date_created").
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (p *ProjectManager) GetTotalCount(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	count, err := db.Model(p).
		Order("date_created").
		Count()

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (p *ProjectManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	savedProject := ProjectManager{ID: p.ID}

	_, err = savedProject.GetOne(pool)
	if err != nil {
		return 0, err
	}

	if len(savedProject.ID) < 1 {
		return 0, errors.New("project does not exist")
	}

	if savedProject.Name != p.Name {

		fmt.Println("p.Name", p.Name)

		var projectWithSimilarName = ProjectManager{
			Name: p.Name,
		}

		c, err := projectWithSimilarName.GetOne(pool)

		fmt.Println("projectWithSimilarName", projectWithSimilarName, err, c)

		if err != nil {
			return 0, err
		}

		if c > 0 {
			return 0, errors.New("project with same name exits")
		}
	}

	res, err := db.Model(p).Where("id = ?", p.ID).Update(p)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (p *ProjectManager) DeleteOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobs []models.JobModel

	err = db.Model(&jobs).Where("project_id = ?", p.ID).Select()
	if err != nil {
		return -1, err
	}

	if len(jobs) > 0 {
		err = errors.New("cannot delete projects with jobs")
		return -1, err
	}

	r, err := db.Model(p).Where("id = ?", p.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
