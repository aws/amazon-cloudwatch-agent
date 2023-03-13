package integration

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
)

type IntegrationTestSuite struct {
	suite.Suite
	IntegConfig IntegConfig
	Region      string
	RootDir     string
	//VarsAbsolutePath string
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.IntegConfig = FetchIntegConfig()
	suite.RootDir = GetRootDir()
	fmt.Println(suite.RootDir)
	//suite.VarsAbsolutePath = WriteVarsFile(suite.IntegConfig)
}

func (suite *IntegrationTestSuite) TestLocalWorkflow() {
	CheckBinaryExists(suite.IntegConfig)

}

func TestLocalWorkflowSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
