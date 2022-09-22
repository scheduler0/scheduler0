package repository_test

//
//import (
//	"fmt"
//	"github.com/stretchr/testify/assert"
//	"log"
//	"scheduler0/server/db"
//	"scheduler0/server/repository"
//	"scheduler0/server/repository/fixtures"
//	"scheduler0/utils"
//	"strconv"
//	"testing"
//	"time"
//)
//
//func TestManager_Executions(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	dbConnection := db.GetTestDBConnection()
//
//	t.Run("Batch Insertion", func(t *testing.T) {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//		executionManagers := []repository.ExecutionRepo{}
//		executionManager := repository.ExecutionRepo{}
//
//		for i := 0; i < 200; i++ {
//			executionManagers = append(executionManagers, repository.ExecutionRepo{
//				JobID:       jobManager.ID,
//				TimeAdded:   time.Now().UTC(),
//				DateCreated: time.Now().UTC(),
//			})
//		}
//
//		uuids, err := executionManager.BatchInsertExecutions(dbConnection, executionManagers)
//		if err != nil {
//			log.Fatalln(err.Message)
//		}
//		assert.Equal(t, len(uuids), 200)
//	})
//
//	t.Run("Batch Get", func(t *testing.T) {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//		executionManagers := []repository.ExecutionRepo{}
//		executionManager := repository.ExecutionRepo{}
//
//		for i := 0; i < 3; i++ {
//			executionManagers = append(executionManagers, repository.ExecutionRepo{
//				JobID:       jobManager.ID,
//				TimeAdded:   time.Now().UTC(),
//				DateCreated: time.Now().UTC(),
//			})
//		}
//
//		insertedIds, err := executionManager.BatchInsertExecutions(dbConnection, executionManagers)
//		if err != nil {
//			t.Error(err)
//		}
//
//		ids, err := executionManager.BatchGetExecutions(dbConnection, insertedIds)
//		if err != nil {
//			log.Fatalln(err.Message)
//		}
//
//		assert.Equal(t, len(ids), 3)
//	})
//
//	t.Run("Batch Update", func(t *testing.T) {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//		executionManagers := []repository.ExecutionRepo{}
//		executionManager := repository.ExecutionRepo{}
//
//		for i := 0; i < 3; i++ {
//			executionManagers = append(executionManagers, repository.ExecutionRepo{
//				JobID:       jobManager.ID,
//				TimeAdded:   time.Now().UTC(),
//				DateCreated: time.Now().UTC(),
//			})
//		}
//
//		uuids, err := executionManager.BatchInsertExecutions(dbConnection, executionManagers)
//		if err != nil {
//			t.Error(err)
//		}
//
//		for i, _ := range executionManagers {
//			executionManagers[i].ID = uuids[i]
//			executionManagers[i].ExecutionTime = 100 + int64(i)
//			executionManagers[i].TimeExecuted = time.Now().UTC()
//			executionManagers[i].StatusCode = strconv.Itoa(200 + int(i))
//		}
//
//		err = executionManager.BatchUpdateExecutions(dbConnection, executionManagers)
//		assert.Nil(t, err)
//	})
//
//	t.Run("Do not create execution without job id", func(t *testing.T) {
//		executionManager := repository.ExecutionRepo{}
//		_, err := executionManager.CreateOne(dbConnection)
//		assert.NotEqual(t, err, nil)
//	})
//
//	t.Run("Create execution with valid job id", func(t *testing.T) {
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//		executionManager := repository.ExecutionRepo{
//			JobID: jobManager.ID,
//		}
//		_, err := executionManager.CreateOne(dbConnection)
//		assert.Nil(t, err)
//	})
//
//	t.Run("Returns 0 if execution does not exist", func(t *testing.T) {
//		executionManager := repository.ExecutionRepo{}
//		count, err := executionManager.GetOneByID(dbConnection)
//		fmt.Println("count", count)
//		assert.Nil(t, err)
//		assert.Equal(t, count == 0, true)
//	})
//
//	t.Run("Returns count 1 if execution exist", func(t *testing.T) {
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//		executionManager := repository.ExecutionRepo{
//			JobID: jobManager.ID,
//		}
//		executionManagerID, err := executionManager.CreateOne(dbConnection)
//		assert.Nil(t, err)
//
//		executionManager = repository.ExecutionRepo{ID: executionManagerID}
//		count, err := executionManager.GetOneByID(dbConnection)
//		assert.Nil(t, err)
//
//		assert.Equal(t, count > 0, true)
//	})
//
//	t.Run("Paginated results from manager", func(t *testing.T) {
//		jobManager := fixtures.CreateJobFixture(dbConnection)
//
//		for i := 0; i < 1000; i++ {
//			executionManager := repository.ExecutionRepo{
//				JobID: jobManager.ID,
//			}
//
//			_, err := executionManager.CreateOne(dbConnection)
//
//			assert.Nil(t, err)
//			if err != nil {
//				utils.Error(err.Message)
//			}
//		}
//
//		manager := repository.ExecutionRepo{}
//
//		executions, err := manager.ListByJobID(dbConnection, jobManager.ID, 0, 100, "date_created")
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
//		}
//
//		assert.Equal(t, len(executions), 100)
//
//		executions, err = manager.ListByJobID(dbConnection, jobManager.ID, 1000, 100, "date_created")
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
//		}
//
//		assert.Equal(t, len(executions), 0)
//	})
//}
