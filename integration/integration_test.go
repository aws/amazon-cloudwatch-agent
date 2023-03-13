package integration

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
)

type IntegrationTestSuite struct {
	suite.Suite
	Config  Config
	Region  string
	RootDir string
	//VarsAbsolutePath string
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.Config = FetchConfig()
	suite.RootDir = GetRootDir()
	fmt.Println(suite.RootDir)
	//suite.VarsAbsolutePath = WriteVarsFile(suite.Config)
}

func (suite *IntegrationTestSuite) TestLocalWorkflow() {
	CheckBinaryExists(suite.Config)

}

func TestLocalWorkflowSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
