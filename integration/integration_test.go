package integration

import (
	"github.com/stretchr/testify/suite"
	"log"
	"path"
	"testing"
)

type IntegrationTestSuite struct {
	suite.Suite
	Config           Config
	RootDir          string
	VarsAbsolutePath string
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.Config = FetchConfig()
	suite.RootDir = GetRootDir()
	suite.VarsAbsolutePath = WriteVarsFile(suite.Config)
}

func (suite *IntegrationTestSuite) TestLocalWorkflow() {
	if terraformRelativePath, ok := suite.Config["terraformRelativePath"].(string); ok {
		terraformAbsolutePath := path.Join(suite.RootDir, terraformRelativePath)
		RunIntegrationTest(terraformAbsolutePath, suite.VarsAbsolutePath)
	} else {
		log.Fatal("Error: terraformPath was not provided in config.json")
	}
}

func TestLocalWorkflowSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
