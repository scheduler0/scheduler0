package auth_test

//var _ = Describe("Server Side Auth Test", func() {
//
//	dbConnection := db.GetTestDBConnection()
//	store := store2.NewStore(dbConnection, nil)
//	credentialRepo := repository.NewCredentialRepo(&store)
//	ctx := context.Background()
//	credentialService := service.NewCredentialService(credentialRepo, ctx)
//
//	BeforeEach(func() {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//	})
//
//	It("Should identify request from server side", func() {
//		req, err := http.NewRequest("POST", "/", nil)
//		Expect(err).To(BeNil())
//
//		credentialFixture := fixtures.CredentialFixture{}
//		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
//		credentialTransformer := credentialTransformers[0]
//
//		credentialTransformer.Platform = repository.ServerPlatform
//
//		_, createError := credentialService.CreateNewCredential(credentialTransformer)
//		if createError != nil {
//			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
//		}
//
//		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
//		req.Header.Set(auth.SecretKeyHeader, credentialTransformer.ApiSecret)
//
//		Expect(server.IsServerClient(req)).To(BeTrue())
//	})
//
//	It("Should not identify request from non server side", func() {
//		req, err := http.NewRequest("POST", "/", nil)
//		Expect(err).To(BeNil())
//
//		credentialFixture := fixtures.CredentialFixture{}
//		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
//
//		_, createError := credentialService.CreateNewCredential(credentialTransformers[0])
//		if createError != nil {
//			utils.Warn(fmt.Sprintf("Error: %v", createError.Message))
//		}
//
//		credentialTransformer := credentialTransformers[0]
//
//		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
//		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)
//		Expect(server.IsServerClient(req)).ToNot(BeTrue())
//
//		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
//		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)
//		Expect(server.IsServerClient(req)).ToNot(BeTrue())
//
//		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
//		Expect(server.IsServerClient(req)).ToNot(BeTrue())
//	})
//
//	It("Should identify authorized request from server clients", func() {
//		req, err := http.NewRequest("POST", "/", nil)
//		Expect(err).To(BeNil())
//
//		credentialFixture := fixtures.CredentialFixture{}
//		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
//		credentialTransformer := credentialTransformers[0]
//
//		credentialTransformer.Platform = repository.ServerPlatform
//
//		credentialManagerUUID, createError := credentialService.CreateNewCredential(credentialTransformer)
//		if createError != nil {
//			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
//		}
//
//		updatedCredentialTransformer, getError := credentialService.FindOneCredentialByID(credentialManagerUUID)
//		if getError != nil {
//			utils.Error(fmt.Sprintf("Error: %v", getError.Error()))
//		}
//
//		if updatedCredentialTransformer == nil {
//			Expect(updatedCredentialTransformer).ToNot(BeNil())
//			return
//		}
//
//		req.Header.Set(auth.APIKeyHeader, updatedCredentialTransformer.ApiKey)
//		req.Header.Set(auth.SecretKeyHeader, updatedCredentialTransformer.ApiSecret)
//
//		Expect(server.IsAuthorizedServerClient(req, credentialService)).To(BeTrue())
//	})
//})
//
//func TestServerSideAuth_Middleware(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Server Side Auth Test")
//}
