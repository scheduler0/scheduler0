package project_test

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"scheduler0/server/db"
	"scheduler0/server/managers/job"
	"scheduler0/server/managers/project"
	fixtures2 "scheduler0/server/managers/project/fixtures"
	"scheduler0/utils"
	"strconv"
	"testing"
)

var _ = Describe("Project Manager", func() {
	pool := db.GetTestPool()

	BeforeEach(func() {
		db.Teardown()
		db.Prepare()
	})

	It("Don't create project with name and description empty", func() {
		projectManager := project.ProjectManager{}
		_, err := projectManager.CreateOne(pool)
		if err == nil {
			utils.Error("[ERROR] Cannot create project without name and descriptions")
		}
		Expect(err).ToNot(BeNil())
	})

	It("Create project with name and description not empty", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}

		_, createOneProjectError := projectManager.CreateOne(pool)
		if createOneProjectError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Error creating project %v", err))
		}
	})

	It("Don't create project with existing name", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManagerOne := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}

		projectManagerTwo := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}

		_, createOneError := projectManagerOne.CreateOne(pool)
		if createOneError != nil {
			utils.Error(createOneError.Message)
		}
		Expect(createOneError).To(BeNil())

		_, createTwoError := projectManagerTwo.CreateOne(pool)
		if createTwoError == nil {
			utils.Error(fmt.Sprintf("[ERROR] Cannot create project with existing name"))
		}
		Expect(createTwoError).ToNot(BeNil())
	})

	It("Can retrieve a single project by uuid", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}
		_, projectManagerError := projectManager.CreateOne(pool)
		if projectManagerError != nil {
			utils.Error(projectManagerError.Message)
		}
		Expect(projectManagerError).To(BeNil())

		projectManagerByName := project.ProjectManager{
			UUID: projectManager.UUID,
		}

		projectManagerByNameError := projectManagerByName.GetOneByUUID(pool)
		if projectManagerByNameError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Could not get project %v", err))
		}

		Expect(projectManagerByName.UUID).To(Equal(projectManager.UUID))
	})

	It("Can retrieve a single project by name", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}
		_, projectManagerError := projectManager.CreateOne(pool)
		if projectManagerError != nil {
			utils.Error(projectManagerError.Message)
		}
		Expect(projectManagerError).To(BeNil())

		projectManagerByName := project.ProjectManager{
			Name: projectManager.Name,
		}

		projectManagerByNameError := projectManagerByName.GetOneByName(pool)
		if projectManagerByNameError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Could not get project %v", err))
		}

		Expect(projectManagerByName.UUID).To(Equal(projectManager.UUID))
		Expect(projectManagerByName.Name).To(Equal(projectManager.Name))
	})

	It("Can update name and description for a project", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}
		_, createProjectManagerError := projectManager.CreateOne(pool)
		if createProjectManagerError != nil {
			utils.Error(createProjectManagerError.Message)
		}
		Expect(createProjectManagerError).To(BeNil())

		projectFixture = fixtures2.ProjectFixture{}
		err = faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectTwoPlaceholder := project.ProjectManager{
			UUID: projectManager.UUID,
			Name: projectFixture.Name,
		}
		_, updateProjectError := projectTwoPlaceholder.UpdateOne(pool)
		if updateProjectError != nil {
			utils.Error(updateProjectError.Message)
		}

		Expect(projectManager.Name).NotTo(Equal(projectTwoPlaceholder.Name))
	})

	It("Delete All Projects", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}
		_, createProjectManagerError := projectManager.CreateOne(pool)
		if createProjectManagerError != nil {
			utils.Error(createProjectManagerError.Message)
		}

		rowsAffected, deleteProjectOneError := projectManager.DeleteOne(pool)
		if deleteProjectOneError != nil || rowsAffected < 1 {
			utils.Error(fmt.Sprintf("[ERROR] Cannot delete project one %v", err))
		}
	})

	It("Don't delete project with a job", func() {
		projectFixture := fixtures2.ProjectFixture{}
		err := faker.FakeData(&projectFixture)
		utils.CheckErr(err)

		projectManager := project.ProjectManager{
			Name:        projectFixture.Name,
			Description: projectFixture.Description,
		}
		_, createProjectManagerError := projectManager.CreateOne(pool)
		if createProjectManagerError != nil {
			utils.Error(createProjectManagerError.Message)
		}

		job := job.Manager{
			ProjectUUID: projectManager.UUID,
			Spec:        "* * * * *",
			CallbackUrl: "https://some-random-url",
		}

		_, createOneJobError := job.CreateOne(pool)
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", createOneJobError.Message))
		}

		rowsAffected, deleteOneProjectError := projectManager.DeleteOne(pool)
		if deleteOneProjectError == nil || rowsAffected > 0 {
			utils.Error(fmt.Sprintf("[ERROR] Projects with jobs shouldn't be deleted %v", rowsAffected))
		}

		rowsAffected, deleteOneJobError := job.DeleteOne(pool)
		if err != nil || rowsAffected < 1 {
			utils.Error(fmt.Sprintf("[ERROR] Could not delete job  %v %v", deleteOneJobError.Message, rowsAffected))
		}

		rowsAffected, deleteOneProjectError = projectManager.DeleteOne(pool)
		if err != nil || rowsAffected < 1 {
			utils.Error("[ERROR] Could not delete project %v", rowsAffected)
		}
	})

	It("ProjectManager.List", func() {
		manager := project.ProjectManager{}

		for i := 0; i < 1000; i++ {
			project := project.ProjectManager{
				Name:        "project " + strconv.Itoa(i),
				Description: "project description " + strconv.Itoa(i),
			}

			_, err := project.CreateOne(pool)
			if err != nil {
				utils.Error("[ERROR] failed to create a project ::", err.Message)
			}
		}

		projects, err := manager.GetAll(pool, 0, 100)
		if err != nil {
			utils.Error("[ERROR] failed to fetch projects ::", err.Message)
		}

		Expect(len(projects)).To(Equal(100))
	})
})

func TestProject_Manager(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Project Manager Suite")
}
