package integration

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type IntegrationTestSuite struct {
	suite.Suite
	IntegConfig      IntegConfig
	VarsAbsolutePath string
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.IntegConfig = FetchIntegConfig()
	suite.VarsAbsolutePath = WriteVarsFile(suite.IntegConfig)

}

func (suite *IntegrationTestSuite) TestLocalWorkflow() {
	CheckBinaryExists(suite.IntegConfig)
	RunIntegrationTest(suite.IntegConfig, suite.VarsAbsolutePath)
}

func TestLocalWorkflowSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
